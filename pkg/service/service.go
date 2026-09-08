package service

import (
	"agents/openai"
	"agents/store"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/audita-bids/private-kit/connectors"
	"github.com/audita-bids/private-kit/pkg/pb/protocols/client"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/log"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Service interface {
	PostAnalysis(ctx context.Context, analysis *store.Analysis) (*store.Analysis, error)
	GetAnalysis(ctx context.Context, analysis *store.Analysis) (*store.Analysis, error)
}

type service struct {
	logger   log.Logger
	db       *mongo.Database
	analysis *store.AnalysisStore
	openai   *openai.Client
	clients  client.ClientServiceClient
}

func NewService(logger log.Logger, db *mongo.Database, redis *redis.Client) Service {
	var svc Service

	analysis := db.Collection("analysis")

	{
		svc = &service{
			logger: logger,
			db:     db,
			analysis: &store.AnalysisStore{
				C: analysis,
			},
			openai:  openai.NewOpenaiClient(),
			clients: client.NewClientServiceClient(connectors.Client()),
		}
		svc = LoggingMiddleware(logger)(svc)
		svc = RecoveryMiddleware(logger)(svc)
		svc = EventMiddleware(logger)(svc)
		svc = CacheMiddleware(logger, redis)(svc)
	}

	return svc
}

func (s *service) PostAnalysis(ctx context.Context, analysis *store.Analysis) (*store.Analysis, error) {
	probe := &store.Analysis{
		BidID:  analysis.BidID,
		UserID: analysis.UserID,
	}

	if existing, err := s.analysis.GetBidAnalysis(ctx, probe); err == nil {
		level.Info(s.logger).Log("msg", "analysis already exists, returning cached", "id", existing.ID.Hex())
		return existing, nil
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		level.Error(s.logger).Log("msg", "failed to check existing analysis", "err", err)
	}

	v, err := s.openai.ResumeBase64(ctx, analysis.Base64)

	if err != nil {
		level.Error(s.logger).Log("msg", "failed to decode base64", "err", err)
		return nil, err
	}

	// for now, deny if user doenst have keywords (or doenst exists, obvious)
	c, err := s.clients.FindClient(ctx, &client.FindClientRequest{
		Id: analysis.UserID,
	})
	if err != nil {
		level.Error(s.logger).Log("msg", "failed to fetch client keywords", "err", err)
		return nil, err
	}

	keywords := make([]string, 0, len(c.GetKeywords()))
	for _, k := range c.GetKeywords() {
		if k = strings.TrimSpace(k); k != "" {
			keywords = append(keywords, k)
		}
	}

	if len(keywords) == 0 {
		return nil, errors.New("client has no keywords configured for scoring")
	}

	extraction, raw, err := s.openai.AnalyzeNotice(ctx, v, keywords)

	if err != nil {
		level.Error(s.logger).Log("msg", "notice analysis failed", "err", err)
		return nil, err
	}

	score := int32(extraction.Score)
	if len(extraction.MatchedKeywords) == 0 && score > 25 {
		level.Info(s.logger).Log("msg", "score clamped: no matched keywords", "model_score", score)
		score = 25
	}

	now := time.Now()
	analysis.Finished = true
	analysis.Object = extraction.Object
	analysis.Modality = extraction.Modality
	analysis.ProcessNumber = extraction.ProcessNumber
	analysis.EstimatedValue = extraction.EstimatedValue
	analysis.OpeningDate = extraction.OpeningDate
	analysis.JudgmentCriteria = extraction.JudgmentCriteria
	analysis.Content = extraction.Summary
	analysis.Score = score
	analysis.Keywords = keywords
	analysis.AnalysisResult = raw
	analysis.CreatedAt = &now
	analysis.UpdatedAt = &now

	err = s.analysis.CreateAnalysis(ctx, analysis)
	if err != nil {
		level.Error(s.logger).Log("msg", "failed to save analysis", "err", err)
		return nil, err
	}

	level.Info(s.logger).Log("msg", "analysis completed", "id", analysis.ID.Hex(), "score", analysis.Score, "qualifications", len(extraction.Qualifications), "qualifications_complete", extraction.QualificationsComplete, "prompt_tokens", extraction.PromptTokens, "completion_tokens", extraction.CompletionTokens)
	return analysis, nil
}

func (s *service) GetAnalysis(ctx context.Context, analysis *store.Analysis) (*store.Analysis, error) {
	return s.analysis.GetBidAnalysis(ctx, analysis)
}

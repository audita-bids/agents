package service

import (
	"agents/openai"
	"agents/store"
	"context"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/log"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Service interface {
	PostAnalysis(ctx context.Context, analysis *store.Analysis) (*store.Analysis, error)
}

type service struct {
	logger   log.Logger
	db       *mongo.Database
	analysis *store.AnalysisStore
	openai   *openai.Client
}

func NewService(logger log.Logger, db *mongo.Database) Service {
	var svc Service

	analysis := db.Collection("analysis")

	{
		svc = &service{
			logger: logger,
			db:     db,
			analysis: &store.AnalysisStore{
				C: analysis,
			},
			openai: openai.NewOpenaiClient(),
		}
		svc = LoggingMiddleware(logger)(svc)
		svc = RecoveryMiddleware(logger)(svc)
		svc = EventMiddleware(logger)(svc)
	}

	return svc
}

func (s *service) PostAnalysis(ctx context.Context, analysis *store.Analysis) (*store.Analysis, error) {
	analysis.ProcessNumber = analysis.ProcessID
	v, err := s.openai.ResumeBase64(ctx, analysis.Base64)

	if err != nil {
		level.Error(s.logger).Log("msg", "failed to decode base64", "err", err)
		return nil, err
	}

	extraction, err := s.openai.ResumeNoticeRAG(ctx, v)

	if err != nil {
		level.Error(s.logger).Log("msg", "RAG extraction failed", "err", err)
		return nil, err
	}

	now := time.Now()
	analysis.Finished = true
	analysis.Object = extraction.Object
	analysis.Modality = extraction.Modality
	analysis.EstimatedValue = extraction.EstimatedValue
	analysis.JudgmentCriteria = extraction.JudgmentCriteria
	analysis.CreatedAt = &now
	analysis.UpdatedAt = &now

	s.analysis.CreateAnalysis(ctx, analysis)
	if err != nil {
		level.Error(s.logger).Log("msg", "failed to save analysis", "err", err)
		return nil, err
	}

	level.Info(s.logger).Log("msg", "analysis completed", "id", analysis.ID.Hex())
	return analysis, nil
}

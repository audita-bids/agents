package transports

import (
	"agents/pkg/endpoint"
	"agents/store"
	"context"
	"strconv"
	"time"

	"github.com/audita-bids/private-kit/decode"
	"github.com/audita-bids/private-kit/pkg/pb/protocols/agents"
	grpctransport "github.com/go-kit/kit/transport/grpc"
)

type GRPCServer struct {
	agents.UnimplementedAgentsServiceServer

	postNoticeAnalysis grpctransport.Handler
	getAnalysis        grpctransport.Handler
	postCopilot        grpctransport.Handler
}

func NewGRPCServer(endpoints endpoint.EndpointSetup) agents.AgentsServiceServer {
	options := []grpctransport.ServerOption{
		grpctransport.ServerBefore(decode.GRPCParams),
	}

	return &GRPCServer{
		postNoticeAnalysis: grpctransport.NewServer(
			endpoints.PostNoticeAnalysis,
			decodeGRPCPostNoticeAnalysisRequest,
			encodeGRPCPostNoticeAnalysisResponse,
			options...,
		),
		getAnalysis: grpctransport.NewServer(
			endpoints.GetAnalysis,
			decodeGRPCGetAnalysisRequest,
			encodeGRPCGetAnalysisResponse,
			options...,
		),
		postCopilot: grpctransport.NewServer(
			endpoints.PostCopilot,
			decodeGRPCPostCopilotRequest,
			encodeGRPCPostCopilotResponse,
			options...,
		),
	}
}

func (s *GRPCServer) PostAnalysis(ctx context.Context, req *agents.PostAnalysisRequest) (*agents.AgentsComplete, error) {
	_, resp, err := s.postNoticeAnalysis.ServeGRPC(ctx, req)

	if err != nil {
		return nil, err
	}

	return resp.(*agents.AgentsComplete), nil
}

func (s *GRPCServer) GetAnalysis(ctx context.Context, req *agents.GetAnalysisRequest) (*agents.AgentsComplete, error) {
	_, resp, err := s.getAnalysis.ServeGRPC(ctx, req)

	if err != nil {
		return nil, err
	}

	return resp.(*agents.AgentsComplete), nil
}

func (s *GRPCServer) PostCopilot(ctx context.Context, req *agents.PostCopilotRequest) (*agents.AgentsComplete, error) {
	_, resp, err := s.postCopilot.ServeGRPC(ctx, req)

	if err != nil {
		return nil, err
	}

	return resp.(*agents.AgentsComplete), nil
}

func decodeGRPCPostCopilotRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*agents.PostCopilotRequest)

	return &store.Analysis{
		Message:   req.Message,
		UserID:    req.UserId,
		AgentType: agents.AgentType_COPILOT,
	}, nil
}

func encodeGRPCPostCopilotResponse(_ context.Context, grpcResp interface{}) (interface{}, error) {
	resp := grpcResp.(*endpoint.Resp)
	if resp.Error != nil {
		return nil, resp.Error
	}

	return toCopilotComplete(resp.Items.(*store.Analysis)), nil
}

func toCopilotComplete(a *store.Analysis) *agents.AgentsComplete {
	out := &agents.AgentsComplete{
		Finished:    true,
		UserId:      a.UserID,
		Type:        agents.AgentType_COPILOT,
		UserPrompt:  a.Message,
		LlmResponse: a.AnalysisResult,
	}
	if a.ID != nil {
		out.Id = a.ID.Hex()
	}
	if a.CreatedAt != nil {
		out.CreatedAt = a.CreatedAt.Format(time.RFC3339)
	}
	return out
}

func decodeGRPCPostNoticeAnalysisRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*agents.PostAnalysisRequest)

	return &store.Analysis{
		Base64: req.Base64,
		UserID: req.UserId,
		BidID:  req.ProcessId,
	}, nil
}

func encodeGRPCPostNoticeAnalysisResponse(_ context.Context, grpcResp interface{}) (interface{}, error) {
	resp := grpcResp.(*endpoint.Resp)
	if resp.Error != nil {
		return nil, resp.Error
	}

	return toAgentsComplete(resp.Items.(*store.Analysis)), nil
}

// toAgentsComplete maps the stored analysis onto the proto message explicitly.
// (JSON-tag-based mapping breaks on the int score vs string proto field and on
// snake/Pascal tag mismatches — keep this the single conversion point.)
func toAgentsComplete(a *store.Analysis) *agents.AgentsComplete {
	out := &agents.AgentsComplete{
		Finished:         a.Finished,
		ProcessId:        a.BidID,
		UserId:           a.UserID,
		Content:          a.Content,
		Object:           a.Object,
		Modality:         a.Modality,
		ProcessNumber:    a.ProcessNumber,
		EstimatedValue:   a.EstimatedValue,
		OpeningDate:      a.OpeningDate,
		JudgmentCriteria: a.JudgmentCriteria,
		AnalysisResult:   a.AnalysisResult,
		Score:            strconv.FormatInt(int64(a.Score), 10),
	}
	if a.ID != nil {
		out.Id = a.ID.Hex()
	}
	if a.CreatedAt != nil {
		out.CreatedAt = a.CreatedAt.Format(time.RFC3339)
	}
	if a.UpdatedAt != nil {
		out.UpdatedAt = a.UpdatedAt.Format(time.RFC3339)
	}
	return out
}

func decodeGRPCGetAnalysisRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*agents.GetAnalysisRequest)

	return &store.Analysis{
		UserID: req.UserId,
		BidID:  req.BidId,
	}, nil
}

func encodeGRPCGetAnalysisResponse(_ context.Context, grpcResp interface{}) (interface{}, error) {
	resp := grpcResp.(*store.Analysis)

	analysis := new(agents.AgentsComplete)
	resp.Unmarshal(&analysis)

	return analysis, nil
}

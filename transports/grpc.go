package transports

import (
	"agents/pkg/endpoint"
	"agents/store"
	"context"

	grpctransport "github.com/go-kit/kit/transport/grpc"
	"github.com/project-pncp/private-kit/decode"
	"github.com/project-pncp/private-kit/pkg/pb/protocols/agents"
)

type GRPCServer struct {
	agents.UnimplementedAgentsServiceServer

	postNoticeAnalysis grpctransport.Handler
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
	}
}

func (s *GRPCServer) PostAnalysis(ctx context.Context, req *agents.PostAnalysisRequest) (*agents.AgentsComplete, error) {
	_, resp, err := s.postNoticeAnalysis.ServeGRPC(ctx, req)

	if err != nil {
		return nil, err
	}

	return resp.(*agents.AgentsComplete), nil
}

func decodeGRPCPostNoticeAnalysisRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*agents.PostAnalysisRequest)

	return &store.Analysis{
		ProcessID: req.ProcessId,
		Base64:    req.Base64,
		UserID:    req.UserId,
	}, nil
}

func encodeGRPCPostNoticeAnalysisResponse(_ context.Context, grpcResp interface{}) (interface{}, error) {
	resp := grpcResp.(*endpoint.Resp)
	if resp.Error != nil {
		return nil, resp.Error
	}

	analysis := resp.Items.(*store.Analysis)

	result := &agents.AgentsComplete{
		Id:               analysis.ID.Hex(),
		UserId:           analysis.UserID,
		ProcessId:        analysis.ProcessID,
		Finished:         analysis.Finished,
		Object:           analysis.Object,
		Modality:         analysis.Modality,
		ProcessNumber:    analysis.ProcessNumber,
		EstimatedValue:   analysis.EstimatedValue,
		OpeningDate:      analysis.OpeningDate,
		JudgmentCriteria: analysis.JudgmentCriteria,
	}
	return result, nil
}

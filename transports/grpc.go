package transports

import (
	"agents/pkg/endpoint"
	"agents/store"
	"context"

	grpctransport "github.com/go-kit/kit/transport/grpc"
	"github.com/newdesksoftwares/private-kit/decode"
	"github.com/newdesksoftwares/private-kit/pkg/pb/protocols/agents"
)

type GRPCServer struct {
	agents.UnimplementedAgentsServiceServer

	postNoticeAnalysis grpctransport.Handler
	getAnalysis        grpctransport.Handler
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

func decodeGRPCPostNoticeAnalysisRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*agents.PostAnalysisRequest)

	return &store.Analysis{
		Base64: req.Base64,
		UserID: req.UserId,
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
		Finished:         analysis.Finished,
		Object:           analysis.Object,
		Modality:         analysis.Modality,
		EstimatedValue:   analysis.EstimatedValue,
		OpeningDate:      analysis.OpeningDate,
		JudgmentCriteria: analysis.JudgmentCriteria,
	}
	return result, nil
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

	var analysis agents.AgentsComplete
	resp.Unmarshal(analysis)

	return analysis, nil
}

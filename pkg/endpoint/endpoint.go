package endpoint

import (
	"agents/pkg/service"
	"agents/store"
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/log"
)

type EndpointSetup struct {
	CreateFavoriteBid  endpoint.Endpoint
	ListFavoriteBid    endpoint.Endpoint
	DeleteFavoriteBid  endpoint.Endpoint
	PostNoticeAnalysis endpoint.Endpoint
}

func NewEndpointSetup(s service.Service, logger log.Logger) *EndpointSetup {
	var postNoticeAnalysisEndpoint endpoint.Endpoint

	{
		postNoticeAnalysisEndpoint = MakePostNoticeAnalysisEndpoint(s)
	}

	return &EndpointSetup{
		PostNoticeAnalysis: postNoticeAnalysisEndpoint,
	}
}

func MakePostNoticeAnalysisEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(*store.Analysis)

		c, err := s.PostAnalysis(ctx, req)
		if err != nil {
			return nil, err
		}

		return &Resp{
			Items: c,
		}, nil
	}
}

type Resp struct {
	Error  error       `json:"error,omitempty"`
	Items  interface{} `json:"items,omitempty"`
	Total  int64       `json:"total,omitempty"`
	Cursor string      `json:"cursor,omitempty"`
}

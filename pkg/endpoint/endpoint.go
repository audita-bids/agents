package endpoint

import (
	"agents/pkg/service"
	"agents/store"
	"context"

	"github.com/audita-bids/private-kit/middlewares"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/log"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type EndpointSetup struct {
	PostNoticeAnalysis endpoint.Endpoint
	GetAnalysis        endpoint.Endpoint
}

func NewEndpointSetup(s service.Service, logger log.Logger) *EndpointSetup {
	var postNoticeAnalysisEndpoint endpoint.Endpoint
	var getAnalysisEndpoint endpoint.Endpoint

	loggingMiddleware := middlewares.EndpointLoggingMiddleware(logger, "agents")
	metricsMiddleware := middlewares.MetricsMiddleware("agents")
	{
		postNoticeAnalysisEndpoint = MakePostNoticeAnalysisEndpoint(s)
		postNoticeAnalysisEndpoint = loggingMiddleware("PostNoticeAnalysis")(postNoticeAnalysisEndpoint)
		postNoticeAnalysisEndpoint = metricsMiddleware("PostNoticeAnalysis")(postNoticeAnalysisEndpoint)

		getAnalysisEndpoint = MakeGetAnalysisEndpoint(s)
		getAnalysisEndpoint = loggingMiddleware("GetAnalysis")(getAnalysisEndpoint)
		getAnalysisEndpoint = metricsMiddleware("GetAnalysis")(getAnalysisEndpoint)
	}

	return &EndpointSetup{
		PostNoticeAnalysis: postNoticeAnalysisEndpoint,
		GetAnalysis:        getAnalysisEndpoint,
	}
}

func MakePostNoticeAnalysisEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(*store.Analysis)

		bId := bson.NewObjectID()
		req.ID = &bId

		c, err := s.PostAnalysis(ctx, req)
		if err != nil {
			return nil, err
		}

		return &Resp{
			Items: c,
		}, nil
	}
}

func MakeGetAnalysisEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(*store.Analysis)

		c, err := s.GetAnalysis(ctx, req)
		if err != nil {
			return nil, err
		}

		return c, nil
	}
}

type Resp struct {
	Error  error       `json:"error,omitempty"`
	Items  interface{} `json:"items,omitempty"`
	Total  int64       `json:"total,omitempty"`
	Cursor string      `json:"cursor,omitempty"`
}

package transports

import (
	"agents/pkg/endpoint"
	"agents/store"
	"context"
	"encoding/json"

	"github.com/audita-bids/private-kit/kafka"
	kaf "github.com/segmentio/kafka-go"
)

func NewKafkaConsumers(endpoints endpoint.EndpointSetup) kafka.Consumers {
	return kafka.Consumers{
		"ANALYSIS_CREATED": kafka.NewConsumer(
			endpoints.ExecuteAnalysis,
			decodeAnalysisMessage,
		),
	}
}

func decodeAnalysisMessage(ctx context.Context, msg kaf.Message) (interface{}, error) {
	req := new(store.Analysis)

	if err := json.Unmarshal(msg.Value, req); err != nil {
		return nil, err
	}

	return req, nil
}

package transports

/*
import (
	"client/pkg/endpoint"
	"context"

	"github.com/project-pncp/private-kit/kafka"
	"github.com/project-pncp/private-kit/pkg/pb/protocols/pncp"
	kafkaGo "github.com/segmentio/kafka-go"
)

func NewKafkaConsumers(endpoints endpoint.EndpointSetup) kafka.Consumers {
	return kafka.Consumers{
		"TEST_ENDPOINT": kafka.NewConsumer(
			endpoints.GetAvailableLicenses,
			decodeTestMessage,
		),
	}
}

func decodeTestMessage(ctx context.Context, msg kafkaGo.Message) (interface{}, error) {
	req := new(pncp.PncpAvailableLicenseRequest)

	if err := kafkaGo.Unmarshal(msg.Value, req); err != nil {
		return nil, err
	}

	return req, nil
}
*/

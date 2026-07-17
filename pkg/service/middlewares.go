package service

import (
	"agents/store"
	"context"

	"github.com/go-kit/log"
	"github.com/newdesksoftwares/private-kit/kafka"
)

type Middleware func(Service) Service

func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw *loggingMiddleware) PostAnalysis(ctx context.Context, request *store.Analysis) (*store.Analysis, error) {
	defer func() {
		mw.logger.Log("method", "PostAnalysis", "status", "completed")
	}()

	mw.logger.Log("method", "PostAnalysis", "status", "started")
	return mw.next.PostAnalysis(ctx, request)
}

func (mw *loggingMiddleware) GetAnalysis(ctx context.Context, request *store.Analysis) (*store.Analysis, error) {
	defer func() {
		mw.logger.Log("method", "GetAnalysis", "status", "completed")
	}()

	mw.logger.Log("method", "GetAnalysis", "status", "started")
	return mw.next.GetAnalysis(ctx, request)
}

func RecoveryMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &recoveryMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type recoveryMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw *recoveryMiddleware) PostAnalysis(ctx context.Context, request *store.Analysis) (*store.Analysis, error) {
	defer func() {
		if r := recover(); r != nil {
			mw.logger.Log("method", "PostAnalysis", "status", "recovered", "error", r)
		}
	}()

	return mw.next.PostAnalysis(ctx, request)
}

func (mw *recoveryMiddleware) GetAnalysis(ctx context.Context, request *store.Analysis) (*store.Analysis, error) {
	defer func() {
		if r := recover(); r != nil {
			mw.logger.Log("method", "GetAnalysis", "status", "recovered", "error", r)
		}
	}()

	return mw.next.GetAnalysis(ctx, request)
}

func EventMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &eventMiddleware{
			next:     next,
			logger:   logger,
			producer: *kafka.NewKafkaProducer(),
		}
	}
}

type eventMiddleware struct {
	next     Service
	producer kafka.KafkaProducer
	logger   log.Logger
}

func (mw *eventMiddleware) PostAnalysis(ctx context.Context, request *store.Analysis) (*store.Analysis, error) {
	return mw.next.PostAnalysis(ctx, request)
}

func (mw *eventMiddleware) GetAnalysis(ctx context.Context, request *store.Analysis) (*store.Analysis, error) {
	return mw.next.GetAnalysis(ctx, request)
}

/*func (mw *eventMiddleware) CreateClient(ctx context.Context, request *store.Client) (v *store.Client, err error) {
	 defer func() {
		mw.logger.Log("method", "GetAvailableLicenses", "sending", "kafka message")

		if err == nil {
			key := strconv.Itoa(int(v.TotalRegistros)) + time.Now().String()
			msg := kafGo.Message{
				Topic: "TEST_ENDPOINT",
				Key:   []byte(key),
				Value: []byte(v.String()),
			}

			mw.producer.Publish(ctx, msg)
		}
	}()

	return mw.next.CreateClient(ctx, request)
}*/

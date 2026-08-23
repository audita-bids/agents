package service

import (
	"agents/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/newdesksoftwares/private-kit/kafka"
	"github.com/redis/go-redis/v9"
)

var t = time.Now()

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

func (mw *recoveryMiddleware) PostAnalysis(ctx context.Context, request *store.Analysis) (analysis *store.Analysis, err error) {
	defer func() {
		if r := recover(); r != nil {
			mw.logger.Log("method", "PostAnalysis", "status", "recovered", "error", r)
			err = fmt.Errorf("recovered from panic: %v", r)
		}
	}()

	return mw.next.PostAnalysis(ctx, request)
}

func (mw *recoveryMiddleware) GetAnalysis(ctx context.Context, request *store.Analysis) (analysis *store.Analysis, err error) {
	defer func() {
		if r := recover(); r != nil {
			mw.logger.Log("method", "GetAnalysis", "status", "recovered", "error", r)
			err = fmt.Errorf("recovered from panic: %v", r)
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

func CacheMiddleware(logger log.Logger, redis *redis.Client) Middleware {
	return func(next Service) Service {
		return &cacheMiddleware{
			next:   next,
			logger: logger,
			redis:  redis,
		}
	}
}

type cacheMiddleware struct {
	next   Service
	logger log.Logger
	redis  *redis.Client
}

func (mw *cacheMiddleware) PostAnalysis(ctx context.Context, request *store.Analysis) (result *store.Analysis, err error) {
	count, err := mw.redis.Get(ctx, request.KeyAnalysisUse()).Int()

	if err == nil && count > 15 {
		return nil, errors.New("you have used AI more than 15x in a day.")
	}

	defer func() {
		if err == nil && result != nil {
			mw.redis.Del(ctx, request.KeyHandles()) // remove handlers.

			mw.redis.Set(ctx, result.Key(), result, 0)

			// we will set here an cache to validate if client used AI more than 15x in a day.
			// we will only use the time.now as identifier to see what day we are. User can use 15x on 0:00am, 5am, 5pm... When he needs. But 15x in a day.
			mw.redis.Incr(ctx, result.KeyAnalysisUse())
		}
	}()

	return mw.next.PostAnalysis(ctx, request)
}

func (mw *cacheMiddleware) GetAnalysis(ctx context.Context, request *store.Analysis) (*store.Analysis, error) {
	cache := mw.redis.Get(ctx, request.Key())

	if cache.Err() == nil {
		var analysis store.Analysis

		if err := json.Unmarshal([]byte(cache.Val()), &analysis); err == nil {
			return &analysis, nil
		}
	}

	return mw.next.GetAnalysis(ctx, request)
}

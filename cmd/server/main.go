package main

import (
	"agents/options"
	"context"
	"net"
	"os"

	"agents/pkg/endpoint"
	"agents/pkg/service"
	"agents/transports"

	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"github.com/project-pncp/private-kit/mongo"
	"github.com/project-pncp/private-kit/pkg/lib"
	"github.com/project-pncp/private-kit/pkg/pb/protocols/agents"
	"google.golang.org/grpc"
)

func main() {
	instanceCfg := options.NewCfg()
	cfg := options.HandleCfg(instanceCfg)

	dbUrl := os.Getenv("MONGODB_URL")

	logger := lib.SetupLogger(cfg.Debug)
	db, err := mongo.NewMongoClient(dbUrl)

	if err != nil {
		level.Error(logger).Log("msg", "failed to connect to mongodb", "err", err)
		os.Exit(1)
	}

	database := db.Database("bids")

	var (
		grpcServer *grpc.Server

		svc         = service.NewService(logger, database)
		endpoints   = endpoint.NewEndpointSetup(svc, logger)
		grpcHandler = transports.NewGRPCServer(*endpoints)
		// kafkaHandler = transports.NewKafkaConsumers(*endpoints)
		// httpHandler = transports.NewHTTPServer(*endpoints, logger)

		// httpServer *httpCaller.Server
	)

	grpcServer = grpc.NewServer(grpc.UnaryInterceptor(lib.RecoveryInterceptor(logger)))
	agents.RegisterAgentsServiceServer(grpcServer, grpcHandler)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var g run.Group
	{
		grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			level.Error(logger).Log("msg", "failed to listen on grpc address", "err", err)
			os.Exit(1)
		}

		if grpcServer != nil {
			level.Info(logger).Log("opened", "grpc", "port", instanceCfg.GRPCAddr)
		}

		g.Add(func() error {
			return grpcServer.Serve(grpcListener)
		}, func(error) {
			grpcServer.GracefulStop()
		})
	}
	/*{
		kafkaListener := kafka.NewListener(
			[]string{"kafka:29092"},
			"pncp",
			kafkaHandler,
		)

		g.Add(func() error {
			level.Info(logger).Log("transport", "kafka", "msg", "consumer started")
			return kafkaListener.Serve(ctx)
		}, func(err error) {
			level.Info(logger).Log("transport", "kafka", "msg", "consumer stopping")
		})
	}*/
	/*{
		httpListener, err := net.Listen("tcp", cfg.HttpAddr)
		if err != nil {
			level.Error(logger).Log("msg", "failed to listen on http address", "err", err)
			os.Exit(1)
		}

		g.Add(func() error {
			strip := http.StripPrefix("/api", httpHandler)

			httpServer = &httpCaller.Server{
				Handler: strip,
			}

			return httpServer.Serve(httpListener)
		}, func(error) {
			level.Error(logger).Log("msg", "failed to listen on http address", "err", err)
		})
	}*/

	if err := g.Run(); err != nil {
		level.Error(logger).Log("msg", "servers failed", "err", err)
		os.Exit(1)
	}
}

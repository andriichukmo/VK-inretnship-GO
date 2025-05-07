package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	servv1 "github.com/andriichukmo/VK-inretnship-GO/api/serv/v1"
	"github.com/andriichukmo/VK-inretnship-GO/internal/config"
	"github.com/andriichukmo/VK-inretnship-GO/internal/server"
	"github.com/andriichukmo/VK-inretnship-GO/internal/subpub"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}
	grpcAddr := cfg.GRPC.Addr
	bufferSize := cfg.Queue.Buffer
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	bus := subpub.NewSubPubWithParams(bufferSize, logger)

	grpcSrv := grpc.NewServer()
	servv1.RegisterPubSubServiceServer(
		grpcSrv,
		server.NewPubSubService(bus, logger),
	)
	reflection.Register(grpcSrv)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatal("failed to listen", zap.Error(err))
	}

	go func() {
		logger.Info("starting gRPC server", zap.String("address", grpcAddr))
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Fatal("failed to serve gRPC", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down gRPC server")
	grpcSrv.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = bus.Close(ctx)
}

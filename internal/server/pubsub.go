package server

import (
	"context"

	servv1 "github.com/andriichukmo/VK-inretnship-GO/api/serv/v1"
	"github.com/andriichukmo/VK-inretnship-GO/internal/subpub"
	"github.com/golang/protobuf/ptypes/empty"
	"go.uber.org/zap"
)

type pubSubService struct {
	servv1.UnimplementedPubSubServiceServer
	bus    subpub.SubPub
	logger *zap.Logger
}

func NewPubSubService(bus subpub.SubPub, logger *zap.Logger) servv1.PubSubServiceServer {
	return &pubSubService{
		bus:    bus,
		logger: logger,
	}
}

func (s *pubSubService) Publish(ctx context.Context, req *servv1.PublishRequest) (*empty.Empty, error) {
	if err := s.bus.Publish(req.Subject, req.Data); err != nil {
		s.logger.Error("failed to publish message", zap.Error(err))
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *pubSubService) Subscribe(req *servv1.SubscribeRequest, stream servv1.PubSubService_SubscribeServer) error {
	sub, err := s.bus.Subscribe(req.Subject, func(msg interface{}) {
		if data, ok := msg.([]byte); ok {
			_ = stream.Send(&servv1.SubscribeResponse{Data: data})
		}
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	<-stream.Context().Done()
	return nil
}

func (s *pubSubService) Health(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

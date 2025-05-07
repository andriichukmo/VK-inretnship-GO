package server_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	servv1 "github.com/andriichukmo/VK-inretnship-GO/api/serv/v1"
	"github.com/andriichukmo/VK-inretnship-GO/internal/server"
	"github.com/andriichukmo/VK-inretnship-GO/internal/subpub"
	"github.com/stretchr/testify/require"
)

const bufSize = 1024 * 1024

func dialer() (*grpc.ClientConn, func(), error) {
	lis := bufconn.Listen(bufSize)
	logger := zap.NewNop()
	bus := subpub.NewSubPubWithParams(64, logger)
	grpcSrv := grpc.NewServer()
	servv1.RegisterPubSubServiceServer(grpcSrv, server.NewPubSubService(bus, logger))
	go grpcSrv.Serve(lis)
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithInsecure())
	cleanup := func() {
		conn.Close()
		grpcSrv.GracefulStop()
		bus.Close(context.Background())
	}
	return conn, cleanup, err
}

func TestPublishSubscribe(t *testing.T) {
	conn, cleanup, err := dialer()
	require.NoError(t, err)
	defer cleanup()

	client := servv1.NewPubSubServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	subject := "test.subject"
	stream, err := client.Subscribe(ctx, &servv1.SubscribeRequest{
		Subject: subject,
	})
	require.NoError(t, err)
	data := []byte("test data")
	_, err = client.Publish(ctx, &servv1.PublishRequest{
		Subject: subject,
		Data:    data,
	})
	require.NoError(t, err)
	resp, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, data, resp.Data)
	cancel()
	_, err = stream.Recv()
	st, ok := status.FromError(err)
	if !(err == io.EOF || (ok && st.Code() == codes.Canceled)) {
		t.Fatal("expected EOF or Canceled, got   ", err)
	}
}

func TestPublishAfterStop(t *testing.T) {
	conn, cleanup, err := dialer()
	require.NoError(t, err)
	defer cleanup()
	client := servv1.NewPubSubServiceClient(conn)
	ctx := context.Background()
	cleanup()
	_, err = client.Publish(ctx, &servv1.PublishRequest{
		Subject: "test.subject",
		Data:    []byte("test data"),
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Condition(t, func() bool {
		return st.Code() == codes.Unavailable || st.Code() == codes.Canceled || err == io.EOF
	}, "expected Unavailable or Canceled, got", st.Code())
}

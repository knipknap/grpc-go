package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"grpc-server-go.localhost/proto"
	"grpc-server-go.localhost/pkg/server"
	"github.com/oklog/oklog/pkg/group"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port = flag.Int("port", 80, "The server port")
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync() // flushes buffer, if any
	sugar := logger.Sugar()
	flag.Parse()

	// clearly demarcates the scope in which each listener/socket may be used.
	var g group.Group
	{
		// The gRPC listener mounts the Go kit gRPC server we created.
		grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
		if err != nil {
			sugar.Errorw("failed to listen grpc", "during", "Listen", "err", err)
			os.Exit(1)
		}
		g.Add(func() error {
			sugar.Infow("grpc address", "addr", fmt.Sprintf(":%d", *port))

			var opts []grpc.ServerOption
			grpcServer := grpc.NewServer(opts...)
			proto.RegisterServiceServer(grpcServer, server.NewServiceServer(logger))
			reflection.Register(grpcServer)
			sugar.Infow("starting server")
			return grpcServer.Serve(grpcListener)
		}, func(error) {
			grpcListener.Close()
		})
	}
	{
		// This function just sits and waits for ctrl-C.
		cancelInterrupt := make(chan struct{})
		g.Add(func() error {
			c := make(chan os.Signal, 1)
			signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
			select {
			case sig := <-c:
				return fmt.Errorf("received signal %s", sig)
			case <-cancelInterrupt:
				return nil
			}
		}, func(error) {
			close(cancelInterrupt)
		})
	}

	sugar.Infow("exit", "reason", g.Run())
}

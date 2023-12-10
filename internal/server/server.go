package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpchealthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	clashv1 "github.com/clash-dev/clash/api/clash/v1"
	C "github.com/clash-dev/clash/internal/constant"
	"github.com/clash-dev/clash/internal/hub"
	"github.com/clash-dev/clash/internal/hub/executor"
)

type Server struct {
	grpc *grpc.Server
}

func New() *Server {
	opts := []grpc.ServerOption{}
	srv := grpc.NewServer(opts...)

	grpchealthv1.RegisterHealthServer(srv, health.NewServer())
	reflection.Register(srv)

	clashv1.RegisterClashServiceServer(srv, &Controller{})

	s := &Server{
		grpc: srv,
	}

	return s
}

func (s *Server) Serve(ctx context.Context) error {
	// ln, err := net.Listen("tcp", ":7788")
	// if err != nil {
	// 	return fmt.Errorf("net.Listen error: %w", err)
	// }

	// if err := s.grpc.Serve(ln); err != nil {
	// 	return fmt.Errorf("grpc.Serve error: %w", err)
	// }

	ctx, cancelFunc := signal.NotifyContext(ctx, os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancelFunc()

	if err := hub.Parse(); err != nil {
		return fmt.Errorf("hub.Parse error: %w", err)
	}

	hupSigCh := make(chan os.Signal, 1)
	signal.Notify(hupSigCh, syscall.SIGHUP)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-hupSigCh:
			if cfg, err := executor.ParseWithPath(C.Path.Config()); err == nil {
				executor.ApplyConfig(cfg, true)
			} else {
				slog.Error("Parse config file failed", slog.Any("error", err), slog.String("config", C.Path.Config()))
			}
		}
	}
}

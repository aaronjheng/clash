package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpchealthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	clashv1 "github.com/clash-dev/clash/api/clash/v1"
	C "github.com/clash-dev/clash/internal/constant"
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
	ctx, cancelFunc := signal.NotifyContext(ctx, os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancelFunc()

	cfg, err := executor.Parse()
	if err != nil {
		return fmt.Errorf("executor.Parse error: %w", err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		addr := cfg.General.APIAddr

		if addr == "" {
			slog.Info("No API address specified.")
		}

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("net.Listen error: %w", err)
		}

		slog.Info("API Server listening", slog.String("address", addr))

		if err := s.grpc.Serve(ln); err != nil {
			return fmt.Errorf("grpc.Serve error: %w", err)
		}

		slog.Info("API Server stopped")

		return nil
	})

	eg.Go(func() error {
		executor.ApplyConfig(cfg, true)

		return nil
	})

	eg.Go(func() error {
		<-ctx.Done()

		s.grpc.GracefulStop()

		return nil
	})

	eg.Go(func() error {
		hupSigCh := make(chan os.Signal, 1)
		signal.Notify(hupSigCh, syscall.SIGHUP)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-hupSigCh:
				slog.Info("Reload config file")
				cfg, err := executor.ParseWithPath(C.Path.Config())
				if err != nil {
					slog.Error("Reload config file failed", slog.Any("error", err), slog.String("config", C.Path.Config()))
					break
				}

				executor.ApplyConfig(cfg, true)
				slog.Info("Reload config file succeeded")
			}
		}
	})

	return eg.Wait()
}

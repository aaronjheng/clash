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
	"github.com/clash-dev/clash/internal/common/observable"
	"github.com/clash-dev/clash/internal/config"
	C "github.com/clash-dev/clash/internal/constant"
	"github.com/clash-dev/clash/internal/hub/executor"
	"github.com/clash-dev/clash/internal/log"
)

type ServerOption struct {
	LoggerProvider *log.LoggerProvider
}

type Server struct {
	logger      *slog.Logger
	logLevelVar *slog.LevelVar

	api *grpc.Server
}

func New(opts *ServerOption) *Server {
	loggerProvider := opts.LoggerProvider

	apiServer := provideApiServer(loggerProvider.Observable())

	s := &Server{
		api:         apiServer,
		logger:      loggerProvider.Logger(),
		logLevelVar: loggerProvider.LevelVar(),
	}

	return s
}

func provideApiServer(logObservable *observable.Observable) *grpc.Server {
	controller := NewController(&ControllerOptions{
		LogObservable: logObservable,
	})

	opts := []grpc.ServerOption{}
	srv := grpc.NewServer(opts...)

	grpchealthv1.RegisterHealthServer(srv, health.NewServer())
	reflection.Register(srv)
	clashv1.RegisterClashServiceServer(srv, controller)

	return srv
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
			s.logger.Info("No API address specified.")
			return nil
		}

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("net.Listen error: %w", err)
		}

		s.logger.Info("API Server listening", slog.String("address", addr))

		if err := s.api.Serve(ln); err != nil {
			return fmt.Errorf("grpc.Serve error: %w", err)
		}

		s.logger.Info("API Server stopped")

		return nil
	})

	eg.Go(func() error {
		s.applyConfig(cfg, true)

		return nil
	})

	eg.Go(func() error {
		<-ctx.Done()

		s.api.GracefulStop()

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
				s.logger.Info("Reload config file")
				cfg, err := executor.ParseWithPath(C.Path.Config())
				if err != nil {
					s.logger.Error("Reload config file failed", slog.Any("error", err), slog.String("config", C.Path.Config()))
					break
				}

				s.applyConfig(cfg, true)
				s.logger.Info("Reload config file succeeded")
			}
		}
	})

	return eg.Wait()
}

func (s *Server) applyConfig(cfg *config.Config, force bool) {
	s.logLevelVar.Set(cfg.General.Logging.Level)

	executor.ApplyConfig(cfg, force)
}

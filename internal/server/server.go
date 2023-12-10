package server

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpchealthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	clashv1 "github.com/Dreamacro/clash/api/clash/v1"
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

func (s *Server) Serve() error {
	ln, err := net.Listen("tcp", ":7788")
	if err != nil {
		return fmt.Errorf("net.Listen error: %w", err)
	}

	if err := s.grpc.Serve(ln); err != nil {
		return fmt.Errorf("grpc.Serve error: %w", err)
	}

	return nil
}

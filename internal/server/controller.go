package server

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/miekg/dns"
	"github.com/samber/lo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	clashv1 "github.com/clash-dev/clash/api/clash/v1"
	"github.com/clash-dev/clash/internal/component/resolver"
	"github.com/clash-dev/clash/internal/constant"
	"github.com/clash-dev/clash/internal/listener"
	"github.com/clash-dev/clash/internal/log"
	"github.com/clash-dev/clash/internal/tunnel"
	"github.com/clash-dev/clash/internal/tunnel/statistic"
	internalversion "github.com/clash-dev/clash/internal/version"
)

type Controller struct {
	clashv1.UnimplementedClashServiceServer
}

func (c *Controller) Version(_ context.Context, _ *emptypb.Empty) (*clashv1.VersionResponse, error) {
	return &clashv1.VersionResponse{
		Version: internalversion.Version,
	}, nil
}

func (c *Controller) SubscribeLogs(req *clashv1.SubscribeLogsRequest, stream clashv1.ClashService_SubscribeLogsServer) error {
	ctx := stream.Context()

	ch := make(chan log.Event, 1024)

	defer func() {
		slog.InfoContext(ctx, "Log subscriber exit")
	}()

	sub := log.Subscribe()
	defer log.UnSubscribe(sub)

	go func() {
		for elm := range sub {
			log := elm.(log.Event)
			select {
			case ch <- log:
			default:
			}
		}
		close(ch)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-ch:
			err := stream.Send(&clashv1.LogRecord{
				Payload: e.Payload,
			})
			if err != nil {
				return fmt.Errorf("stream sending error: %w", err)
			}
		}
	}
}

func (c *Controller) SubscribeTraffics(_ *emptypb.Empty, stream clashv1.ClashService_SubscribeTrafficsServer) error {
	ctx := stream.Context()

	ch := make(chan log.Event, 1024)

	defer func() {
		slog.InfoContext(ctx, "Traffic subscriber exit")
	}()

	sub := log.Subscribe()
	defer log.UnSubscribe(sub)

	go func() {
		for elm := range sub {
			log := elm.(log.Event)
			select {
			case ch <- log:
			default:
			}
		}
		close(ch)
	}()

	t := statistic.DefaultManager

	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			up, down := t.Now()

			err := stream.Send(&clashv1.Traffic{
				Up:   up,
				Down: down,
			})
			if err != nil {
				return fmt.Errorf("stream sending error: %w", err)
			}
		}
	}
}

func (c *Controller) ListRules(_ context.Context, _ *emptypb.Empty) (*clashv1.ListRulesResponse, error) {
	rawRules := tunnel.Rules()

	cnt := len(rawRules)
	rules := make([]*clashv1.ListRulesResponse_Rule, 0, cnt)
	for _, r := range rawRules {
		rules = append(rules, &clashv1.ListRulesResponse_Rule{
			Type:    r.RuleType().String(),
			Payload: r.Payload(),
			Proxy:   r.Adapter(),
		})
	}

	return &clashv1.ListRulesResponse{
		Rules: rules,
	}, nil
}

func (c *Controller) ListInbounds(_ context.Context, _ *emptypb.Empty) (*clashv1.ListInboundsResponse, error) {
	rawInbounds := listener.GetInbounds()

	cnt := len(rawInbounds)
	inbounds := make([]*clashv1.ListInboundsResponse_Inbound, 0, cnt)
	for _, i := range rawInbounds {
		inbounds = append(inbounds, &clashv1.ListInboundsResponse_Inbound{
			Type:        string(i.Type),
			BindAddress: i.BindAddress,
		})
	}

	return &clashv1.ListInboundsResponse{
		Inbounds: inbounds,
	}, nil
}

func (c *Controller) BatchUpdateInbounds(ctx context.Context, req *clashv1.BatchUpdateInboundsRequest) (*emptypb.Empty, error) {
	updates := req.GetInbounds()
	cnt := len(updates)

	inbounds := make([]constant.Inbound, 0, cnt)
	for _, i := range updates {
		inbounds = append(inbounds, constant.Inbound{
			Type:        constant.InboundType(i.Type),
			BindAddress: i.BindAddress,
		})
	}

	listener.ReCreateListeners(inbounds, tunnel.TCPIn(), tunnel.UDPIn())

	return nil, nil
}

func (c *Controller) SubscribeConnections(req *clashv1.SubscribeConnectionsRequest, stream clashv1.ClashService_SubscribeConnectionsServer) error {
	ctx := stream.Context()

	defer func() {
		slog.InfoContext(ctx, "Connection subscriber exit")
	}()

	interval := req.GetInternal()
	ticker := time.NewTicker(interval.AsDuration())

	snapshot := func() *clashv1.SubscribeConnectionsResponse {
		snapshot := statistic.DefaultManager.Snapshot()

		resp := &clashv1.SubscribeConnectionsResponse{
			DownloadTotal: snapshot.DownloadTotal,
			UploadTotal:   snapshot.UploadTotal,
			Connections:   []*clashv1.SubscribeConnectionsResponse_Connection{},
		}

		// TODO: Extract tracker info

		return resp
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := stream.Send(snapshot()); err != nil {
				if err != nil {
					return fmt.Errorf("stream sending error: %w", err)
				}
			}
		}
	}
}

func (c *Controller) DeleteConnection(ctx context.Context, req *clashv1.DeleteConnectionRequest) (*emptypb.Empty, error) {
	id := req.GetId()

	snapshot := statistic.DefaultManager.Snapshot()
	for _, c := range snapshot.Connections {
		if id == c.ID() {
			c.Close()
			break
		}
	}

	return &emptypb.Empty{}, nil
}

func (c *Controller) CloseAllConnections(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	snapshot := statistic.DefaultManager.Snapshot()
	for _, c := range snapshot.Connections {
		c.Close()
	}

	return &emptypb.Empty{}, nil
}

func (c *Controller) QueryDNS(ctx context.Context, req *clashv1.QueryDNSRequest) (*clashv1.QueryDNSResponse, error) {
	if resolver.DefaultResolver == nil {
		return nil, status.New(codes.Unavailable, "Resolver unavailable").Err()
	}

	name := req.GetName()
	recordTypeStr := req.GetType()

	recordType, exist := dns.StringToType[recordTypeStr]
	if !exist {
		return nil, status.New(codes.InvalidArgument, "invalid type").Err()
	}

	msg := &dns.Msg{}
	msg.SetQuestion(dns.Fqdn(name), recordType)

	msg, err := resolver.DefaultResolver.ExchangeContext(ctx, msg)
	if err != nil {
		return nil, status.Newf(codes.Internal, "Resolver query failed: %s", err).Err()
	}

	question := make([]*clashv1.QueryDNSResponse_Question, 0, len(msg.Question))
	for _, q := range msg.Question {
		question = append(question, &clashv1.QueryDNSResponse_Question{
			Name:  q.Name,
			Type:  int32(q.Qtype),
			Class: int32(q.Qclass),
		})
	}

	toAnswer := func(rr dns.RR, _ int) *clashv1.QueryDNSResponse_Answer {
		header := rr.Header()
		return &clashv1.QueryDNSResponse_Answer{
			Name: header.Name,
			Type: int32(header.Rrtype),
			Ttl:  int64(header.Ttl),
			Data: lo.Substring(rr.String(), len(header.String()), math.MaxUint),
		}
	}

	// TODO: Assembly response
	resp := &clashv1.QueryDNSResponse{
		Status:             int32(msg.Rcode),
		Question:           question,
		Truncated:          msg.Truncated,
		RecursionDesired:   msg.RecursionDesired,
		RecursionAvailable: msg.RecursionAvailable,
		AuthenticatedData:  msg.AuthenticatedData,
		CheckingDisabled:   msg.CheckingDisabled,

		Answer:    lo.Map(msg.Answer, toAnswer),
		Authority: lo.Map(msg.Ns, toAnswer),
	}

	return resp, nil
}

// For interface stubs generation. Do not remove.
// c *Controller github.com/clash-dev/clash/api/clash/v1.ClashServiceServer

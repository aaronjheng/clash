package listener

import (
	"errors"
	"fmt"
	"net"

	"github.com/clash-dev/clash/internal/adapter/inbound"
	C "github.com/clash-dev/clash/internal/constant"
	"github.com/clash-dev/clash/internal/listener/http"
	"github.com/clash-dev/clash/internal/listener/mixed"
	"github.com/clash-dev/clash/internal/listener/redir"
	"github.com/clash-dev/clash/internal/listener/socks"
	"github.com/clash-dev/clash/internal/listener/tproxy"
)

type Ports struct {
	Port       int `json:"port"`
	SocksPort  int `json:"socks-port"`
	RedirPort  int `json:"redir-port"`
	TProxyPort int `json:"tproxy-port"`
	MixedPort  int `json:"mixed-port"`
}

var tcpListenerCreators = map[C.InboundType]tcpListenerCreator{
	C.InboundTypeHTTP:   http.New,
	C.InboundTypeSocks:  socks.New,
	C.InboundTypeRedir:  redir.New,
	C.InboundTypeTproxy: tproxy.New,
	C.InboundTypeMixed:  mixed.New,
}

var udpListenerCreators = map[C.InboundType]udpListenerCreator{
	C.InboundTypeSocks:  socks.NewUDP,
	C.InboundTypeRedir:  tproxy.NewUDP,
	C.InboundTypeTproxy: tproxy.NewUDP,
	C.InboundTypeMixed:  socks.NewUDP,
}

type (
	tcpListenerCreator func(addr string, tcpIn chan<- C.ConnContext) (C.Listener, error)
	udpListenerCreator func(addr string, udpIn chan<- *inbound.PacketAdapter) (C.Listener, error)
)

var (
	ErrInvalidPort            = errors.New("invalid port")
	ErrUnsupportedInboundType = errors.New("unsupported inbound type")
)

func New(inbound C.Inbound, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) (C.Listener, C.Listener, error) {
	addr := inbound.BindAddress
	if portIsZero(addr) {
		return nil, nil, ErrInvalidPort
	}

	tcpCreator := tcpListenerCreators[inbound.Type]
	udpCreator := udpListenerCreators[inbound.Type]
	if tcpCreator == nil && udpCreator == nil {
		return nil, nil, ErrUnsupportedInboundType
	}

	var tcpListener, udpListener C.Listener

	if tcpCreator != nil {
		var err error
		tcpListener, err = tcpCreator(addr, tcpIn)
		if err != nil {
			return nil, nil, fmt.Errorf("create tcp listener error: %w", err)
		}
	}
	if udpCreator != nil {
		var err error
		udpListener, err = udpCreator(addr, udpIn)
		if err != nil {
			return nil, nil, fmt.Errorf("create udp listener error: %w", err)
		}
	}

	return tcpListener, udpListener, nil
}

func portIsZero(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if port == "0" || port == "" || err != nil {
		return true
	}
	return false
}

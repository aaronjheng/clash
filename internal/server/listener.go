package server

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/samber/lo"

	"github.com/clash-dev/clash/internal/adapter/inbound"
	"github.com/clash-dev/clash/internal/config"
	C "github.com/clash-dev/clash/internal/constant"
	"github.com/clash-dev/clash/internal/listener"
	"github.com/clash-dev/clash/internal/listener/driver"
	"github.com/clash-dev/clash/internal/listener/tunnel"
	"github.com/clash-dev/clash/internal/log"
)

type ListenerManager struct {
	allowLan    bool
	bindAddress string

	tcpListeners map[C.Inbound]driver.Listener
	udpListeners map[C.Inbound]driver.Listener

	tunnelTCPListeners map[string]*tunnel.Listener
	tunnelUDPListeners map[string]*tunnel.PacketConn

	// lock for recreate function
	recreateMux sync.Mutex
	tunnelMux   sync.Mutex
}

func NewListenerManager() *ListenerManager {
	return &ListenerManager{
		allowLan:           false,
		bindAddress:        "*",
		tcpListeners:       map[C.Inbound]driver.Listener{},
		udpListeners:       map[C.Inbound]driver.Listener{},
		tunnelTCPListeners: map[string]*tunnel.Listener{},
		tunnelUDPListeners: map[string]*tunnel.PacketConn{},
	}
}

type Ports struct {
	Port       int `json:"port"`
	SocksPort  int `json:"socks-port"`
	RedirPort  int `json:"redir-port"`
	TProxyPort int `json:"tproxy-port"`
	MixedPort  int `json:"mixed-port"`
}

func (l *ListenerManager) closeListener(inbound C.Inbound) {
	listener := l.tcpListeners[inbound]
	if listener != nil {
		if err := listener.Close(); err != nil {
			log.Errorln("close tcp address `%s` error: %s", inbound.ToAlias(), err.Error())
		}
		delete(l.tcpListeners, inbound)
	}
	listener = l.udpListeners[inbound]
	if listener != nil {
		if err := listener.Close(); err != nil {
			log.Errorln("close udp address `%s` error: %s", inbound.ToAlias(), err.Error())
		}
		delete(l.udpListeners, inbound)
	}
}

func (l *ListenerManager) getNeedCloseAndCreateInbound(originInbounds []C.Inbound, newInbounds []C.Inbound) ([]C.Inbound, []C.Inbound) {
	needCloseMap := map[C.Inbound]bool{}
	needClose := []C.Inbound{}
	needCreate := []C.Inbound{}

	for _, inbound := range originInbounds {
		needCloseMap[inbound] = true
	}
	for _, inbound := range newInbounds {
		if needCloseMap[inbound] {
			delete(needCloseMap, inbound)
		} else {
			needCreate = append(needCreate, inbound)
		}
	}
	for inbound := range needCloseMap {
		needClose = append(needClose, inbound)
	}
	return needClose, needCreate
}

// only recreate inbound config listener
func (l *ListenerManager) ReCreateListeners(inbounds []C.Inbound, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) {
	newInbounds := []C.Inbound{}
	newInbounds = append(newInbounds, inbounds...)
	for _, inbound := range l.getInbounds() {
		if inbound.IsFromPortCfg {
			newInbounds = append(newInbounds, inbound)
		}
	}

	l.reCreateListeners(newInbounds, tcpIn, udpIn)
}

// only recreate ports config listener
func (l *ListenerManager) recreatePortsListeners(ports Ports, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) {
	newInbounds := []C.Inbound{}
	newInbounds = append(newInbounds, l.GetInbounds()...)
	newInbounds = l.addPortInbound(newInbounds, C.InboundTypeHTTP, ports.Port)
	newInbounds = l.addPortInbound(newInbounds, C.InboundTypeSocks, ports.SocksPort)
	newInbounds = l.addPortInbound(newInbounds, C.InboundTypeRedir, ports.RedirPort)
	newInbounds = l.addPortInbound(newInbounds, C.InboundTypeTproxy, ports.TProxyPort)
	newInbounds = l.addPortInbound(newInbounds, C.InboundTypeMixed, ports.MixedPort)
	l.reCreateListeners(newInbounds, tcpIn, udpIn)
}

func (l *ListenerManager) addPortInbound(inbounds []C.Inbound, inboundType C.InboundType, port int) []C.Inbound {
	if port != 0 {
		inbounds = append(inbounds, C.Inbound{
			Type:          inboundType,
			BindAddress:   l.genAddr(l.bindAddress, port, l.allowLan),
			IsFromPortCfg: true,
		})
	}
	return inbounds
}

func (l *ListenerManager) reCreateListeners(inbounds []C.Inbound, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) {
	l.recreateMux.Lock()
	defer l.recreateMux.Unlock()

	needClose, needCreate := l.getNeedCloseAndCreateInbound(l.getInbounds(), inbounds)
	for _, inbound := range needClose {
		l.closeListener(inbound)
	}

	for _, inbound := range needCreate {
		tcpListener, udpListener, err := listener.OpenListener(inbound, tcpIn, udpIn)
		if err != nil {
			slog.Error("Create listener failed", slog.Any("error", err))
		}

		if tcpListener != nil {
			l.tcpListeners[inbound] = tcpListener
		}

		if udpListener != nil {
			l.udpListeners[inbound] = udpListener
		}

		slog.Info("Inbound created", slog.String("inbound", inbound.ToAlias()))
	}
}

func (l *ListenerManager) patchTunnel(tunnels []config.Tunnel, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) {
	l.tunnelMux.Lock()
	defer l.tunnelMux.Unlock()

	type addrProxy struct {
		network string
		addr    string
		target  string
		proxy   string
	}

	tcpOld := lo.Map(
		lo.Keys(l.tunnelTCPListeners),
		func(key string, _ int) addrProxy {
			parts := strings.Split(key, "/")
			return addrProxy{
				network: "tcp",
				addr:    parts[0],
				target:  parts[1],
				proxy:   parts[2],
			}
		},
	)
	udpOld := lo.Map(
		lo.Keys(l.tunnelUDPListeners),
		func(key string, _ int) addrProxy {
			parts := strings.Split(key, "/")
			return addrProxy{
				network: "udp",
				addr:    parts[0],
				target:  parts[1],
				proxy:   parts[2],
			}
		},
	)
	oldElm := lo.Union(tcpOld, udpOld)

	newElm := lo.FlatMap(
		tunnels,
		func(tunnel config.Tunnel, _ int) []addrProxy {
			return lo.Map(
				tunnel.Network,
				func(network string, _ int) addrProxy {
					return addrProxy{
						network: network,
						addr:    tunnel.Address,
						target:  tunnel.Target,
						proxy:   tunnel.Proxy,
					}
				},
			)
		},
	)

	needClose, needCreate := lo.Difference(oldElm, newElm)

	for _, elm := range needClose {
		key := fmt.Sprintf("%s/%s/%s", elm.addr, elm.target, elm.proxy)
		if elm.network == "tcp" {
			l.tunnelTCPListeners[key].Close()
			delete(l.tunnelTCPListeners, key)
		} else {
			l.tunnelUDPListeners[key].Close()
			delete(l.tunnelUDPListeners, key)
		}
	}

	for _, elm := range needCreate {
		key := fmt.Sprintf("%s/%s/%s", elm.addr, elm.target, elm.proxy)
		if elm.network == "tcp" {
			t, err := tunnel.New(elm.addr, elm.target, elm.proxy, tcpIn)
			if err != nil {
				log.Errorln("Start tunnel %s error: %s", elm.target, err.Error())
				continue
			}
			l.tunnelTCPListeners[key] = t
			log.Infoln("Tunnel(tcp/%s) proxy %s listening at: %s", elm.target, elm.proxy, l.tunnelTCPListeners[key].Address())
		} else {
			t, err := tunnel.NewUDP(elm.addr, elm.target, elm.proxy, udpIn)
			if err != nil {
				log.Errorln("Start tunnel %s error: %s", elm.target, err.Error())
				continue
			}
			l.tunnelUDPListeners[key] = t
			log.Infoln("Tunnel(udp/%s) proxy %s listening at: %s", elm.target, elm.proxy, l.tunnelUDPListeners[key].Address())
		}
	}
}

func (l *ListenerManager) GetInbounds() []C.Inbound {
	return lo.Filter(l.getInbounds(), func(inbound C.Inbound, idx int) bool {
		return !inbound.IsFromPortCfg
	})
}

// GetInbounds return the inbounds of proxy servers
func (l *ListenerManager) getInbounds() []C.Inbound {
	var inbounds []C.Inbound

	for inbound := range l.tcpListeners {
		inbounds = append(inbounds, inbound)
	}

	for inbound := range l.udpListeners {
		if _, ok := l.tcpListeners[inbound]; !ok {
			inbounds = append(inbounds, inbound)
		}
	}

	return inbounds
}

func (l *ListenerManager) genAddr(host string, port int, allowLan bool) string {
	if allowLan {
		if host == "*" {
			return fmt.Sprintf(":%d", port)
		}
		return fmt.Sprintf("%s:%d", host, port)
	}

	return fmt.Sprintf("127.0.0.1:%d", port)
}

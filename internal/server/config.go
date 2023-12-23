package server

import (
	"sync"

	"github.com/clash-dev/clash/internal/adapter"
	"github.com/clash-dev/clash/internal/adapter/outboundgroup"
	"github.com/clash-dev/clash/internal/component/auth"
	"github.com/clash-dev/clash/internal/component/dialer"
	"github.com/clash-dev/clash/internal/component/iface"
	"github.com/clash-dev/clash/internal/component/profile"
	"github.com/clash-dev/clash/internal/component/profile/cachefile"
	"github.com/clash-dev/clash/internal/component/resolver"
	"github.com/clash-dev/clash/internal/component/trie"
	"github.com/clash-dev/clash/internal/config"
	C "github.com/clash-dev/clash/internal/constant"
	"github.com/clash-dev/clash/internal/constant/provider"
	"github.com/clash-dev/clash/internal/dns"
	authStore "github.com/clash-dev/clash/internal/listener/auth"
	"github.com/clash-dev/clash/internal/log"
	"github.com/clash-dev/clash/internal/tunnel"
)

var mux sync.Mutex

func loadConfig() (*config.Config, error) {
	return config.Load(C.Path.Config())
}

func (s *Server) updateExperimental(c *config.Config) {
	tunnel.UDPFallbackMatch.Store(c.Experimental.UDPFallbackMatch)
}

func (s *Server) updateDNS(c *config.DNS) {
	if !c.Enable {
		resolver.DefaultResolver = nil
		resolver.DefaultHostMapper = nil
		dns.ReCreateServer("", nil, nil)
		return
	}

	cfg := dns.Config{
		Main:         c.NameServer,
		Fallback:     c.Fallback,
		IPv6:         c.IPv6,
		EnhancedMode: c.EnhancedMode,
		Pool:         c.FakeIPRange,
		Hosts:        c.Hosts,
		FallbackFilter: dns.FallbackFilter{
			GeoIP:     c.FallbackFilter.GeoIP,
			GeoIPCode: c.FallbackFilter.GeoIPCode,
			IPCIDR:    c.FallbackFilter.IPCIDR,
			Domain:    c.FallbackFilter.Domain,
		},
		Default:       c.DefaultNameserver,
		Policy:        c.NameServerPolicy,
		SearchDomains: c.SearchDomains,
	}

	r := dns.NewResolver(cfg)
	m := dns.NewEnhancer(cfg)

	// reuse cache of old host mapper
	if old := resolver.DefaultHostMapper; old != nil {
		m.PatchFrom(old.(*dns.ResolverEnhancer))
	}

	resolver.DefaultResolver = r
	resolver.DefaultHostMapper = m

	dns.ReCreateServer(c.Listen, r, m)
}

func (s *Server) updateHosts(tree *trie.DomainTrie) {
	resolver.DefaultHosts = tree
}

func (s *Server) updateProxies(proxies map[string]C.Proxy, providers map[string]provider.ProxyProvider) {
	tunnel.UpdateProxies(proxies, providers)
}

func (s *Server) updateRules(rules []C.Rule) {
	tunnel.UpdateRules(rules)
}

func (s *Server) updateTunnels(tunnels []config.Tunnel) {
	s.listenerManager.patchTunnel(tunnels, tunnel.TCPIn(), tunnel.UDPIn())
}

func (s *Server) updateInbounds(inbounds []C.Inbound, force bool) {
	if !force {
		return
	}
	tcpIn := tunnel.TCPIn()
	udpIn := tunnel.UDPIn()

	s.listenerManager.ReCreateListeners(inbounds, tcpIn, udpIn)
}

func (s *Server) updateGeneral(general *config.General, force bool) {
	tunnel.SetMode(general.Mode)
	resolver.DisableIPv6 = !general.IPv6

	dialer.DefaultInterface.Store(general.Interface)
	dialer.DefaultRoutingMark.Store(int32(general.RoutingMark))

	iface.FlushCache()

	if !force {
		return
	}

	// allowLan := general.AllowLan
	// listener.SetAllowLan(allowLan)

	// bindAddress := general.BindAddress
	// listener.SetBindAddress(bindAddress)

	ports := Ports{
		Port:       general.Port,
		SocksPort:  general.SocksPort,
		RedirPort:  general.RedirPort,
		TProxyPort: general.TProxyPort,
		MixedPort:  general.MixedPort,
	}

	s.listenerManager.recreatePortsListeners(ports, tunnel.TCPIn(), tunnel.UDPIn())
}

func (s *Server) updateUsers(users []auth.AuthUser) {
	authenticator := auth.NewAuthenticator(users)
	authStore.SetAuthenticator(authenticator)
	if authenticator != nil {
		log.Infoln("Authentication of local server updated")
	}
}

func (s *Server) updateProfile(cfg *config.Config) {
	profileCfg := cfg.Profile

	profile.StoreSelected.Store(profileCfg.StoreSelected)
	if profileCfg.StoreSelected {
		patchSelectGroup(cfg.Proxies)
	}
}

func patchSelectGroup(proxies map[string]C.Proxy) {
	mapping := cachefile.Cache().SelectedMap()
	if mapping == nil {
		return
	}

	for name, proxy := range proxies {
		outbound, ok := proxy.(*adapter.Proxy)
		if !ok {
			continue
		}

		selector, ok := outbound.ProxyAdapter.(*outboundgroup.Selector)
		if !ok {
			continue
		}

		selected, exist := mapping[name]
		if !exist {
			continue
		}

		selector.Set(selected)
	}
}

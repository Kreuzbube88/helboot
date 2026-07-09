package main

import (
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/config"
	"github.com/kreuzbube88/helboot/backend/internal/network"
	"github.com/kreuzbube88/helboot/backend/internal/service"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// Settings keys read here; written by the network config API.
const (
	settingServerIP     = "network.server_ip"
	settingDHCPStart    = "network.dhcp.range_start"
	settingDHCPEnd      = "network.dhcp.range_end"
	settingDHCPMask     = "network.dhcp.subnet_mask"
	settingDHCPGateway  = "network.dhcp.gateway"
	settingDHCPDNS      = "network.dhcp.dns"
	settingDHCPLeaseMin = "network.dhcp.lease_minutes"
)

// buildNetworkServices assembles the boot-network services for the
// configured mode (ADR-0006). Before the first-run wizard completed, no
// network services run — only the web UI is up. The observer receives
// DHCP sightings for rogue-server detection (ADR-0016).
func buildNetworkServices(cfg config.Config, st *store.Store, log *slog.Logger, observer *network.DHCPObserver) []service.Service {
	completed, err := st.SetupCompleted()
	if err != nil {
		log.Error("cannot read setup state; network services disabled", "error", err)
		return nil
	}
	if !completed {
		log.Info("setup not completed yet; boot network services stay off")
		return nil
	}

	serverIP := detectServerIP(getSetting(st, settingServerIP), log)
	if serverIP == nil {
		log.Error("no usable IPv4 address found; boot network services disabled",
			"hint", "set the server IP in the network settings")
		return nil
	}
	observer.SetSelfIP(serverIP)
	bootCfg := network.BootConfig{
		ServerIP:  serverIP,
		ScriptURL: fmt.Sprintf("http://%s/boot/ipxe", net.JoinHostPort(serverIP.String(), httpPort(cfg.HTTPAddr))),
	}

	services := []service.Service{
		network.NewTFTPServer(log, filepath.Join(cfg.AssetsPath(), "tftp")),
	}

	mode, _ := st.GetSetting(store.SettingNetworkMode)
	switch mode {
	case "dhcp":
		if dhcpSrv := buildDHCPServer(st, log, bootCfg); dhcpSrv != nil {
			dhcpSrv.Observer = observer
			services = append(services, dhcpSrv)
		}
	default: // proxy_dhcp is the safe default (never assigns addresses)
		proxy := network.NewProxyDHCP(log, bootCfg)
		proxy.Observer = observer
		services = append(services, proxy)
	}
	return services
}

func buildDHCPServer(st *store.Store, log *slog.Logger, bootCfg network.BootConfig) *network.DHCPServer {
	start := net.ParseIP(getSetting(st, settingDHCPStart))
	end := net.ParseIP(getSetting(st, settingDHCPEnd))
	if start == nil || end == nil {
		log.Error("DHCP mode selected but no address range configured; DHCP service disabled",
			"hint", "configure the range in the network settings")
		return nil
	}

	leaseMinutes, _ := strconv.Atoi(getSetting(st, settingDHCPLeaseMin))
	if leaseMinutes <= 0 {
		leaseMinutes = 60
	}
	pool, err := network.NewLeasePool(start, end, time.Duration(leaseMinutes)*time.Minute, leaseAdapter{st})
	if err != nil {
		log.Error("invalid DHCP range; DHCP service disabled", "error", err)
		return nil
	}
	// Re-load persisted leases so clients keep their addresses across
	// restarts.
	leases, err := st.ActiveLeases(time.Now())
	if err != nil {
		log.Warn("could not restore DHCP leases", "error", err)
	}
	for _, l := range leases {
		pool.Restore(l.MAC, net.ParseIP(l.IP), l.ExpiresAt)
	}

	mask := net.IPMask(net.ParseIP(getSetting(st, settingDHCPMask)).To4())
	if mask == nil {
		mask = net.CIDRMask(24, 32)
		log.Warn("no subnet mask configured, defaulting to /24")
	}
	var dns []net.IP
	for _, s := range strings.Split(getSetting(st, settingDHCPDNS), ",") {
		if ip := net.ParseIP(strings.TrimSpace(s)); ip != nil {
			dns = append(dns, ip)
		}
	}
	return network.NewDHCPServer(log, network.DHCPConfig{
		Boot:       bootCfg,
		SubnetMask: mask,
		Gateway:    net.ParseIP(getSetting(st, settingDHCPGateway)),
		DNS:        dns,
		Pool:       pool,
	})
}

// leaseAdapter bridges the store to the pool's persistence interface.
type leaseAdapter struct{ st *store.Store }

func (a leaseAdapter) UpsertLease(mac, ip, hostname string, expiresAt time.Time) error {
	return a.st.UpsertLease(store.Lease{MAC: mac, IP: ip, Hostname: hostname, ExpiresAt: expiresAt})
}

func (a leaseAdapter) DeleteLease(mac string) error { return a.st.DeleteLease(mac) }

func getSetting(st *store.Store, key string) string {
	v, err := st.GetSetting(key)
	if err != nil {
		return ""
	}
	return v
}

// detectServerIP returns the configured server IP, or the first global
// unicast IPv4 of the host as a sensible default.
func detectServerIP(configured string, log *slog.Logger) net.IP {
	if ip := net.ParseIP(configured); ip != nil {
		return ip.To4()
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Error("cannot enumerate network interfaces", "error", err)
		return nil
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP.To4()
		if ip != nil && ip.IsGlobalUnicast() {
			return ip
		}
	}
	return nil
}

// httpPort extracts the port from a listen address like ":8080".
func httpPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return "8080"
	}
	return port
}

package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
)

// ProxyDHCP implements Mode A (ADR-0006): the existing DHCP server keeps
// assigning addresses; HELBOOT answers only the PXE part of the
// conversation. It listens on UDP 67 (alongside broadcast traffic) and
// on UDP 4011 (the PXE boot service port BIOS ROMs use for the follow-up
// request).
type ProxyDHCP struct {
	log *slog.Logger
	cfg BootConfig
	// Observer, when set, records foreign DHCP servers seen in client
	// broadcasts (ADR-0016). Nil-safe.
	Observer *DHCPObserver
	// ports are configurable for tests; zero values mean 67/4011.
	port67, port4011 int
}

// NewProxyDHCP creates the Mode A service.
func NewProxyDHCP(log *slog.Logger, cfg BootConfig) *ProxyDHCP {
	return &ProxyDHCP{log: log, cfg: cfg, port67: 67, port4011: 4011}
}

// Name implements service.Service.
func (p *ProxyDHCP) Name() string { return "proxydhcp" }

// Run starts both listeners and blocks until ctx is cancelled.
func (p *ProxyDHCP) Run(ctx context.Context) error {
	srv67, err := server4.NewServer("", &net.UDPAddr{IP: net.IPv4zero, Port: p.port67},
		p.handler(true))
	if err != nil {
		return fmt.Errorf("proxydhcp: listen :%d: %w", p.port67, err)
	}
	srv4011, err := server4.NewServer("", &net.UDPAddr{IP: net.IPv4zero, Port: p.port4011},
		p.handler(false))
	if err != nil {
		srv67.Close()
		return fmt.Errorf("proxydhcp: listen :%d: %w", p.port4011, err)
	}

	errCh := make(chan error, 2)
	go func() { errCh <- srv67.Serve() }()
	go func() { errCh <- srv4011.Serve() }()

	select {
	case <-ctx.Done():
		srv67.Close()
		srv4011.Close()
		return ctx.Err()
	case err := <-errCh:
		srv67.Close()
		srv4011.Close()
		return fmt.Errorf("proxydhcp: %w", err)
	}
}

// handler answers PXE requests. On port 67 only DISCOVERs are answered
// (with a ProxyDHCP OFFER carrying no IP address); on 4011 REQUESTs get
// the final ACK with the boot file.
func (p *ProxyDHCP) handler(isPort67 bool) server4.Handler {
	return func(conn net.PacketConn, peer net.Addr, req *dhcpv4.DHCPv4) {
		if isPort67 {
			p.Observer.Observe(req)
		}
		reply := p.buildReply(req, isPort67)
		if reply == nil {
			return
		}
		if _, err := conn.WriteTo(reply.ToBytes(), replyAddr(peer, req)); err != nil {
			p.log.Warn("proxydhcp: send reply failed", "peer", peer.String(), "error", err)
			return
		}
		p.log.Info("proxydhcp: served PXE client",
			"mac", req.ClientHWAddr.String(),
			"arch", FirmwareOf(req),
			"bootfile", reply.BootFileName)
	}
}

// buildReply computes the response for a PXE request, or nil when the
// message must be ignored. Split out for testability.
func (p *ProxyDHCP) buildReply(req *dhcpv4.DHCPv4, isPort67 bool) *dhcpv4.DHCPv4 {
	if req.OpCode != dhcpv4.OpcodeBootRequest || !isPXERequest(req) {
		return nil
	}
	msgType := req.MessageType()
	if isPort67 && msgType != dhcpv4.MessageTypeDiscover {
		return nil // the real DHCP server handles address assignment
	}
	if !isPort67 && msgType != dhcpv4.MessageTypeRequest {
		return nil
	}

	reply, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		p.log.Warn("proxydhcp: cannot build reply", "error", err)
		return nil
	}
	if isPort67 {
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
	} else {
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
	}
	// ProxyDHCP never assigns addresses: yiaddr stays 0.0.0.0.
	reply.YourIPAddr = net.IPv4zero
	reply.UpdateOption(dhcpv4.OptServerIdentifier(p.cfg.ServerIP))
	if err := p.cfg.applyPXEOptions(reply, req); err != nil {
		p.log.Debug("proxydhcp: ignoring client", "mac", req.ClientHWAddr.String(), "reason", err)
		return nil
	}
	return reply
}

// replyAddr picks the destination for a reply: broadcast for clients
// that don't have an address yet (RFC 2131 §4.1), unicast otherwise.
func replyAddr(peer net.Addr, req *dhcpv4.DHCPv4) net.Addr {
	udp, ok := peer.(*net.UDPAddr)
	if !ok || udp.IP == nil || udp.IP.IsUnspecified() || req.IsBroadcast() {
		return &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
	}
	return peer
}

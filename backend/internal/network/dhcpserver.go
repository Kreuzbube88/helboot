package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
)

// DHCPConfig configures the authoritative Mode B server (ADR-0006).
type DHCPConfig struct {
	Boot       BootConfig
	SubnetMask net.IPMask
	Gateway    net.IP // optional
	DNS        []net.IP
	Pool       *LeasePool
}

// DHCPServer is HELBOOT's own DHCP server for networks without one
// (Mode B). PXE options are attached to every offer/ack so a separate
// ProxyDHCP service is unnecessary in this mode.
type DHCPServer struct {
	log *slog.Logger
	cfg DHCPConfig
	// Observer, when set, records foreign DHCP servers answering on
	// HELBOOT's segment (ADR-0016). Nil-safe.
	Observer *DHCPObserver
	port     int
}

// NewDHCPServer creates the Mode B service.
func NewDHCPServer(log *slog.Logger, cfg DHCPConfig) *DHCPServer {
	return &DHCPServer{log: log, cfg: cfg, port: dhcpv4.ServerPort}
}

// Name implements service.Service.
func (d *DHCPServer) Name() string { return "dhcp" }

// Run starts the listener and blocks until ctx is cancelled.
func (d *DHCPServer) Run(ctx context.Context) error {
	srv, err := server4.NewServer("", &net.UDPAddr{IP: net.IPv4zero, Port: d.port}, d.handle)
	if err != nil {
		return fmt.Errorf("dhcp: listen :%d: %w", d.port, err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve() }()
	select {
	case <-ctx.Done():
		srv.Close()
		return ctx.Err()
	case err := <-errCh:
		srv.Close()
		return fmt.Errorf("dhcp: %w", err)
	}
}

func (d *DHCPServer) handle(conn net.PacketConn, peer net.Addr, req *dhcpv4.DHCPv4) {
	d.Observer.Observe(req)
	reply := d.buildReply(req)
	if reply == nil {
		return
	}
	if _, err := conn.WriteTo(reply.ToBytes(), replyAddr(peer, req)); err != nil {
		d.log.Warn("dhcp: send reply failed", "peer", peer.String(), "error", err)
		return
	}
	d.log.Debug("dhcp: reply sent", "mac", req.ClientHWAddr.String(),
		"type", reply.MessageType().String(), "ip", reply.YourIPAddr.String())
}

// buildReply implements the DISCOVER/REQUEST/RELEASE state machine.
// Split out for testability; returns nil when no reply must be sent.
func (d *DHCPServer) buildReply(req *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		return nil
	}
	mac := req.ClientHWAddr.String()

	switch req.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		ip := d.cfg.Pool.Allocate(mac, req.RequestedIPAddress(), req.HostName())
		if ip == nil {
			d.log.Warn("dhcp: pool exhausted", "mac", mac)
			return nil
		}
		return d.reply(req, dhcpv4.MessageTypeOffer, ip)

	case dhcpv4.MessageTypeRequest:
		requested := req.RequestedIPAddress()
		if requested == nil || requested.IsUnspecified() {
			requested = req.ClientIPAddr // renewal
		}
		if !d.cfg.Pool.Confirm(mac, requested) {
			// Wrong or stale address (e.g. from a previous network): try a
			// fresh allocation, NAK if even that fails.
			if ip := d.cfg.Pool.Allocate(mac, requested, req.HostName()); ip != nil && ip.Equal(requested) {
				return d.reply(req, dhcpv4.MessageTypeAck, ip)
			}
			return d.nak(req)
		}
		// Renew and acknowledge.
		ip := d.cfg.Pool.Allocate(mac, requested, req.HostName())
		return d.reply(req, dhcpv4.MessageTypeAck, ip)

	case dhcpv4.MessageTypeRelease, dhcpv4.MessageTypeDecline:
		d.cfg.Pool.Release(mac)
		return nil

	default:
		return nil
	}
}

func (d *DHCPServer) reply(req *dhcpv4.DHCPv4, mt dhcpv4.MessageType, ip net.IP) *dhcpv4.DHCPv4 {
	reply, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		return nil
	}
	reply.UpdateOption(dhcpv4.OptMessageType(mt))
	reply.YourIPAddr = ip
	reply.UpdateOption(dhcpv4.OptServerIdentifier(d.cfg.Boot.ServerIP))
	reply.UpdateOption(dhcpv4.OptSubnetMask(d.cfg.SubnetMask))
	reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(d.cfg.Pool.Duration()))
	if d.cfg.Gateway != nil {
		reply.UpdateOption(dhcpv4.OptRouter(d.cfg.Gateway))
	}
	if len(d.cfg.DNS) > 0 {
		reply.UpdateOption(dhcpv4.OptDNS(d.cfg.DNS...))
	}
	// PXE clients additionally get boot options; regular clients are
	// served plain addresses.
	if isPXERequest(req) {
		if err := d.cfg.Boot.applyPXEOptions(reply, req); err != nil {
			d.log.Debug("dhcp: no boot options for client", "mac", req.ClientHWAddr.String(), "reason", err)
		}
	}
	return reply
}

func (d *DHCPServer) nak(req *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	reply, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		return nil
	}
	reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeNak))
	reply.UpdateOption(dhcpv4.OptServerIdentifier(d.cfg.Boot.ServerIP))
	return reply
}

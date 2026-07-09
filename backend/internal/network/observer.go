package network

import (
	"log/slog"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// DHCPObserver passively records which DHCP servers are active on the
// segment (ADR-0016). Client DHCPREQUEST broadcasts carry the server
// identifier (option 54) of the server whose offer the client accepted
// — HELBOOT already receives them on UDP 67 in both modes, so no
// probing or extra sockets are needed. The API derives rogue-DHCP
// warnings from these sightings.
type DHCPObserver struct {
	log *slog.Logger

	mu        sync.Mutex
	selfIP    string
	sightings map[string]time.Time // server IP → last seen
}

// ServerSighting is one observed foreign DHCP server.
type ServerSighting struct {
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"lastSeen"`
}

// NewDHCPObserver creates an observer. The own server IP (set via
// SetSelfIP once known) is excluded from sightings.
func NewDHCPObserver(log *slog.Logger) *DHCPObserver {
	return &DHCPObserver{log: log, sightings: map[string]time.Time{}}
}

// SetSelfIP registers HELBOOT's own address so Mode B's own ACKs never
// count as a foreign server.
func (o *DHCPObserver) SetSelfIP(ip net.IP) {
	if o == nil || ip == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.selfIP = ip.String()
}

// Observe inspects one DHCP message received on port 67. Nil-safe so
// services can call it unconditionally.
func (o *DHCPObserver) Observe(req *dhcpv4.DHCPv4) {
	if o == nil || req == nil || req.OpCode != dhcpv4.OpcodeBootRequest {
		return
	}
	if req.MessageType() != dhcpv4.MessageTypeRequest {
		return
	}
	server := req.ServerIdentifier()
	if server == nil || server.IsUnspecified() {
		return
	}
	ip := server.String()

	o.mu.Lock()
	defer o.mu.Unlock()
	if ip == o.selfIP {
		return
	}
	_, known := o.sightings[ip]
	o.sightings[ip] = time.Now()
	if !known {
		o.log.Warn("dhcp: observed foreign DHCP server on this segment", "ip", ip)
	}
}

// Sightings returns the foreign servers seen within the window, oldest
// entries pruned, sorted by IP for stable output.
func (o *DHCPObserver) Sightings(window time.Duration) []ServerSighting {
	if o == nil {
		return nil
	}
	cutoff := time.Now().Add(-window)

	o.mu.Lock()
	defer o.mu.Unlock()
	out := []ServerSighting{}
	for ip, seen := range o.sightings {
		if seen.Before(cutoff) {
			delete(o.sightings, ip)
			continue
		}
		out = append(out, ServerSighting{IP: ip, LastSeen: seen})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })
	return out
}

package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

// LeaseStore is the persistence interface the pool uses; implemented by
// the store package. Keeping it an interface makes the pool testable
// without a database.
type LeaseStore interface {
	UpsertLease(mac, ip, hostname string, expiresAt time.Time) error
	DeleteLease(mac string) error
}

// LeasePool hands out IPv4 addresses from a contiguous range. All state
// is kept in memory and mirrored to the LeaseStore so leases survive
// restarts (loaded via Restore at startup).
type LeasePool struct {
	mu       sync.Mutex
	start    uint32
	end      uint32
	duration time.Duration
	store    LeaseStore
	byMAC    map[string]*poolLease
	byIP     map[uint32]*poolLease
	now      func() time.Time // injectable for tests
}

type poolLease struct {
	mac     string
	ip      uint32
	expires time.Time
}

// NewLeasePool creates a pool for [start, end] (inclusive). start and
// end must be valid IPv4 addresses with start <= end.
func NewLeasePool(start, end net.IP, duration time.Duration, store LeaseStore) (*LeasePool, error) {
	s, err := ipToUint(start)
	if err != nil {
		return nil, err
	}
	e, err := ipToUint(end)
	if err != nil {
		return nil, err
	}
	if s > e {
		return nil, fmt.Errorf("lease range start %s is after end %s", start, end)
	}
	return &LeasePool{
		start:    s,
		end:      e,
		duration: duration,
		store:    store,
		byMAC:    map[string]*poolLease{},
		byIP:     map[uint32]*poolLease{},
		now:      time.Now,
	}, nil
}

// Restore pre-loads a persisted lease (called at startup). Leases
// outside the current range are ignored — the range may have changed.
func (p *LeasePool) Restore(mac string, ip net.IP, expires time.Time) {
	n, err := ipToUint(ip)
	if err != nil || n < p.start || n > p.end {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	l := &poolLease{mac: mac, ip: n, expires: expires}
	p.byMAC[mac] = l
	p.byIP[n] = l
}

// Allocate returns the IP for mac, preferring its existing lease, then
// the requested IP, then the first free address. The lease is renewed
// and persisted. Returns nil when the pool is exhausted.
func (p *LeasePool) Allocate(mac string, requested net.IP, hostname string) net.IP {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()

	var ip uint32
	switch {
	case p.byMAC[mac] != nil:
		ip = p.byMAC[mac].ip
	case p.isFree(requested, now):
		ip, _ = ipToUint(requested)
	default:
		free, ok := p.firstFree(now)
		if !ok {
			return nil
		}
		ip = free
	}

	expires := now.Add(p.duration)
	// Evict whoever held the address if it expired (isFree/firstFree
	// only return expired or unused addresses).
	if old := p.byIP[ip]; old != nil && old.mac != mac {
		delete(p.byMAC, old.mac)
	}
	l := &poolLease{mac: mac, ip: ip, expires: expires}
	p.byMAC[mac] = l
	p.byIP[ip] = l
	if p.store != nil {
		if err := p.store.UpsertLease(mac, uintToIP(ip).String(), hostname, expires); err != nil {
			// Persistence failure must not break address assignment.
			_ = err
		}
	}
	return uintToIP(ip)
}

// Confirm reports whether mac currently holds ip (used for DHCPREQUEST
// validation: mismatches are NAKed).
func (p *LeasePool) Confirm(mac string, ip net.IP) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := p.byMAC[mac]
	if l == nil || p.now().After(l.expires) {
		return false
	}
	n, err := ipToUint(ip)
	return err == nil && n == l.ip
}

// Release frees the lease held by mac (DHCPRELEASE).
func (p *LeasePool) Release(mac string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if l := p.byMAC[mac]; l != nil {
		delete(p.byMAC, mac)
		delete(p.byIP, l.ip)
		if p.store != nil {
			_ = p.store.DeleteLease(mac)
		}
	}
}

// Duration returns the configured lease time.
func (p *LeasePool) Duration() time.Duration { return p.duration }

func (p *LeasePool) isFree(ip net.IP, now time.Time) bool {
	n, err := ipToUint(ip)
	if err != nil || n < p.start || n > p.end {
		return false
	}
	l := p.byIP[n]
	return l == nil || now.After(l.expires)
}

func (p *LeasePool) firstFree(now time.Time) (uint32, bool) {
	for n := p.start; n <= p.end; n++ {
		if l := p.byIP[n]; l == nil || now.After(l.expires) {
			return n, true
		}
	}
	return 0, false
}

func ipToUint(ip net.IP) (uint32, error) {
	v4 := ip.To4()
	if v4 == nil {
		return 0, fmt.Errorf("not an IPv4 address: %v", ip)
	}
	return binary.BigEndian.Uint32(v4), nil
}

func uintToIP(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

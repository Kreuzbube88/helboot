package network

import (
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func request(t *testing.T, mt dhcpv4.MessageType, serverID net.IP) *dhcpv4.DHCPv4 {
	t.Helper()
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	req, err := dhcpv4.New(dhcpv4.WithHwAddr(mac), dhcpv4.WithMessageType(mt))
	if err != nil {
		t.Fatal(err)
	}
	if serverID != nil {
		req.UpdateOption(dhcpv4.OptServerIdentifier(serverID))
	}
	return req
}

func TestObserverRecordsForeignServers(t *testing.T) {
	o := NewDHCPObserver(slog.New(slog.NewTextHandler(io.Discard, nil)))
	o.SetSelfIP(net.IPv4(192, 168, 1, 10))

	o.Observe(request(t, dhcpv4.MessageTypeRequest, net.IPv4(192, 168, 1, 1)))
	o.Observe(request(t, dhcpv4.MessageTypeRequest, net.IPv4(192, 168, 1, 1)))   // dedup
	o.Observe(request(t, dhcpv4.MessageTypeRequest, net.IPv4(192, 168, 1, 10)))  // self
	o.Observe(request(t, dhcpv4.MessageTypeDiscover, net.IPv4(192, 168, 1, 66))) // not a REQUEST
	o.Observe(request(t, dhcpv4.MessageTypeRequest, nil))                        // no option 54
	o.Observe(nil)                                                               // nil-safe

	got := o.Sightings(time.Minute)
	if len(got) != 1 || got[0].IP != "192.168.1.1" {
		t.Fatalf("sightings = %+v, want exactly 192.168.1.1", got)
	}

	// A second foreign server shows up as a distinct sighting.
	o.Observe(request(t, dhcpv4.MessageTypeRequest, net.IPv4(192, 168, 1, 66)))
	if got := o.Sightings(time.Minute); len(got) != 2 {
		t.Fatalf("sightings = %+v, want two servers", got)
	}
}

func TestObserverPrunesOldSightings(t *testing.T) {
	o := NewDHCPObserver(slog.New(slog.NewTextHandler(io.Discard, nil)))
	o.Observe(request(t, dhcpv4.MessageTypeRequest, net.IPv4(10, 0, 0, 1)))
	o.sightings["10.0.0.1"] = time.Now().Add(-time.Hour)

	if got := o.Sightings(30 * time.Minute); len(got) != 0 {
		t.Fatalf("sightings = %+v, want pruned", got)
	}
}

func TestNilObserverIsSafe(t *testing.T) {
	var o *DHCPObserver
	o.Observe(request(t, dhcpv4.MessageTypeRequest, net.IPv4(10, 0, 0, 1)))
	o.SetSelfIP(net.IPv4(10, 0, 0, 2))
	if got := o.Sightings(time.Minute); got != nil {
		t.Fatalf("nil observer sightings = %+v, want nil", got)
	}
}

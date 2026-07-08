package network

import (
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func pxeDiscover(t *testing.T, arch iana.Arch) *dhcpv4.DHCPv4 {
	t.Helper()
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	req, err := dhcpv4.NewDiscovery(mac)
	if err != nil {
		t.Fatal(err)
	}
	req.UpdateOption(dhcpv4.OptClassIdentifier("PXEClient:Arch:00007:UNDI:003016"))
	req.UpdateOption(dhcpv4.OptClientArch(arch))
	return req
}

func testBootConfig() BootConfig {
	return BootConfig{
		ServerIP:  net.IPv4(192, 168, 1, 10),
		ScriptURL: "http://192.168.1.10:8080/boot/ipxe",
	}
}

func TestBootFilenameByArch(t *testing.T) {
	cfg := testBootConfig()
	cases := []struct {
		arch iana.Arch
		want string
	}{
		{iana.INTEL_X86PC, "undionly.kpxe"},
		{iana.EFI_X86_64, "ipxe.efi"},
		{iana.EFI_BC, "ipxe.efi"},
		{iana.EFI_IA32, "ipxe-i386.efi"},
		{iana.EFI_ARM64, "ipxe-arm64.efi"},
	}
	for _, c := range cases {
		req := pxeDiscover(t, c.arch)
		if got := cfg.bootFilename(req); got != c.want {
			t.Errorf("arch %v: bootFilename = %q, want %q", c.arch, got, c.want)
		}
	}
}

func TestIPXEClientsChainToHTTP(t *testing.T) {
	cfg := testBootConfig()
	req := pxeDiscover(t, iana.EFI_X86_64)
	req.UpdateOption(dhcpv4.OptUserClass("iPXE"))
	if got := cfg.bootFilename(req); got != cfg.ScriptURL {
		t.Errorf("iPXE client got %q, want script URL", got)
	}
}

func TestProxyDHCPBuildReply(t *testing.T) {
	p := NewProxyDHCP(discardLogger(), testBootConfig())

	// PXE discover on port 67 → offer without an address.
	req := pxeDiscover(t, iana.EFI_X86_64)
	reply := p.buildReply(req, true)
	if reply == nil {
		t.Fatal("no reply for PXE discover")
	}
	if reply.MessageType() != dhcpv4.MessageTypeOffer {
		t.Errorf("message type = %v, want OFFER", reply.MessageType())
	}
	if !reply.YourIPAddr.Equal(net.IPv4zero) {
		t.Errorf("proxy offer must not assign an address, got %v", reply.YourIPAddr)
	}
	if reply.BootFileName != "ipxe.efi" {
		t.Errorf("bootfile = %q, want ipxe.efi", reply.BootFileName)
	}
	if reply.Options.Get(dhcpv4.OptionVendorSpecificInformation) == nil {
		t.Error("PXE vendor options (43) missing")
	}

	// Non-PXE discover must be ignored.
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:01")
	plain, _ := dhcpv4.NewDiscovery(mac)
	if p.buildReply(plain, true) != nil {
		t.Error("non-PXE discover must be ignored")
	}

	// Requests on port 67 are the real DHCP server's business.
	req67 := pxeDiscover(t, iana.EFI_X86_64)
	req67.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeRequest))
	if p.buildReply(req67, true) != nil {
		t.Error("REQUEST on port 67 must be ignored in proxy mode")
	}

	// Request on 4011 → ACK with boot file.
	req4011 := pxeDiscover(t, iana.INTEL_X86PC)
	req4011.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeRequest))
	ack := p.buildReply(req4011, false)
	if ack == nil || ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("expected ACK on port 4011, got %v", ack)
	}
	if ack.BootFileName != "undionly.kpxe" {
		t.Errorf("bootfile = %q, want undionly.kpxe", ack.BootFileName)
	}
}

func TestLeasePoolAllocateAndConfirm(t *testing.T) {
	pool, err := NewLeasePool(net.IPv4(10, 0, 0, 10), net.IPv4(10, 0, 0, 12), time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}

	ip1 := pool.Allocate("mac-1", nil, "host1")
	if ip1 == nil || !ip1.Equal(net.IPv4(10, 0, 0, 10).To4()) {
		t.Fatalf("first allocation = %v, want 10.0.0.10", ip1)
	}
	// Same MAC keeps its address.
	if again := pool.Allocate("mac-1", nil, "host1"); !again.Equal(ip1) {
		t.Errorf("re-allocation moved the lease: %v", again)
	}
	if !pool.Confirm("mac-1", ip1) {
		t.Error("Confirm rejected the holder")
	}
	if pool.Confirm("mac-2", ip1) {
		t.Error("Confirm accepted a non-holder")
	}

	// Requested address is honored when free.
	ip2 := pool.Allocate("mac-2", net.IPv4(10, 0, 0, 12), "host2")
	if !ip2.Equal(net.IPv4(10, 0, 0, 12).To4()) {
		t.Errorf("requested address not honored: %v", ip2)
	}

	// Pool exhaustion.
	pool.Allocate("mac-3", nil, "")
	if ip := pool.Allocate("mac-4", nil, ""); ip != nil {
		t.Errorf("exhausted pool still allocated %v", ip)
	}

	// Release frees the address for others.
	pool.Release("mac-3")
	if ip := pool.Allocate("mac-4", nil, ""); ip == nil {
		t.Error("released address was not reusable")
	}
}

func TestLeasePoolExpiry(t *testing.T) {
	pool, err := NewLeasePool(net.IPv4(10, 0, 0, 10), net.IPv4(10, 0, 0, 10), time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	pool.now = func() time.Time { return now }

	pool.Allocate("mac-1", nil, "")
	if ip := pool.Allocate("mac-2", nil, ""); ip != nil {
		t.Fatal("single-address pool double-allocated")
	}

	// After expiry the address is reclaimed by another client.
	now = now.Add(2 * time.Hour)
	if ip := pool.Allocate("mac-2", nil, ""); ip == nil {
		t.Error("expired lease was not reclaimed")
	}
	if pool.Confirm("mac-1", net.IPv4(10, 0, 0, 10)) {
		t.Error("evicted client still confirmed")
	}
}

func TestDHCPServerStateMachine(t *testing.T) {
	pool, _ := NewLeasePool(net.IPv4(10, 0, 0, 100), net.IPv4(10, 0, 0, 110), time.Hour, nil)
	srv := NewDHCPServer(discardLogger(), DHCPConfig{
		Boot:       testBootConfig(),
		SubnetMask: net.CIDRMask(24, 32),
		Gateway:    net.IPv4(10, 0, 0, 1),
		DNS:        []net.IP{net.IPv4(10, 0, 0, 1)},
		Pool:       pool,
	})

	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:02")
	discover, _ := dhcpv4.NewDiscovery(mac)
	offer := srv.buildReply(discover)
	if offer == nil || offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("expected OFFER, got %v", offer)
	}
	if offer.YourIPAddr.IsUnspecified() {
		t.Fatal("offer carries no address")
	}

	request, _ := dhcpv4.NewRequestFromOffer(offer)
	ack := srv.buildReply(request)
	if ack == nil || ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("expected ACK, got %v", ack)
	}
	if !ack.YourIPAddr.Equal(offer.YourIPAddr) {
		t.Errorf("ACK address %v differs from offer %v", ack.YourIPAddr, offer.YourIPAddr)
	}

	// A REQUEST for a foreign address that cannot be honored → NAK.
	otherMAC, _ := net.ParseMAC("aa:bb:cc:dd:ee:03")
	bad, _ := dhcpv4.New(dhcpv4.WithHwAddr(otherMAC))
	bad.OpCode = dhcpv4.OpcodeBootRequest
	bad.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeRequest))
	bad.UpdateOption(dhcpv4.OptRequestedIPAddress(offer.YourIPAddr)) // taken by mac-02
	nak := srv.buildReply(bad)
	if nak == nil || nak.MessageType() != dhcpv4.MessageTypeNak {
		t.Fatalf("expected NAK for stolen address, got %v", nak)
	}

	// PXE clients get boot options in the offer.
	pxe := pxeDiscover(t, iana.EFI_X86_64)
	pxeOffer := srv.buildReply(pxe)
	if pxeOffer == nil || pxeOffer.BootFileName != "ipxe.efi" {
		t.Fatalf("PXE offer missing boot file: %v", pxeOffer)
	}
}

// Package network implements HELBOOT's boot-network services (ADR-0006):
// ProxyDHCP (Mode A), an authoritative DHCP server (Mode B) and TFTP.
// All post-firmware traffic is HTTP, handled by the boot package.
package network

import (
	"fmt"
	"net"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

// BootConfig is what both DHCP modes need to point clients at HELBOOT.
type BootConfig struct {
	// ServerIP is the IPv4 address clients use to reach HELBOOT
	// (TFTP next-server and HTTP host).
	ServerIP net.IP
	// ScriptURL is the iPXE boot script endpoint, e.g.
	// "http://192.168.1.10:8080/boot/ipxe".
	ScriptURL string
}

// bootFilename returns the TFTP boot file for a client, derived from the
// client system architecture (RFC 4578, option 93). iPXE clients (user
// class "iPXE") skip TFTP entirely and chain straight to the HTTP script.
// The empty string means "not a PXE client we can serve".
func (c BootConfig) bootFilename(req *dhcpv4.DHCPv4) string {
	if isIPXE(req) {
		return c.ScriptURL
	}
	archs := req.ClientArch()
	if len(archs) == 0 {
		// Old ROMs that omit option 93 are practically always BIOS.
		return "undionly.kpxe"
	}
	switch archs[0] {
	case iana.INTEL_X86PC:
		return "undionly.kpxe"
	case iana.EFI_IA32:
		return "ipxe-i386.efi"
	case iana.EFI_X86_64, iana.EFI_BC:
		return "ipxe.efi"
	case iana.EFI_ARM64:
		return "ipxe-arm64.efi"
	default:
		return ""
	}
}

// isPXERequest reports whether the message is a PXE boot request we
// should answer at all (vendor class "PXEClient"/"HTTPClient" or iPXE).
func isPXERequest(req *dhcpv4.DHCPv4) bool {
	class := req.ClassIdentifier()
	return strings.HasPrefix(class, "PXEClient") ||
		strings.HasPrefix(class, "HTTPClient") ||
		isIPXE(req)
}

func isIPXE(req *dhcpv4.DHCPv4) bool {
	for _, uc := range req.UserClass() {
		if uc == "iPXE" {
			return true
		}
	}
	return false
}

// FirmwareOf classifies the client firmware for host discovery.
func FirmwareOf(req *dhcpv4.DHCPv4) string {
	archs := req.ClientArch()
	if len(archs) == 0 || archs[0] == iana.INTEL_X86PC {
		return "bios"
	}
	return "uefi"
}

// applyPXEOptions fills the PXE-relevant fields of a reply: next-server,
// boot filename, vendor class and — for pure ProxyDHCP offers — the PXE
// vendor options (option 43) telling the ROM to use the boot file as-is
// instead of running boot server discovery.
func (c BootConfig) applyPXEOptions(reply *dhcpv4.DHCPv4, req *dhcpv4.DHCPv4) error {
	filename := c.bootFilename(req)
	if filename == "" {
		return fmt.Errorf("unsupported client architecture %v", req.ClientArch())
	}
	reply.ServerIPAddr = c.ServerIP
	reply.BootFileName = filename
	reply.UpdateOption(dhcpv4.OptTFTPServerName(c.ServerIP.String()))
	reply.UpdateOption(dhcpv4.OptBootFileName(filename))
	reply.UpdateOption(dhcpv4.OptClassIdentifier("PXEClient"))
	// PXE vendor options: PXE_DISCOVERY_CONTROL (tag 6) = 8 → "use the
	// boot file name from this packet, do not discover boot servers".
	reply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation,
		[]byte{6, 1, 8, 255}))
	return nil
}

// Package netutil contains small network-related helpers shared across
// layers.
package netutil

import (
	"fmt"
	"net"
	"strings"
)

// NormalizeMAC parses a MAC address in any common notation
// (aa:bb:cc:dd:ee:ff, AA-BB-..., aabb.ccdd.eeff) and returns the
// canonical lowercase colon form used as the host key everywhere.
func NormalizeMAC(s string) (string, error) {
	hw, err := net.ParseMAC(strings.TrimSpace(s))
	if err != nil {
		return "", fmt.Errorf("invalid MAC address %q", s)
	}
	if len(hw) != 6 {
		return "", fmt.Errorf("unsupported MAC address length %d (want 48-bit)", len(hw)*8)
	}
	return strings.ToLower(hw.String()), nil
}

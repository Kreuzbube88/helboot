package netutil

import "testing"

func TestNormalizeMAC(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff", true},
		{"aa-bb-cc-dd-ee-ff", "aa:bb:cc:dd:ee:ff", true},
		{"aabb.ccdd.eeff", "aa:bb:cc:dd:ee:ff", true},
		{" 00:11:22:33:44:55 ", "00:11:22:33:44:55", true},
		{"not-a-mac", "", false},
		{"", "", false},
		{"01:23:45:67:89:ab:cd:ef", "", false}, // EUI-64 is not a host MAC here
	}
	for _, c := range cases {
		got, err := NormalizeMAC(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("NormalizeMAC(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("NormalizeMAC(%q) succeeded, want error", c.in)
		}
	}
}

package api

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/network"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// networkConfig is the API shape of the boot-network configuration
// (ADR-0006). Changes take effect after a restart; the response flags
// this so the UI can tell the user.
type networkConfig struct {
	Mode string `json:"mode"` // proxy_dhcp | dhcp
	// ServerIP is the address PXE clients use to reach HELBOOT. Empty
	// means auto-detect at startup.
	ServerIP string `json:"serverIp"`
	DHCP     struct {
		RangeStart   string `json:"rangeStart"`
		RangeEnd     string `json:"rangeEnd"`
		SubnetMask   string `json:"subnetMask"`
		Gateway      string `json:"gateway"`
		DNS          string `json:"dns"`
		LeaseMinutes int    `json:"leaseMinutes"`
	} `json:"dhcp"`
}

// Settings keys for the network configuration.
const (
	settingServerIP     = "network.server_ip"
	settingDHCPStart    = "network.dhcp.range_start"
	settingDHCPEnd      = "network.dhcp.range_end"
	settingDHCPMask     = "network.dhcp.subnet_mask"
	settingDHCPGateway  = "network.dhcp.gateway"
	settingDHCPDNS      = "network.dhcp.dns"
	settingDHCPLeaseMin = "network.dhcp.lease_minutes"
)

func (s *Server) handleGetNetworkConfig(w http.ResponseWriter, _ *http.Request) {
	cfg, err := s.readNetworkConfig()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handlePutNetworkConfig(w http.ResponseWriter, r *http.Request) {
	var cfg networkConfig
	if !decodeJSON(w, r, &cfg) {
		return
	}
	if cfg.Mode != "proxy_dhcp" && cfg.Mode != "dhcp" {
		writeError(w, http.StatusBadRequest, "network.invalid_mode", "mode must be one of: proxy_dhcp, dhcp")
		return
	}
	for _, ip := range []string{cfg.ServerIP, cfg.DHCP.Gateway} {
		if ip != "" && net.ParseIP(ip) == nil {
			writeError(w, http.StatusBadRequest, "network.invalid_ip", "invalid IP address: "+ip)
			return
		}
	}
	if cfg.Mode == "dhcp" {
		start, end := net.ParseIP(cfg.DHCP.RangeStart), net.ParseIP(cfg.DHCP.RangeEnd)
		if start == nil || end == nil {
			writeError(w, http.StatusBadRequest, "network.invalid_range", "DHCP mode requires a valid address range")
			return
		}
		if cfg.DHCP.SubnetMask != "" && net.ParseIP(cfg.DHCP.SubnetMask) == nil {
			writeError(w, http.StatusBadRequest, "network.invalid_ip", "invalid subnet mask")
			return
		}
		if cfg.DHCP.LeaseMinutes < 0 {
			writeError(w, http.StatusBadRequest, "network.invalid_lease", "lease minutes must be positive")
			return
		}
	}

	values := map[string]string{
		store.SettingNetworkMode: cfg.Mode,
		settingServerIP:          cfg.ServerIP,
		settingDHCPStart:         cfg.DHCP.RangeStart,
		settingDHCPEnd:           cfg.DHCP.RangeEnd,
		settingDHCPMask:          cfg.DHCP.SubnetMask,
		settingDHCPGateway:       cfg.DHCP.Gateway,
		settingDHCPDNS:           cfg.DHCP.DNS,
		settingDHCPLeaseMin:      strconv.Itoa(cfg.DHCP.LeaseMinutes),
	}
	for key, value := range values {
		if err := s.store.SetSetting(key, value); err != nil {
			s.internalError(w, err)
			return
		}
	}
	s.audit(r, "network.update_config", "settings", 0)
	writeJSON(w, http.StatusOK, map[string]any{
		"saved":           true,
		"restartRequired": true,
	})
}

// observationWindow bounds how long a DHCP-server sighting stays
// relevant for warnings (ADR-0016).
const observationWindow = 30 * time.Minute

// networkWarning is a machine-readable warning; code doubles as the
// i18n key for the UI.
type networkWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// handleNetworkStatus reports observed DHCP servers and the warnings
// derived from them for the active mode (ADR-0016): in Mode A more than
// one foreign server is suspicious, in Mode B any foreign server is.
func (s *Server) handleNetworkStatus(w http.ResponseWriter, _ *http.Request) {
	mode, err := s.store.GetSetting(store.SettingNetworkMode)
	if err != nil && err != store.ErrNotFound {
		s.internalError(w, err)
		return
	}

	sightings := []network.ServerSighting{}
	if s.dhcpObserver != nil {
		sightings = s.dhcpObserver.Sightings(observationWindow)
	}
	warnings := []networkWarning{}
	switch mode {
	case "dhcp":
		if len(sightings) >= 1 {
			warnings = append(warnings, networkWarning{
				Code:    "network.rogue_dhcp_server",
				Message: "another DHCP server is answering on this network although HELBOOT is the DHCP server",
			})
		}
	default: // Mode A: exactly one foreign server (the router) is expected
		if len(sightings) >= 2 {
			warnings = append(warnings, networkWarning{
				Code:    "network.multiple_dhcp_servers",
				Message: "more than one DHCP server is answering on this network; PXE boot may be unreliable",
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":        mode,
		"dhcpServers": sightings,
		"warnings":    warnings,
	})
}

func (s *Server) readNetworkConfig() (*networkConfig, error) {
	get := func(key string) (string, error) {
		v, err := s.store.GetSetting(key)
		if err == store.ErrNotFound {
			return "", nil
		}
		return v, err
	}
	var cfg networkConfig
	var err error
	if cfg.Mode, err = get(store.SettingNetworkMode); err != nil {
		return nil, err
	}
	if cfg.ServerIP, err = get(settingServerIP); err != nil {
		return nil, err
	}
	if cfg.DHCP.RangeStart, err = get(settingDHCPStart); err != nil {
		return nil, err
	}
	if cfg.DHCP.RangeEnd, err = get(settingDHCPEnd); err != nil {
		return nil, err
	}
	if cfg.DHCP.SubnetMask, err = get(settingDHCPMask); err != nil {
		return nil, err
	}
	if cfg.DHCP.Gateway, err = get(settingDHCPGateway); err != nil {
		return nil, err
	}
	if cfg.DHCP.DNS, err = get(settingDHCPDNS); err != nil {
		return nil, err
	}
	lease, err := get(settingDHCPLeaseMin)
	if err != nil {
		return nil, err
	}
	cfg.DHCP.LeaseMinutes, _ = strconv.Atoi(lease)
	return &cfg, nil
}

// Package model defines HELBOOT's core domain types. It has no
// dependencies on storage, HTTP or network code so every layer can share
// these types freely.
package model

import "time"

// Role is the authorization level of a user. Roles are strictly ordered:
// viewer < operator < admin.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Valid reports whether r is one of the defined roles.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	}
	return false
}

// AtLeast reports whether r grants at least the privileges of min.
func (r Role) AtLeast(min Role) bool {
	return roleRank(r) >= roleRank(min)
}

func roleRank(r Role) int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleOperator:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

// User is a local HELBOOT account.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	Locale       string    `json:"locale"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Session is a server-side login session (ADR-0007).
type Session struct {
	Token     string
	UserID    int64
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Host is a machine managed by MAC address.
type Host struct {
	ID        int64    `json:"id"`
	MAC       string   `json:"mac"`
	Hostname  string   `json:"hostname"`
	Vendor    string   `json:"vendor"`
	Model     string   `json:"model"`
	Serial    string   `json:"serial"`
	AssetID   string   `json:"assetId"`
	Tags      []string `json:"tags"`
	Firmware  string   `json:"firmware"` // "bios" or "uefi"
	Arch      string   `json:"arch"`     // e.g. "x86_64", "arm64"
	ProfileID *int64   `json:"profileId"`
	// ProfileVersion pins the profile version this host installs
	// (ADR-0013); 0 while no profile is assigned. Never an implicit
	// "latest": assigning a profile pins its then-current version.
	ProfileVersion int        `json:"profileVersion"`
	Status         HostStatus `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// HostStatus describes where a host is in its lifecycle.
type HostStatus string

const (
	// HostDiscovered marks hosts that appeared via network discovery and
	// have not been adopted by the user yet.
	HostDiscovered HostStatus = "discovered"
	HostReady      HostStatus = "ready"
	HostInstalling HostStatus = "installing"
	HostError      HostStatus = "error"
)

// Profile is an installation recipe for one provider. Its settings live
// in immutable ProfileVersion snapshots.
type Profile struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`
	ISOID          *int64    `json:"isoId"`
	CurrentVersion int       `json:"currentVersion"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ProfileVersion is one immutable snapshot of a profile's configuration.
// Config is a provider-shaped JSON document (language, users, network,
// partitioning, packages, scripts, ...) validated by the provider.
type ProfileVersion struct {
	ID        int64     `json:"id"`
	ProfileID int64     `json:"profileId"`
	Version   int       `json:"version"`
	Config    string    `json:"config"`
	CreatedAt time.Time `json:"createdAt"`
}

// InstallationStatus tracks the installation queue states (§16).
type InstallationStatus string

const (
	InstallDiscovered InstallationStatus = "discovered"
	InstallWaiting    InstallationStatus = "waiting"
	InstallRunning    InstallationStatus = "installing"
	InstallSuccess    InstallationStatus = "success"
	InstallError      InstallationStatus = "error"
)

// Installation links a host to a specific profile version, preserving
// accurate history even when the profile changes later. Token scopes the
// unauthenticated boot-time endpoints (answer file, status report) to
// this one installation and is never exposed through the JSON API.
type Installation struct {
	ID               int64              `json:"id"`
	HostID           int64              `json:"hostId"`
	ProfileVersionID int64              `json:"profileVersionId"`
	Status           InstallationStatus `json:"status"`
	StartedAt        *time.Time         `json:"startedAt"`
	FinishedAt       *time.Time         `json:"finishedAt"`
	Log              string             `json:"log"`
	Token            string             `json:"-"`
	CreatedAt        time.Time          `json:"createdAt"`
}

// ISOImage is an uploaded, never-modified original installation medium
// together with its analysis result.
type ISOImage struct {
	ID            int64     `json:"id"`
	Filename      string    `json:"filename"`
	Provider      string    `json:"provider"`
	OSName        string    `json:"osName"`
	Version       string    `json:"version"`
	Arch          string    `json:"arch"`
	Bootloader    string    `json:"bootloader"`
	InstallMethod string    `json:"installMethod"`
	SizeBytes     int64     `json:"sizeBytes"`
	SHA256        string    `json:"sha256"`
	Status        string    `json:"status"` // uploaded | analyzing | ready | unsupported
	CreatedAt     time.Time `json:"createdAt"`
}

// Package plugin defines HELBOOT's extension-point interfaces
// (ADR-0008). Version 1 registers implementations at compile time; the
// interfaces live here, isolated from their implementations, so a future
// dynamic plugin runtime can adopt them without breaking anything.
package plugin

import "context"

// BootMethod delivers boot artifacts to a machine. PXE, HTTP boot and
// USB image generation are the first implementations; VM provisioning,
// Redfish and IPMI are future ones.
type BootMethod interface {
	// Name is the stable identifier ("pxe", "http_boot", "usb_boot", ...).
	Name() string
	// Run starts the method's network services (if any) and blocks until
	// ctx is cancelled. Methods without long-running services return nil
	// immediately.
	Run(ctx context.Context) error
}

// IdentityProvider authenticates users (ADR-0007). The local provider is
// the baseline; OIDC is the second implementation.
type IdentityProvider interface {
	Name() string
	// Authenticate verifies credentials and returns the internal user ID,
	// or an error for invalid credentials.
	Authenticate(ctx context.Context, username, password string) (userID int64, err error)
}

// AnswerFileRenderer generates an unattended-install answer file from a
// provider template and a profile configuration document.
type AnswerFileRenderer interface {
	// Format is the answer-file format this renderer produces
	// ("autounattend.xml", "preseed", "kickstart", ...).
	Format() string
	// Render produces the answer file for one host installation.
	Render(ctx context.Context, template []byte, profileConfig []byte) ([]byte, error)
}

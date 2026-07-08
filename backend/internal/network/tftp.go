package network

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pin/tftp/v3"
)

// TFTPServer serves read-only boot files (iPXE binaries) from a root
// directory. TFTP is only the minimal firmware bootstrap; everything
// after iPXE runs over HTTP (ADR-0006).
type TFTPServer struct {
	log  *slog.Logger
	root string
	addr string
}

// NewTFTPServer creates a TFTP service rooted at root (typically
// "<assets>/tftp").
func NewTFTPServer(log *slog.Logger, root string) *TFTPServer {
	return &TFTPServer{log: log, root: root, addr: ":69"}
}

// Name implements service.Service.
func (t *TFTPServer) Name() string { return "tftp" }

// Run starts the listener and blocks until ctx is cancelled.
func (t *TFTPServer) Run(ctx context.Context) error {
	srv := tftp.NewServer(t.readHandler, nil) // nil write handler: read-only
	srv.SetTimeout(5 * time.Second)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(t.addr) }()
	select {
	case <-ctx.Done():
		srv.Shutdown()
		return ctx.Err()
	case err := <-errCh:
		return fmt.Errorf("tftp: %w", err)
	}
}

func (t *TFTPServer) readHandler(filename string, rf io.ReaderFrom) error {
	path, err := t.securePath(filename)
	if err != nil {
		t.log.Warn("tftp: rejected request", "file", filename, "error", err)
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		t.log.Warn("tftp: file not found", "file", filename,
			"hint", "place iPXE binaries in the tftp assets directory; see docs/DOCKER.md")
		return err
	}
	defer f.Close()
	n, err := rf.ReadFrom(f)
	if err != nil {
		return err
	}
	t.log.Info("tftp: served file", "file", filename, "bytes", n)
	return nil
}

// securePath resolves filename inside the root directory, rejecting any
// path traversal attempt (§29 input validation).
func (t *TFTPServer) securePath(filename string) (string, error) {
	clean := filepath.Clean("/" + strings.ReplaceAll(filename, "\\", "/"))
	path := filepath.Join(t.root, clean)
	if !strings.HasPrefix(path, filepath.Clean(t.root)+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes tftp root")
	}
	return path, nil
}

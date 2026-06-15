package registry

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/course-studio/skillex/internal/config"
)

// ErrAutoRefreshDisabled is returned by EnsureIndex when the index needs to be
// built but auto-build is disabled via SKILLEX_NO_AUTO_REFRESH. Callers map it to
// their own "registry not found — run 'skillex refresh' first" message.
var ErrAutoRefreshDisabled = errors.New("registry missing and auto-refresh disabled (SKILLEX_NO_AUTO_REFRESH)")

// autoRefreshEnvVar is the opt-out switch: set it to a truthy value to disable
// on-demand index building for query/mcp (restoring the legacy "run skillex
// refresh first" behavior).
const autoRefreshEnvVar = "SKILLEX_NO_AUTO_REFRESH"

// EnsureIndex opens the skillex registry at <root>/.skillex/index.db, building it
// on demand when it is missing. Reused by the runtime commands (query, mcp) so
// skillex works in a fresh checkout or git worktree without a manual
// `skillex refresh` first. A healthy existing index is opened as-is — EnsureIndex
// does not rebuild a stale index (`skillex refresh --check` remains the staleness
// gate). Build progress is written to progress, which callers MUST set to
// os.Stderr (never os.Stdout) so the mcp JSON-RPC stream stays byte-clean.
func EnsureIndex(root string, progress io.Writer) (*Registry, error) {
	if progress == nil {
		progress = io.Discard
	}
	dbPath := filepath.Join(root, ".skillex", "index.db")

	if _, err := os.Stat(dbPath); err == nil {
		// Index exists — open and use it as-is, without rebuilding.
		if reg, openErr := Open(dbPath); openErr == nil {
			return reg, nil
		}
		// Exists but unopenable (corrupt): rebuild it, unless opted out.
		if autoRefreshDisabled() {
			return nil, ErrAutoRefreshDisabled
		}
		fmt.Fprintln(progress, "skillex: index unreadable — rebuilding "+filepath.Join(".skillex", "index.db"))
		removeDBFiles(dbPath)
	} else if autoRefreshDisabled() {
		// Missing index and auto-build disabled: preserve legacy behavior.
		return nil, ErrAutoRefreshDisabled
	} else {
		fmt.Fprintln(progress, "skillex: index not found — building "+filepath.Join(".skillex", "index.db")+" (set SKILLEX_NO_AUTO_REFRESH=1 to disable)")
	}

	return buildIndex(root, dbPath, progress)
}

// autoRefreshDisabled reports whether on-demand index building is turned off via
// the SKILLEX_NO_AUTO_REFRESH environment variable.
func autoRefreshDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(autoRefreshEnvVar))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// buildIndex loads config and builds the index for root, returning an open
// registry. It reuses the transactional Refresh builder, building into a uniquely
// named temp database and then atomically renaming it into place at dbPath. The
// build-then-swap keeps concurrent cold starts safe: another process sees either
// no index yet (and builds its own) or the fully-built index — never a half-built
// database. Build progress goes to progress (stderr), never stdout.
func buildIndex(root, dbPath string, progress io.Writer) (*Registry, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating registry directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "index-*.db.tmp")
	if err != nil {
		return nil, fmt.Errorf("creating temp index: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close() //nolint:errcheck // reopened via Open below; CreateTemp just reserves a unique name
	installed := false
	defer func() {
		if !installed {
			removeDBFiles(tmpPath)
		}
	}()

	reg, err := Open(tmpPath)
	if err != nil {
		return nil, err
	}
	if _, err := Refresh(reg, cfg, RefreshOptions{Root: root, DevMode: true}); err != nil {
		reg.Close() //nolint:errcheck // returning the more relevant Refresh error
		return nil, err
	}
	// Close before renaming so SQLite checkpoints the WAL into the main file and
	// releases its sidecars, leaving tmpPath a self-contained database.
	if err := reg.Close(); err != nil {
		return nil, fmt.Errorf("finalizing built index: %w", err)
	}

	// Atomically swap the freshly built index into place. os.Rename replaces an
	// existing file atomically on POSIX; if it fails (e.g. a stale file is in the
	// way), clear the destination and retry.
	if err := os.Rename(tmpPath, dbPath); err != nil {
		removeDBFiles(dbPath)
		if err := os.Rename(tmpPath, dbPath); err != nil {
			return nil, fmt.Errorf("installing built index: %w", err)
		}
	}
	installed = true

	return Open(dbPath)
}

// removeDBFiles deletes the SQLite database at dbPath and its WAL/SHM/journal
// sidecars, ignoring missing-file errors.
func removeDBFiles(dbPath string) {
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm", dbPath + "-journal"} {
		_ = os.Remove(p)
	}
}

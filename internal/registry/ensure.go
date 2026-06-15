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

// ErrAutoRefreshDisabled is returned by EnsureIndex when the index is missing but
// auto-build is disabled via SKILLEX_NO_AUTO_REFRESH. Callers map it to their own
// "registry not found — run 'skillex refresh' first" message.
var ErrAutoRefreshDisabled = errors.New("registry missing and auto-refresh disabled (SKILLEX_NO_AUTO_REFRESH)")

// autoRefreshEnvVar is the opt-out switch: set it to a truthy value to disable
// on-demand index building for query/mcp (restoring the legacy "run skillex
// refresh first" behavior).
const autoRefreshEnvVar = "SKILLEX_NO_AUTO_REFRESH"

// osRename is a seam so tests can simulate a failing rename (e.g. the Windows
// sharing violation when a peer process holds the destination index open).
var osRename = os.Rename

// EnsureIndex opens the skillex registry at <root>/.skillex/index.db, building it
// on demand when it is missing or unreadable. Reused by the runtime commands
// (query, mcp) so skillex works in a fresh checkout or git worktree without a
// manual `skillex refresh` first. A healthy existing index is opened as-is —
// EnsureIndex does not rebuild a stale index (`skillex refresh --check` remains the
// staleness gate). Build progress is written to progress, which callers MUST set to
// os.Stderr (never os.Stdout) so the mcp JSON-RPC stream stays byte-clean.
//
// Auto-build is skipped when SKILLEX_NO_AUTO_REFRESH is set to a truthy value: a
// missing index then yields ErrAutoRefreshDisabled, and an existing-but-unreadable
// index yields the underlying open error.
func EnsureIndex(root string, progress io.Writer) (*Registry, error) {
	if progress == nil {
		progress = io.Discard
	}
	dbPath := filepath.Join(root, ".skillex", "index.db")

	if _, statErr := os.Stat(dbPath); statErr == nil {
		// Index exists — open and use it as-is, without rebuilding.
		reg, openErr := Open(dbPath)
		if openErr == nil {
			return reg, nil
		}
		// Exists but unreadable (corrupt).
		if autoRefreshDisabled() {
			return nil, fmt.Errorf("opening registry: %w — run 'skillex refresh' first", openErr)
		}
		removeDBFiles(dbPath)
		return buildIndex(root, dbPath, progress, "rebuilding unreadable index")
	}

	// Index missing.
	if autoRefreshDisabled() {
		return nil, ErrAutoRefreshDisabled
	}
	return buildIndex(root, dbPath, progress, "building index")
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
// named temp database and then installing it at dbPath (see installIndex). The
// build-then-install keeps concurrent cold starts safe: another process sees
// either no index yet (and builds its own) or the fully-built index — never a
// half-built database. reason ("building index" / "rebuilding unreadable index")
// is announced to progress only after config loads, so a config-less directory
// never sees a build line before the "run skillex init" error. All progress goes
// to progress (stderr), never stdout.
func buildIndex(root, dbPath string, progress io.Writer, reason string) (*Registry, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(progress, "skillex: %s (%s)\n", reason, filepath.Join(".skillex", "index.db"))

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
	res, err := Refresh(reg, cfg, RefreshOptions{Root: root, DevMode: true})
	if err != nil {
		reg.Close() //nolint:errcheck // returning the more relevant Refresh error
		return nil, err
	}
	// Surface best-effort scanner/parse warnings (skipped/unparseable skills) to
	// stderr, matching `skillex refresh` — they are not fatal but aid diagnosis.
	if len(res.Errors) > 0 {
		fmt.Fprint(progress, FormatErrors(res.Errors))
	}
	// Close before installing so SQLite checkpoints the WAL into the main file and
	// releases its sidecars, leaving tmpPath a self-contained database.
	if err := reg.Close(); err != nil {
		return nil, fmt.Errorf("finalizing built index: %w", err)
	}

	final, err := installIndex(tmpPath, dbPath)
	if err != nil {
		return nil, err
	}
	installed = true
	return final, nil
}

// installIndex moves the freshly built tmpPath into place at dbPath and returns an
// open registry. On POSIX os.Rename atomically replaces any existing index, so the
// rename normally succeeds. If it fails — most notably on Windows, where it cannot
// replace an index.db that another process currently holds open — and a healthy
// index is already present, that peer-built index is used instead (the temp is
// discarded). Only when no usable index is present does it clear a stale/leftover
// file and retry once.
func installIndex(tmpPath, dbPath string) (*Registry, error) {
	if err := osRename(tmpPath, dbPath); err == nil {
		return Open(dbPath)
	}
	// Rename failed. A concurrent process may have installed the index first (its
	// open handle blocks our rename on Windows). If a healthy index is now present,
	// use it and drop our temp.
	if reg, openErr := Open(dbPath); openErr == nil {
		removeDBFiles(tmpPath)
		return reg, nil
	}
	// Otherwise a stale/leftover file is blocking the rename — clear it and retry.
	removeDBFiles(dbPath)
	if err := osRename(tmpPath, dbPath); err != nil {
		return nil, fmt.Errorf("installing built index: %w", err)
	}
	return Open(dbPath)
}

// removeDBFiles deletes the SQLite database at dbPath and its WAL/SHM/journal
// sidecars, ignoring missing-file errors.
func removeDBFiles(dbPath string) {
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm", dbPath + "-journal"} {
		_ = os.Remove(p)
	}
}

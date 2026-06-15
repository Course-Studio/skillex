package registry

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// Show the path relative to root (".skillex/index.db") derived from dbPath.
	fmt.Fprintf(progress, "skillex: %s (%s)\n", reason, filepath.Join(filepath.Base(filepath.Dir(dbPath)), filepath.Base(dbPath)))

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating registry directory: %w", err)
	}
	sweepStaleTempIndexes(dir)

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
		// The renamed-in database is self-contained. Drop any stale sidecars left
		// beside the destination (e.g. an orphaned -wal from a killed session) so
		// SQLite cannot replay a foreign WAL onto the freshly installed index.
		dropStaleSidecars(dbPath)
		return Open(dbPath)
	}
	// Rename failed. A concurrent process may have installed the index first (its
	// open handle blocks our rename on Windows). Adopt an existing peer index ONLY
	// when dbPath already exists on disk — Open() auto-creates an empty stub on a
	// missing path, which would otherwise mask a genuine install failure as a
	// healthy (but empty) index. File existence, not skill count, is the right
	// discriminator: a legitimately empty repo yields a valid 0-skill peer.
	if _, statErr := os.Stat(dbPath); statErr == nil {
		if reg, openErr := Open(dbPath); openErr == nil {
			removeDBFiles(tmpPath)
			return reg, nil
		}
	}
	// No usable index present — clear the destination and retry once.
	removeDBFiles(dbPath)
	if err := osRename(tmpPath, dbPath); err != nil {
		return nil, fmt.Errorf("installing built index: %w", err)
	}
	return Open(dbPath)
}

// removeDBFiles deletes the SQLite database at dbPath and its WAL/SHM/journal
// sidecars, ignoring missing-file errors.
func removeDBFiles(dbPath string) {
	_ = os.Remove(dbPath)
	dropStaleSidecars(dbPath)
}

// dropStaleSidecars removes the WAL/SHM/journal sidecars beside dbPath, ignoring
// missing-file errors.
func dropStaleSidecars(dbPath string) {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		_ = os.Remove(dbPath + suffix)
	}
}

// sweepStaleTempIndexes best-effort removes orphaned build temp databases left by
// an interrupted build (SIGKILL/power-loss between CreateTemp and install). Only
// clearly-stale temps (untouched for over an hour) are reaped, so a concurrent
// peer's in-flight build temp is never removed.
func sweepStaleTempIndexes(dir string) {
	matches, err := filepath.Glob(filepath.Join(dir, "index-*.db.tmp*"))
	if err != nil {
		return
	}
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil && time.Since(fi.ModTime()) > time.Hour {
			_ = os.Remove(m)
		}
	}
}

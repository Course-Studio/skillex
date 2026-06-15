# Changelog

All notable changes to this project should be documented in this file.

The format is based on Keep a Changelog and the project uses Semantic Versioning.

## [Unreleased]

## [0.10.0]

- **Added:** `skillex query` and `skillex mcp` now auto-build a missing or unreadable `.skillex/index.db` on demand — reusing the existing transactional `refresh` builder — so skillex works in a fresh checkout or git worktree without running `skillex refresh` first. The index is built into a temporary database and atomically renamed into place, so concurrent cold starts on a fresh tree are safe (a competing process sees either no index or the fully-built one, never a half-built database). Build progress is written to stderr only, keeping the `mcp` JSON-RPC stdout stream byte-clean. A healthy existing index is opened as-is (no staleness rebuild — `skillex refresh --check` remains the staleness gate). Set `SKILLEX_NO_AUTO_REFRESH=1` to disable auto-build and restore the previous `registry not found — run 'skillex refresh' first` behavior. `skillex refresh --check` and `skillex test validate --check` are unchanged and still fail on a missing/stale index (auto-build is scoped to `query`/`mcp` only); `refresh` and `doctor` behavior is unchanged. Course Studio fork enhancement with no upstream equivalent (Apache-2.0 §4(b) change notice).

## [0.9.0]

- **Added:** project-local and package-shipped *packs*. A `pack.yaml` manifest bundles skill files with their own activation rules (`activate-when`: `files-present` / `files-matching` / `dependency-declared`) and scope strategies (`repo` / `subtree` / `directory` / `matching-files` / `nearest-ancestor` / `boundary`). Project packs are discovered at `skillex/pack.yaml` and `skillex/packs/*/pack.yaml`; packages may ship `skillex/pack.yaml`. Pack skills are indexed with `source_type: pack`. Ported from upstream skillex (#31–#34); additive — existing `skillex.json` rules, `skillex/public`, and `skillex/private` behavior are unchanged.

## [0.8.1]

- **Changed:** releases now publish to npm via GitHub OIDC trusted publishing instead of a stored `NPM_TOKEN` secret — no long-lived credential. No functional changes to the CLI.

## [0.8.0]

Claude Code agent experience.

- **Added:** the generated AGENTS.md section now lists each skill (path, name, truncated description) grouped by scope, up to a configurable cutoff (default 40, set `CatalogCutoff` in skillex.json) above which it falls back to the taxonomy view. Combined with the upstream CLAUDE.md/GEMINI.md bridge, agents see the skill catalog passively.
- **Fixed:** `query --path` accepts an absolute path inside the repository (normalized to repo-relative); an absolute path outside the repository returns `no_match` with an explanatory note. Previously absolute paths silently returned only globally-scoped skills.
- **Fixed:** `search` now matches skill topics and tags in addition to names and descriptions, and escapes `%`/`_` so they are matched literally.
- **Changed:** `init --harness claude-code` writes a root `.mcp.json` (using `pnpm exec` when a pnpm lockfile/workspace is present, otherwise `npx`) instead of `.claude/mcp.json`, which Claude Code does not read.
- **Fixed:** MCP skill resources read their *content* live from the registry, so an edit to an existing skill is reflected mid-session instead of being served from a boot-time snapshot. (The set of advertised resources is still established at startup; a skill added or removed after boot is not yet reflected in the resource list.) The `skillex_query` tool description is tuned for agent discovery.

## [0.7.0]

First Course Studio fork release (fork of atheory-ai/skillex at v0.6.4).

- **Fork identity:** packages renamed to `@course-studio/skillex-by-jeremy` (+ platform packages); Go module moved to `github.com/course-studio/skillex`; CLI command unchanged (`skillex`).
- **Fixed:** a skill listed in multiple rules (multi-scope) no longer corrupts the registry; repo skills are deduplicated in the linker and `InsertSkill` uses `RETURNING id` instead of `LastInsertId` after upserts. Previously this caused silent cross-skill topic/tag corruption or `FOREIGN KEY constraint failed` warnings on every refresh.
- **Fixed:** `get`/`import` no longer strip `name`, `description`, and `reviewed` frontmatter (these drive `--search` discoverability); the YAML scalar writer now safely quotes values that would otherwise be misparsed (null/boolean keyword forms, control characters, leading indicators).
- **Fixed:** the `AGENTS.md` package listing is deterministic (sorted), keeping postinstall refresh idempotent.
- **Fixed:** commands now find the repo root from subdirectories by walking up to `skillex.json`/`skillex.yaml`; `init` still operates on the current directory.
- **Fixed:** the safety review is fail-closed — flagged content aborts in `--quiet` or non-interactive sessions instead of being vendored anyway. HTML payloads (including BOM-prefixed) are rejected. GitHub `/blob/` URLs are rewritten to raw automatically; `/tree/` URLs error with guidance. Fetches have a 30s timeout and a User-Agent.
- **Fixed:** refresh warns when a configured skill path is missing from disk instead of skipping silently.
- **Removed:** the no-op `import --visibility` flag.
- **Changed:** release binaries are built with `-s -w -trimpath` (about 31% smaller); the registry rebuild runs in a single transaction (atomic — no half-empty index window for a live MCP reader).

## [0.6.1]

- Added an npm-facing README and richer package metadata for `@atheory-ai/skillex` so the npm package page explains what Skillex is and how to use it.
- Added a guarded `make release-tag` helper to create and push the `v*` tag that triggers the GitHub Actions release workflow.
- Initial open source project scaffolding for contribution, security, and release process documentation.
- **Query:** `name` and `description` frontmatter fields indexed in the registry; `--search` (CLI) and `search` (MCP) for keyword discovery over those fields (multi-token OR). Schema migration v3 adds columns on existing databases.

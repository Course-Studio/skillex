# Changelog

All notable changes to this project should be documented in this file.

The format is based on Keep a Changelog and the project uses Semantic Versioning.

## [Unreleased]

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

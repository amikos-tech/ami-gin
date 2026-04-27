# Phase 1: Foundation - Research

**Researched:** 2026-03-24
**Domain:** Go module rename, OSS licensing, git artifact cleanup
**Confidence:** HIGH

## Summary

Phase 1 is a mechanical housekeeping phase with zero algorithmic complexity. It involves four discrete operations: adding an MIT LICENSE file, renaming the Go module path from `github.com/amikos-tech/gin-index` to `github.com/amikos-tech/ami-gin` across all source files, deleting an internal PRD file, and gitignoring the `.planning/` directory. All operations are well-understood, reversible, and independently verifiable.

The primary risk is the module rename (FOUND-02): a missed import or stale `go.sum` will cause build failures. Research confirms exactly 16 non-`.planning/` tracked files contain the old module path. The rename is safe because the repo has no external consumers (private repo, no published tags). The target GitHub repo `amikos-tech/ami-gin` already exists (private), so `go get` will work once the repo is public and the module path matches.

**Primary recommendation:** Execute as four sequential commits (one per requirement). For the module rename, use `sed` for bulk replacement, then `go mod tidy` to regenerate `go.sum`, then `go build ./...` and `go test ./...` to verify.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Add `.planning/` to `.gitignore` and remove from git tracking before going public. GSD workflow continues locally but process artifacts are not exposed to external users.
- **D-02:** No scrubbing of `.planning/` contents needed since the directory won't be tracked. Sensitive content (Kiba Team refs, private repo URL, security vulnerability catalog in CONCERNS.md) stays local-only.
- **D-03:** Minimal scrub -- the module path rename (FOUND-02) and PRD deletion (FOUND-03) handle all material internal references. No additional grep-and-clean pass needed beyond what those two changes cover.
- **D-04:** One commit per requirement -- 4 separate commits in this order:
  1. Add MIT LICENSE file (FOUND-01)
  2. Module path rename across all 26 files (FOUND-02) -- single isolated commit per STATE.md decision
  3. Delete `gin-index-prd.md` (FOUND-03)
  4. Add `.planning/` to `.gitignore` and untrack (FOUND-04)

### Claude's Discretion
- MIT LICENSE file content (standard template, year and copyright holder)
- Exact order of file edits within the module rename commit
- Whether to run `go mod tidy` as part of the rename or separately

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FOUND-01 | Repository has an MIT LICENSE file at the root | MIT template verified from choosealicense.com; amikos-tech org convention confirmed (copyright: "Amikos Tech Ltd.", year: 2026) |
| FOUND-02 | Module path changed from `github.com/amikos-tech/gin-index` to `github.com/amikos-tech/ami-gin` across go.mod, all imports, golangci-lint config, and examples | Exact file list enumerated (16 non-.planning files); sed-based bulk replacement is standard approach; go mod tidy regenerates go.sum |
| FOUND-03 | Internal PRD (`gin-index-prd.md`) removed from the repository | File confirmed to exist (49KB, contains 2 "Kiba" references); simple `git rm` |
| FOUND-04 | `.planning/` contents reviewed and any sensitive internal references removed | 19 files tracked under `.planning/`; decision is to gitignore entire directory rather than scrub individual files |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **License**: MIT
- **Module path**: Must be `github.com/amikos-tech/ami-gin`
- **Go version**: 1.25.5 (go.mod specifies this; local Go is 1.26.1 which is compatible)
- **No breaking API changes**: Existing API surface preserved
- **Conventional commits**: Required for all commits
- **No push to main without PR**: All changes go through PR
- **Radically simple**: No over-engineering
- **golangci-lint v2**: Currently at v2.11.4
- **Import ordering**: gci formatter with sections: standard, default, prefix(module-path), blank, dot

## Standard Stack

No new libraries are introduced in this phase. The work is entirely file editing, git operations, and Go toolchain commands.

### Core Tools
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Go toolchain | 1.26.1 (local) / 1.25.5 (go.mod) | Build, test, mod tidy | Standard Go development |
| golangci-lint | v2.11.4 | Lint verification after rename | Already configured |
| gotestsum | v1.13.0 | Test runner (via Makefile) | Already configured |
| sed | system | Bulk string replacement | Standard Unix tool for module rename |
| git | system | Version control | All changes committed per D-04 |

## Architecture Patterns

### Module Rename Pattern (Go Standard)

The Go module rename for a private-to-public transition follows this proven sequence:

1. **Update `go.mod` module directive** -- change the module line
2. **Update all Go imports** -- every `import` statement referencing the old path
3. **Update non-Go references** -- `.golangci.yml` (gci prefix), `README.md`, `CLAUDE.md`
4. **Regenerate `go.sum`** -- `go mod tidy` recalculates all checksums
5. **Verify** -- `go build ./...` && `go test ./...`

This is safe because:
- No external consumers exist (private repo, no published tags)
- All imports are internal (single module, no sub-modules)
- The target repo `amikos-tech/ami-gin` already exists on GitHub

### File Inventory for Module Rename

**go.mod (1 file):**
- Line 1: `module github.com/amikos-tech/gin-index` -> `module github.com/amikos-tech/ami-gin`

**Go source imports (12 files):**
- `cmd/gin-index/main.go` -- line 15: `gin "github.com/amikos-tech/gin-index"`
- `examples/basic/main.go` -- line 10
- `examples/full/main.go` -- line 10
- `examples/fulltext/main.go` -- line 10
- `examples/nested/main.go` -- line 10
- `examples/null/main.go` -- line 10
- `examples/parquet/main.go` -- line 10
- `examples/range/main.go` -- line 10
- `examples/regex/main.go` -- line 10
- `examples/serialize/main.go` -- line 10
- `examples/transformers/main.go` -- line 11
- `examples/transformers-advanced/main.go` -- line 10

**Config/docs (3 files):**
- `.golangci.yml` -- line 41: `prefix(github.com/amikos-tech/gin-index)`
- `README.md` -- lines 73, 83, 467 (installation, import example, CLI install)
- `CLAUDE.md` -- lines 196, 207, 416 (module path references, import order, triggers)

**Total: 16 non-`.planning/` tracked files.**

Note: 11 `.planning/` files also contain references, but per D-01/D-02, these will be gitignored and do not need updating.

### MIT LICENSE Pattern (amikos-tech Org Convention)

Based on the existing `amikos-tech/chromadb-chart` repo:

```
The MIT License

Copyright (c) 2026 Amikos Tech Ltd.

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

Year: 2026 (current). Copyright holder: "Amikos Tech Ltd." (matches org convention from chromadb-chart).

### .gitignore Addition Pattern

Current `.gitignore` has no `.planning/` entry. Add `.planning/` then `git rm -r --cached .planning/` to untrack without deleting local files.

### Anti-Patterns to Avoid
- **Partial rename:** Updating `go.mod` but missing a file. The build passes locally due to module cache but fails in CI (clean environment). Always verify with `go build ./...` after clearing cache.
- **Forgetting go.sum regeneration:** After module path change, `go.sum` contains checksums for the old module path. Must run `go mod tidy` to regenerate.
- **Editing .planning/ files:** Per D-02, do NOT update module path references in `.planning/` files. They will be gitignored.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bulk string replacement | Manual file-by-file edits | `sed -i '' 's|old|new|g' file` or editor bulk replace | 16 files need identical substitution; manual editing is error-prone |
| go.sum regeneration | Manual editing of go.sum | `go mod tidy` | go.sum is machine-generated; manual edits will be wrong |
| MIT license text | Writing license from memory | Copy from choosealicense.com or existing org repo | Legal text must be exact |

## Common Pitfalls

### Pitfall 1: Stale Module Cache Masks Missing Renames
**What goes wrong:** Build passes locally because Go caches resolved modules. A missed import still compiles against the cached old-path module.
**Why it happens:** `GOMODCACHE` retains the old module from previous builds.
**How to avoid:** After rename, run `go clean -modcache` (or at minimum `go mod tidy`) then `go build ./...` in a clean state. Alternatively, verify with `grep -r 'gin-index' --include='*.go' .` to confirm zero remaining references.
**Warning signs:** Build passes locally but fails in CI.

### Pitfall 2: .golangci.yml gci Prefix Not Updated
**What goes wrong:** `golangci-lint run` fails or auto-formats imports incorrectly because the `gci` formatter still references the old module path prefix.
**Why it happens:** `.golangci.yml` is not a `.go` file so it is easy to miss in a grep for imports.
**How to avoid:** Include `.golangci.yml` in the rename file list. Run `golangci-lint run` as part of verification.
**Warning signs:** Import ordering violations in lint output after rename.

### Pitfall 3: README Code Examples Still Show Old Path
**What goes wrong:** README shows `go get github.com/amikos-tech/gin-index` -- users copy-paste and get a 404.
**Why it happens:** Documentation files are often forgotten in module renames.
**How to avoid:** Include `README.md` and `CLAUDE.md` in the rename file list.
**Warning signs:** Manual review of README after rename.

### Pitfall 4: git rm --cached Without .gitignore Update
**What goes wrong:** Running `git rm -r --cached .planning/` without first adding `.planning/` to `.gitignore` means the files will be re-added on the next `git add .`.
**Why it happens:** Sequence error -- must add to `.gitignore` first, then untrack.
**How to avoid:** Always: (1) add to `.gitignore`, (2) `git rm -r --cached .planning/`, (3) commit both changes.
**Warning signs:** `.planning/` files reappear in `git status` after the commit.

### Pitfall 5: go.mod Go Version Compatibility
**What goes wrong:** `go.mod` says `go 1.25.5` but the local toolchain is `go1.26.1`. After `go mod tidy`, the go directive might get bumped.
**Why it happens:** `go mod tidy` can update the `go` directive to match the running toolchain.
**How to avoid:** After running `go mod tidy`, check that `go.mod` still says `go 1.25.5` (or the intended minimum version). If bumped, manually set it back if needed.
**Warning signs:** `go.mod` diff shows unexpected go version change.

## Code Examples

### Bulk Module Path Rename via sed

```bash
# For macOS sed (empty string for -i backup suffix)
# Update go.mod
sed -i '' 's|github.com/amikos-tech/gin-index|github.com/amikos-tech/ami-gin|g' go.mod

# Update all Go source files with imports
sed -i '' 's|github.com/amikos-tech/gin-index|github.com/amikos-tech/ami-gin|g' \
  cmd/gin-index/main.go \
  examples/*/main.go

# Update config and docs
sed -i '' 's|github.com/amikos-tech/gin-index|github.com/amikos-tech/ami-gin|g' \
  .golangci.yml README.md CLAUDE.md

# Regenerate go.sum
go mod tidy

# Verify
go build ./...
go test ./...
golangci-lint run
```

### Untrack .planning/ Directory

```bash
# Step 1: Add to .gitignore
echo '.planning/' >> .gitignore

# Step 2: Remove from git tracking (keeps local files)
git rm -r --cached .planning/

# Step 3: Commit
git add .gitignore
git commit -m "chore: add .planning/ to .gitignore and untrack"
```

### Verify No Remaining References

```bash
# Check for any remaining old module path in tracked Go files
git ls-files '*.go' | xargs grep -l 'gin-index' 2>/dev/null
# Should return empty

# Check non-Go tracked files
git ls-files | grep -v '^\.planning/' | xargs grep -l 'gin-index' 2>/dev/null
# Should return empty (after all 4 commits)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `GO111MODULE=on` needed for module builds | Module mode is default since Go 1.16 | Go 1.16 (2021) | No need for env var |
| `go mod init` for new module | Same, unchanged | Stable | N/A |
| golangci-lint v1 config format | golangci-lint v2 config format (version: "2") | golangci-lint v2.0 (2025) | Config already uses v2 format |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + gopter (property-based) |
| Config file | None (standard `go test`) |
| Quick run command | `go build ./... && go test -v -count=1 -timeout=5m ./...` |
| Full suite command | `make test` (runs gotestsum with coverage) |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 | MIT LICENSE file exists at root | smoke | `test -f LICENSE && head -1 LICENSE` | N/A (file check) |
| FOUND-02 | Module path is ami-gin everywhere, build+test pass | integration | `go build ./... && go test -count=1 -timeout=5m ./...` | Existing tests cover this |
| FOUND-02 | No old module path references remain | smoke | `git ls-files '*.go' '*.yml' '*.md' \| grep -v '\.planning/' \| xargs grep -c 'amikos-tech/gin-index' \| grep -v ':0$'` (should be empty) | N/A (grep check) |
| FOUND-02 | Lint passes with new gci prefix | lint | `golangci-lint run` | N/A (lint tool) |
| FOUND-03 | PRD file deleted | smoke | `test ! -f gin-index-prd.md` | N/A (file check) |
| FOUND-04 | .planning/ not tracked | smoke | `git ls-files .planning/ \| wc -l` (should be 0) | N/A (git check) |

### Sampling Rate
- **Per task commit:** `go build ./... && go test -v -count=1 -timeout=5m ./...`
- **Per wave merge:** `make test && golangci-lint run`
- **Phase gate:** Full suite green + all smoke checks pass before `/gsd:verify-work`

### Wave 0 Gaps
None -- existing test infrastructure covers all phase requirements. No new test files needed. Validation is via build/test/lint commands and file existence checks.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build, test, mod tidy | Yes | 1.26.1 | -- |
| golangci-lint | Lint verification | Yes | v2.11.4 | -- |
| gotestsum | make test | Yes | v1.13.0 | `go test` directly |
| sed | Bulk rename | Yes | system (BSD) | Manual edit |
| git | All commits | Yes | system | -- |
| GitHub CLI (gh) | PR creation | Yes | system | -- |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None.

## Open Questions

1. **Copyright year: 2026 or 2024-2026?**
   - What we know: The earliest commit is from Jan 2025 based on file dates. The org convention from chromadb-chart uses a single year (2023).
   - What's unclear: Whether the copyright year should be the year of first authorship or the year of public release.
   - Recommendation: Use `2026` (year of public release), matching org convention of single year. This is Claude's discretion per CONTEXT.md.

2. **LICENSE discrepancy in STATE.md**
   - What we know: STATE.md notes "PROJECT.md says Apache-2.0 throughout but REQUIREMENTS.md (FOUND-01) says MIT." CONTEXT.md decisions clearly state MIT. CLAUDE.md constraint says MIT.
   - What's unclear: Nothing -- this is resolved. MIT is correct.
   - Recommendation: Use MIT. The discrepancy in PROJECT.md is a doc artifact; REQUIREMENTS.md and CLAUDE.md are authoritative.

3. **go.mod go directive after mod tidy**
   - What we know: go.mod says `go 1.25.5`, local toolchain is 1.26.1. `go mod tidy` may bump the directive.
   - What's unclear: Whether the bump will happen (depends on Go toolchain behavior).
   - Recommendation: Check after `go mod tidy`. If bumped, decide whether to keep 1.25.5 (broader compat) or accept the bump. Per CLAUDE.md: "Go version: 1.25.5 (already current)".

## Sources

### Primary (HIGH confidence)
- [choosealicense.com/licenses/mit/](https://choosealicense.com/licenses/mit/) -- MIT license template text
- [go.dev/wiki/Resolving-Problems-From-Modified-Module-Path](https://go.dev/wiki/Resolving-Problems-From-Modified-Module-Path) -- Go module rename guidance
- Local codebase grep -- exact file inventory for module path references (16 files)
- `amikos-tech/chromadb-chart` LICENSE -- org convention for copyright format

### Secondary (MEDIUM confidence)
- [appliedgo.net/spotlight/rename-or-relocate-a-public-go-module/](https://appliedgo.net/spotlight/rename-or-relocate-a-public-go-module/) -- Go module rename patterns
- [feliciano.tech/blog/3-ways-to-rename-a-go-module/](https://www.feliciano.tech/blog/3-ways-to-rename-a-go-module/) -- Rename approaches

### Tertiary (LOW confidence)
None -- all findings verified from primary sources.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new libraries, just Go toolchain commands
- Architecture: HIGH -- exact file list enumerated, pattern is mechanical string replacement
- Pitfalls: HIGH -- well-known Go module rename pitfalls, all verifiable

**Research date:** 2026-03-24
**Valid until:** 2026-04-24 (stable -- nothing here changes quickly)

---
phase: 20
slug: realistic-benchmark-dataset-foundation
status: verified
threats_open: 0
asvs_level: 1
created: 2026-07-21
---

# Phase 20 — Security

> Retroactive STRIDE audit. The phase plans predate formal threat registers, so this register was derived from the implemented Phase-20 fixture and local-input boundaries.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Fixture refresh | A developer refreshes committed Phase-20 JSONL files. | Generated JSONL bytes written to the checked-in fixture directory. |
| Optional external benchmark input | A developer explicitly enables reading a local directory. | Untrusted local JSON/JSONL bytes enter benchmark setup. |
| Committed smoke fixtures | Checked-in data is loaded into tests and benchmarks. | Fixture bytes enter JSON validation and index construction. |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T20-S01 | Spoofing | External-tier activation | mitigate | Only the exact enable value `1` unlocks local input; all other values return before reading the directory setting. | closed |
| T20-T01 | Tampering | Checked-in smoke fixtures | mitigate | Tests validate JSON, count and size limits, semantic shape, source kind, and byte-for-byte combined-stream provenance. | closed |
| T20-I01 | Information disclosure | Fixture provenance | mitigate | Fixtures are synthesized and checked in; documentation prohibits upstream-row copying, downloads, and vendoring. | closed |
| T20-E01 | Elevation of privilege | Generator and local tier | mitigate | The generator has four fixed outputs and no command or network execution; the local tier accepts only regular top-level JSON-family files. | closed |
| T20-T02 | Tampering | Fixture refresh generator | mitigate | The generator rejects symlink and non-regular destinations, then atomically renames a temporary file created inside the fixture directory. | closed |
| T20-D01 | Denial of service | Enabled local-data tier | mitigate | Local input is limited to 64 files, 8 MiB total, 1 MiB per document, and 64 traversal levels; it is read as a bounded stream and counts LF and CRLF delimiters. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-21 | 6 | 6 | 0 | Codex security audit |

## Security Audit 2026-07-21

| Metric | Count |
|--------|-------|
| Threats found | 6 |
| Closed | 6 |
| Open | 0 |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-21

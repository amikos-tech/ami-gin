# Quick 261006-mlx: Deferred items

Items found during grilling. Not in scope for this task. Each needs its own issue and decision.

## 1. NE / NIN matching absent documents

Today `NE(P, v)` and `NIN` require P to be present. MongoDB `$ne`, Elasticsearch `must_not term` and
Prometheus `!=` match absent documents too; the SQL family does not
(`261006-mlx-RESEARCH-NULL-SEMANTICS.md`). If NE matched absent, TCLR's `NE ∪ IsNull` would not need the
`IsNull` term. Separate behavior change, not requested.

## 2. Strict "explicit null only" operator

D1 makes `IsNull` broad (null or absent). A narrow operator (like MongoDB `$type:"null"` or Snowflake
`IS_NULL_VALUE`) stays possible as an additive change. Add only when a caller asks.

## 3. Release cut

Q3 decided a minor release (v1.5.0). This task writes the CHANGELOG Unreleased entries only. The release
cut is a separate `chore(release)` commit.

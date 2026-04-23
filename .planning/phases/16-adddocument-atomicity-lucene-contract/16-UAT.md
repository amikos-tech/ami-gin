---
status: complete
phase: 16-adddocument-atomicity-lucene-contract
source: [16-01-SUMMARY.md, 16-02-SUMMARY.md, 16-03-SUMMARY.md, 16-04-SUMMARY.md]
started: 2026-04-23T11:13:14Z
updated: 2026-04-23T11:18:31Z
---

## Current Test

[testing complete]

## Tests

### 1. Non-Tragic AddDocument Atomicity
expected: When AddDocument returns a normal public error for a bad document, the failed call leaves no partial index mutation. Building from the full attempted corpus and building from only the successful documents should produce identical encoded bytes while preserving original document IDs.
result: pass

### 2. Public Failure Catalog Stays Non-Tragic
expected: Parser, staging, transformer, numeric, pre-parser gate, parser-contract, uint overflow, and unsupported-number failures should return errors without setting tragicErr, and the same builder should continue accepting later valid documents.
result: pass

### 3. Lossy Numeric Promotion Is Validator-Rejected
expected: Unsafe mixed numeric promotion in either direction should be rejected before merge mutation, the rejected document should not poison the builder, and a later valid document should still be indexable.
result: pass

### 4. Merge Panic Becomes Tragic State
expected: A panic inside the merge-only path should be recovered into a tragic builder error, the current AddDocument should return that error without document bookkeeping, later AddDocument calls should be refused with the original tragic cause, and recovery logging should include only safe type attributes.
result: pass

### 5. Validator Marker Policy Is Enforced
expected: The three merge-layer validator markers should directly precede their functions, marked signatures should not return error, `make check-validator-markers` should pass, `make lint` should run that policy, and CI should include the same marker check in the lint job.
result: pass

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]

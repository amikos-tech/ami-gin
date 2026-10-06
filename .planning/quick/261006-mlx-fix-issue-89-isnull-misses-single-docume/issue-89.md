## Problem

`IsNull(path)` does not return a row group whose only document does not carry the path. `NE(path, v) ∪ IsNull(path)` then misses that row group too. A caller that prunes on this result skips a row group that holds a matching document.

`IsNull` is `NullRGBitmap ∪ AggregateIndex.AbsentRGs` (`query.go:704-708`). `AbsentRGs` marks only row groups "holding at least two documents where at least one document does not carry the path" (`gin.go:246-249`). A row group with one document that lacks the path is in neither set.

## Reproduction (v1.4.0)

```go
b, _ := gin.NewBuilder(gin.DefaultConfig(), 4)
docs := []struct{ rg int; j string }{
    {0, `{"env":"prod"}`}, {0, `{"env":"prod"}`},
    {1, `{"env":"prod"}`},
    {2, `{"app":"x"}`}, {2, `{"app":"y"}`}, // 2 documents, no env
    {3, `{"app":"z"}`},                     // 1 document, no env
}
for _, d := range docs { _ = b.AddDocument(gin.DocID(d.rg), []byte(d.j)) }
idx := b.Finalize()

idx.Evaluate([]gin.Predicate{gin.IsNull("$.env")})    // [2]   want [2 3]
idx.Evaluate([]gin.Predicate{gin.NE("$.env", "prod")}) // []
```

Same result on v1.2.0 and v1.3.0.

## Expected

`IsNull(path)` returns every row group that holds at least one document without the path, whatever the document count of the row group. The `AbsentRGs` field comment and the `IsNull` godoc describe the same rule.

## Impact

A consumer that evaluates "not equal, or label absent" as `NE ∪ IsNull` under-selects. In TCLR a log line in such a row group is never scanned. The short last row group of a file is the common case. TCLR now stops pruning on these operators until a fixed release is out (chaoslabs-bg/tclr-v2#4055).

## Acceptance

- [ ] The reproduction above returns `[2 3]` for `IsNull($.env)`.
- [ ] A test covers a one-document row group without the path, and a row group where the path is absent from every document.
- [ ] The wire format decision is stated: either the fix is build-side only (no format change, old indexes still under-select), or the format version changes. Old indexes must not be reported as complete if they can still under-select.


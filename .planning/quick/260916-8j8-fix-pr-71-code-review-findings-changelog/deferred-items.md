# Deferred Items

## Pre-existing test failure (out of scope)

- **Test:** `TestAddDocumentDefaultParserErrorStringsPreserved/unterminated-object`
- **File:** `parser_test.go`
- **Symptom:** `AddDocument err = "ingest parser failure: read object key at $: unexpected end of JSON input", want substring "close object at $"`
- **Cause:** Local toolchain is Go 1.27.1; `go.mod` targets 1.25.5. The stdlib
  `encoding/json` error wording for unterminated objects differs between these
  Go versions, so the hardcoded substring assertion no longer matches.
- **Why deferred:** Unrelated to this plan's scope (CHANGELOG.md, regex.go,
  regex_soundness_test.go — documentation/comment/test-only changes for PR
  #71 review findings). Not caused by any change in this plan. Confirmed
  pre-existing by running the test in isolation; failure occurs regardless
  of this plan's edits.
- **Action:** Not fixed here per deviation-rules scope boundary. Left for a
  separate task/issue to either pin the CI Go version or make the parser
  test assertion Go-version-tolerant.

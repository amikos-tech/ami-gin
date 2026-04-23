package gin

import (
	"strings"
	"testing"
)

func TestIngestFailureModeDefaultsAndValidation(t *testing.T) {
	defaults := DefaultConfig()
	if defaults.ParserFailureMode != IngestFailureHard {
		t.Fatalf("DefaultConfig().ParserFailureMode = %q, want %q", defaults.ParserFailureMode, IngestFailureHard)
	}
	if defaults.NumericFailureMode != IngestFailureHard {
		t.Fatalf("DefaultConfig().NumericFailureMode = %q, want %q", defaults.NumericFailureMode, IngestFailureHard)
	}

	cfg, err := NewConfig(
		WithParserFailureMode(IngestFailureSoft),
		WithNumericFailureMode(IngestFailureSoft),
		WithToLowerTransformer("$.email", "lower", WithTransformerFailureMode(IngestFailureSoft)),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}
	if cfg.ParserFailureMode != IngestFailureSoft {
		t.Fatalf("ParserFailureMode = %q, want %q", cfg.ParserFailureMode, IngestFailureSoft)
	}
	if cfg.NumericFailureMode != IngestFailureSoft {
		t.Fatalf("NumericFailureMode = %q, want %q", cfg.NumericFailureMode, IngestFailureSoft)
	}
	specs := cfg.representationSpecs["$.email"]
	if len(specs) != 1 {
		t.Fatalf("representationSpecs[$.email] len = %d, want 1", len(specs))
	}
	if specs[0].Transformer.FailureMode != IngestFailureSoft {
		t.Fatalf("Transformer.FailureMode = %q, want %q", specs[0].Transformer.FailureMode, IngestFailureSoft)
	}

	builder := mustNewBuilder(t, GINConfig{
		BloomFilterSize:   1024,
		BloomFilterHashes: 2,
		EnableTrigrams:    true,
		TrigramMinLength:  3,
		HLLPrecision:      12,
		PrefixBlockSize:   16,
	}, 2)
	if builder.config.ParserFailureMode != IngestFailureHard {
		t.Fatalf("builder.config.ParserFailureMode = %q, want %q", builder.config.ParserFailureMode, IngestFailureHard)
	}
	if builder.config.NumericFailureMode != IngestFailureHard {
		t.Fatalf("builder.config.NumericFailureMode = %q, want %q", builder.config.NumericFailureMode, IngestFailureHard)
	}

	if _, err := NewConfig(WithParserFailureMode(IngestFailureMode("invalid"))); err == nil {
		t.Fatal("NewConfig(WithParserFailureMode(invalid)) error = nil, want validation failure")
	} else if !strings.Contains(err.Error(), "invalid ingest failure mode") {
		t.Fatalf("parser mode error = %v, want invalid ingest failure mode", err)
	}
	if _, err := NewConfig(WithNumericFailureMode(IngestFailureMode("invalid"))); err == nil {
		t.Fatal("NewConfig(WithNumericFailureMode(invalid)) error = nil, want validation failure")
	} else if !strings.Contains(err.Error(), "invalid ingest failure mode") {
		t.Fatalf("numeric mode error = %v, want invalid ingest failure mode", err)
	}
	if _, err := NewConfig(WithToLowerTransformer("$.email", "lower", WithTransformerFailureMode(IngestFailureMode("invalid")))); err == nil {
		t.Fatal("NewConfig(WithTransformerFailureMode(invalid)) error = nil, want validation failure")
	} else if !strings.Contains(err.Error(), "invalid transformer failure mode") {
		t.Fatalf("transformer mode error = %v, want invalid transformer failure mode", err)
	}
}

func TestValidateIngestFailureModeRejectsLegacyTokens(t *testing.T) {
	if err := validateIngestFailureMode(IngestFailureMode("strict")); err == nil {
		t.Fatal(`validateIngestFailureMode(IngestFailureMode("strict")) error = nil, want validation failure`)
	} else if !strings.Contains(err.Error(), "invalid ingest failure mode") {
		t.Fatalf("strict ingest mode error = %v, want invalid ingest failure mode", err)
	}
	if err := validateIngestFailureMode(IngestFailureMode("soft_fail")); err == nil {
		t.Fatal(`validateIngestFailureMode(IngestFailureMode("soft_fail")) error = nil, want validation failure`)
	} else if !strings.Contains(err.Error(), "invalid ingest failure mode") {
		t.Fatalf("soft_fail ingest mode error = %v, want invalid ingest failure mode", err)
	}

	if err := validateTransformerFailureMode(IngestFailureMode("strict")); err != nil {
		t.Fatalf(`validateTransformerFailureMode(IngestFailureMode("strict")) error = %v, want nil`, err)
	}
	if err := validateTransformerFailureMode(IngestFailureMode("soft_fail")); err != nil {
		t.Fatalf(`validateTransformerFailureMode(IngestFailureMode("soft_fail")) error = %v, want nil`, err)
	}
	if got := normalizeTransformerFailureMode(IngestFailureMode("strict")); got != IngestFailureHard {
		t.Fatalf(`normalizeTransformerFailureMode(IngestFailureMode("strict")) = %q, want %q`, got, IngestFailureHard)
	}
	if got := normalizeTransformerFailureMode(IngestFailureMode("soft_fail")); got != IngestFailureSoft {
		t.Fatalf(`normalizeTransformerFailureMode(IngestFailureMode("soft_fail")) = %q, want %q`, got, IngestFailureSoft)
	}
}

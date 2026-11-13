package gin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// companionTransformer is what the index holds for one companion path: the
// representation-section spec (always present) and the transformer function
// (nil when it is unknown, for example when reconstruction failed).
type companionTransformer struct {
	spec TransformerSpec
	fn   FieldTransformer
	// degraded is set for a regex companion that must return every row group
	// (#115). The query guard reads it; the function is never consulted for
	// that decision.
	degraded *companionDegradation
}

// isLoweringSpec reports whether spec describes a transformer that lowercases
// with the frozen Unicode tables: ToLower, EmailDomain or URLHost. It matches
// on the transformer ID OR the transformer name, never on the alias, so a
// tampered representation section that zeroes only one of the two fields still
// classifies as lowering.
func isLoweringSpec(spec TransformerSpec) bool {
	switch spec.ID {
	case TransformerToLower, TransformerEmailDomain, TransformerURLHost:
		return true
	}
	switch spec.Name {
	case transformerNames[TransformerToLower], transformerNames[TransformerEmailDomain], transformerNames[TransformerURLHost]:
		return true
	}
	return false
}

// buildCompanionTransformers fills idx.companionTransformers from
// idx.representations. Finalize passes the builder's Config, which keeps
// in-memory custom functions. Decode passes the freshly decoded Config, after
// checkRepresentationSpecs has accepted it. cfg is used at build time only:
// Normalized and the companionReturnsAllRGs guard read the index-owned map,
// never idx.Config.
//
// A representation the Config lists takes the Config's function and
// degradation, so an index finalized from a decoded config inherits it. A
// regex representation the Config does not list goes through
// classifyRegexCompanion; invalid params return an error, which fails
// Decode. Any other representation the Config does not list falls back to
// the registry; failure to reconstruct leaves fn nil and never fails Decode.
//
// It also fills idx.degradedCompanions from both the Config and the
// representations, so a degraded companion that holds no values is listed.
// The two sources never list the same (source path, alias): a representation
// the Config lists takes the Config's degradation, and duplicate aliases are
// rejected earlier.
//
// A GINIndex built by hand, without Finalize or Decode, has no companion
// lookup. Normalized then returns every row group, and the unknown-token guard
// cannot fire. Such an index carries no token either.
func (idx *GINIndex) buildCompanionTransformers(cfg *GINConfig) error {
	idx.companionTransformers = nil
	idx.degradedCompanions = nil
	var degradedCompanions []DegradedCompanion
	// A degraded config companion that holds no values has no representation
	// entry; it is still listed.
	for _, representation := range collectRepresentationsFromConfig(cfg) {
		if registered, ok := configRepresentationFor(cfg, representation); ok && registered.degraded != nil {
			degradedCompanions = append(degradedCompanions, newDegradedCompanion(representation, registered.degraded))
		}
	}
	var companions map[uint16]companionTransformer
	if len(idx.representations) > 0 {
		companions = make(map[uint16]companionTransformer, len(idx.representations))
	}
	for _, representation := range idx.representations {
		pathID, ok := idx.pathLookup[representation.TargetPath]
		if !ok {
			continue
		}
		entry := companionTransformer{spec: representation.Transformer}
		if registered, ok := configRepresentationFor(cfg, representation); ok {
			entry.fn = registered.FieldTransformer
			entry.degraded = registered.degraded
		} else if isRegexTransformerSpec(representation.Transformer) {
			fn, degraded, err := classifyRegexCompanion(representation.Transformer)
			if err != nil {
				return errors.Wrapf(err, "representation %s on %s", representation.Alias, representation.SourcePath)
			}
			entry.fn, entry.degraded = fn, degraded
			if degraded != nil {
				degradedCompanions = append(degradedCompanions, newDegradedCompanion(representation, degraded))
			}
		} else if representation.Transformer.ID != TransformerUnknown {
			if fn, err := ReconstructTransformer(representation.Transformer.ID, representation.Transformer.Params); err == nil {
				entry.fn = fn
			}
		}
		companions[pathID] = entry
	}
	sort.Slice(degradedCompanions, func(i, j int) bool {
		if degradedCompanions[i].SourcePath != degradedCompanions[j].SourcePath {
			return degradedCompanions[i].SourcePath < degradedCompanions[j].SourcePath
		}
		return degradedCompanions[i].Alias < degradedCompanions[j].Alias
	})
	idx.companionTransformers = companions
	idx.degradedCompanions = degradedCompanions
	return nil
}

// newDegradedCompanion returns the inspection record of one degraded
// companion.
func newDegradedCompanion(representation RepresentationSpec, degradation *companionDegradation) DegradedCompanion {
	return DegradedCompanion{
		SourcePath: representation.SourcePath,
		Alias:      representation.Alias,
		Reason:     degradation.reason,
		Detail:     degradation.detail,
	}
}

// checkRepresentationSpecs runs on Decode only. A representation whose
// (source path, alias) the Config also lists must agree with the Config spec
// on transformer ID, name and params. Otherwise a tampered representation
// section could hide a lowering companion from the guard or change what
// Normalized computes. Finalize does not call it: there both sections come
// from the same Config.
func (idx *GINIndex) checkRepresentationSpecs(cfg *GINConfig) error {
	for _, representation := range idx.representations {
		registered, ok := configRepresentationFor(cfg, representation)
		if !ok {
			continue
		}
		configSpec := registered.Transformer
		if configSpec.ID != representation.Transformer.ID || configSpec.Name != representation.Transformer.Name {
			return errors.Wrapf(ErrInvalidFormat, "representation %s on %s disagrees with config transformer (id %d/%d, name %q/%q)",
				representation.Alias, representation.SourcePath,
				representation.Transformer.ID, configSpec.ID,
				representation.Transformer.Name, configSpec.Name)
		}
		if !transformerParamsEqual(configSpec.Params, representation.Transformer.Params) {
			// The payloads can be near maxConfigSize: name only their lengths.
			return errors.Wrapf(ErrInvalidFormat, "representation %s on %s disagrees with config transformer params (lengths %d/%d bytes)",
				representation.Alias, representation.SourcePath,
				len(representation.Transformer.Params), len(configSpec.Params))
		}
		// Identical bytes decode identically; only different bytes need the
		// wire comparison.
		if isRegexTransformerSpec(configSpec) && !bytes.Equal(configSpec.Params, representation.Transformer.Params) {
			if err := regexWireParamsAgree(configSpec.Params, representation.Transformer.Params); err != nil {
				return errors.Wrapf(ErrInvalidFormat, "representation %s on %s disagrees with config regex params: %v",
					representation.Alias, representation.SourcePath, err)
			}
		}
	}
	return nil
}

// regexWireParamsAgree compares two regex params payloads as decoded wire
// params (pattern, group, source_pattern, frozen_unicode). JSON value
// equality is not enough: {"pattern":"a","PATTERN":"b"} and
// {"PATTERN":"b","pattern":"a"} are equal JSON objects, but encoding/json
// keeps the last case-insensitive match, so they decode to different
// executable patterns. Stored bytes are never rewritten.
func regexWireParamsAgree(configParams, representationParams json.RawMessage) error {
	configWire, _, err := decodeRegexWireParams(configParams)
	if err != nil {
		return errors.Wrap(err, "config params")
	}
	representationWire, _, err := decodeRegexWireParams(representationParams)
	if err != nil {
		return errors.Wrap(err, "representation params")
	}
	if configWire != representationWire {
		return errors.Errorf("decoded params differ: %s", regexWireParamsDiff(representationWire, configWire))
	}
	return nil
}

// regexWireParamsDiff names the fields that differ between the representation
// and the config wire params. For a string field it gives only the lengths:
// a frozen pattern can be near maxConfigSize, so the text is never printed.
func regexWireParamsDiff(representation, config regexWireParams) string {
	var diffs []string
	addString := func(name, a, b string) {
		if a != b {
			diffs = append(diffs, fmt.Sprintf("%s (lengths %d/%d bytes)", name, len(a), len(b)))
		}
	}
	addString("pattern", representation.Pattern, config.Pattern)
	if representation.Group != config.Group {
		diffs = append(diffs, fmt.Sprintf("group (%d/%d)", representation.Group, config.Group))
	}
	addString(regexParamSourcePattern, representation.SourcePattern, config.SourcePattern)
	addString(regexParamFrozenUnicode, representation.FrozenUnicode, config.FrozenUnicode)
	return strings.Join(diffs, ", ")
}

// configRepresentationFor returns the Config entry with the representation's
// source path and alias. The Config rejects duplicate aliases, so there is at
// most one.
func configRepresentationFor(cfg *GINConfig, representation RepresentationSpec) (registeredRepresentation, bool) {
	for _, registered := range cfg.representations(representation.SourcePath) {
		if registered.Alias == representation.Alias {
			return registered, true
		}
	}
	return registeredRepresentation{}, false
}

// transformerParamsEqual compares two transformer parameter payloads as JSON
// values, so formatting differences do not matter. An empty payload equals
// JSON null. A payload that is not valid JSON equals only identical bytes.
func transformerParamsEqual(a, b json.RawMessage) bool {
	if bytes.Equal(a, b) {
		return true
	}
	decode := func(raw json.RawMessage) (any, bool) {
		if len(raw) == 0 {
			return nil, true
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, false
		}
		return v, true
	}
	av, aok := decode(a)
	bv, bok := decode(b)
	if !aok || !bok {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(av, bv)
}

// companionReturnsAllRGs reports whether pathID is a companion that this
// reader cannot answer exactly, so every predicate on it returns every row
// group. Two cases:
//   - a degraded regex companion (legacy, uncompilable, timed out or
//     unverified, #115);
//   - a ToLower, EmailDomain or URLHost companion of an index whose
//     trigram_unicode_version token names tables this reader does not have,
//     so the companion terms may hold a lowercase mapping it cannot
//     reproduce (#102).
func (idx *GINIndex) companionReturnsAllRGs(pathID int) bool {
	companion, ok := idx.companionTransformers[uint16(pathID)]
	if !ok {
		return false
	}
	if companion.degraded != nil {
		return true
	}
	return isLoweringSpec(companion.spec) && !trigramUnicodeTokenSupported(idx.trigramUnicodeToken)
}

// normalizeQueryValue applies the companion's transformer to the raw value of
// a Normalized predicate. It reports false when the predicate must return
// every row group.
func (idx *GINIndex) normalizeQueryValue(op Operator, pathID int, raw any, listForm bool) (any, bool) {
	switch op {
	case OpEQ, OpNE, OpGT, OpGTE, OpLT, OpLTE:
		if listForm {
			return nil, false
		}
	case OpIN, OpNIN:
		if !listForm {
			return nil, false
		}
	default:
		return nil, false
	}

	companion, ok := idx.companionTransformers[uint16(pathID)]
	if !ok || companion.fn == nil {
		return nil, false
	}
	if !listForm {
		return callCompanionTransformer(companion.fn, raw)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, ok := callCompanionTransformer(companion.fn, item)
		if !ok {
			return nil, false
		}
		out[i] = value
	}
	return out, true
}

// callCompanionTransformer runs a companion transformer on a raw query value.
// Query values need not have the types that ingest sees (a Go int, not a JSON
// float64), so a transformer that never panics at ingest can panic here. A
// panic reports ok=false, and the predicate returns every row group.
func callCompanionTransformer(fn FieldTransformer, raw any) (out any, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = nil, false
		}
	}()
	return fn(prepareTransformerValue(raw))
}

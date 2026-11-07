package gin

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"
)

// companionTransformer is what the index holds for one companion path: the
// representation-section spec (always present) and the transformer function
// (nil when it is unknown, for example when reconstruction failed).
type companionTransformer struct {
	spec TransformerSpec
	fn   FieldTransformer
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
// Normalized and the unknown-token guard read the index-owned map, never
// idx.Config.
//
// A representation the Config lists takes the Config's function. A
// representation the Config does not list falls back to the registry. Failure
// to reconstruct leaves fn nil and never fails Decode.
//
// A GINIndex built by hand, without Finalize or Decode, has no companion
// lookup. Normalized then returns every row group, and the unknown-token guard
// cannot fire. Such an index carries no token either.
func (idx *GINIndex) buildCompanionTransformers(cfg *GINConfig) {
	idx.companionTransformers = nil
	if len(idx.representations) == 0 {
		return
	}
	companions := make(map[uint16]companionTransformer, len(idx.representations))
	for _, representation := range idx.representations {
		pathID, ok := idx.pathLookup[representation.TargetPath]
		if !ok {
			continue
		}
		entry := companionTransformer{spec: representation.Transformer}
		if registered, ok := configRepresentationFor(cfg, representation); ok {
			entry.fn = registered.FieldTransformer
		} else if representation.Transformer.ID != TransformerUnknown {
			if fn, err := ReconstructTransformer(representation.Transformer.ID, representation.Transformer.Params); err == nil {
				entry.fn = fn
			}
		}
		companions[pathID] = entry
	}
	idx.companionTransformers = companions
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
			return errors.Wrapf(ErrInvalidFormat, "representation %s on %s disagrees with config transformer params (%s/%s)",
				representation.Alias, representation.SourcePath,
				representation.Transformer.Params, configSpec.Params)
		}
	}
	return nil
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

// unknownUnicodeLoweringCompanion reports whether pathID is a ToLower,
// EmailDomain or URLHost companion that this reader cannot answer exactly: the
// index carries a trigram_unicode_version token whose tables this reader does
// not have, so the companion terms may hold a lowercase mapping it cannot
// reproduce. Callers return every row group.
func (idx *GINIndex) unknownUnicodeLoweringCompanion(pathID int) bool {
	if trigramUnicodeTokenSupported(idx.trigramUnicodeToken) {
		return false
	}
	companion, ok := idx.companionTransformers[uint16(pathID)]
	return ok && isLoweringSpec(companion.spec)
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

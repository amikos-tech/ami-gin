package gin

import (
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
// in-memory custom functions. Decode passes the freshly decoded Config. cfg is
// used at build time only: Normalized and the unknown-token guard read the
// index-owned map, never idx.Config.
//
// A representation whose (source path, alias) the Config also lists must agree
// with the Config spec on transformer ID and name, otherwise a tampered
// representation section could hide a lowering companion from the guard. A
// representation the Config does not list is accepted and falls back to the
// registry; failure to reconstruct leaves fn nil and never fails Decode (L3).
//
// Hand-constructed GINIndex values (the fallback in rebuildRepresentationLookup
// that never ran Finalize or Decode) get no companion transformer lookup, so
// Normalized fails open to every row group for them and the unknown-token
// guard cannot fire. They carry no token either.
func (idx *GINIndex) buildCompanionTransformers(cfg *GINConfig) error {
	idx.companionTransformers = nil
	if len(idx.representations) == 0 {
		return nil
	}
	companions := make(map[uint16]companionTransformer, len(idx.representations))
	for _, representation := range idx.representations {
		pathID, ok := idx.pathLookup[representation.TargetPath]
		if !ok {
			continue
		}
		entry := companionTransformer{spec: representation.Transformer}
		found := false
		for _, registered := range cfg.representations(representation.SourcePath) {
			if registered.Alias != representation.Alias {
				continue
			}
			found = true
			configSpec := registered.Transformer
			if configSpec.ID != representation.Transformer.ID || configSpec.Name != representation.Transformer.Name {
				return errors.Wrapf(ErrInvalidFormat, "representation %s on %s disagrees with config transformer (id %d/%d, name %q/%q)",
					representation.Alias, representation.SourcePath,
					representation.Transformer.ID, configSpec.ID,
					representation.Transformer.Name, configSpec.Name)
			}
			entry.fn = registered.FieldTransformer
			break
		}
		if !found && representation.Transformer.ID != TransformerUnknown {
			if fn, err := ReconstructTransformer(representation.Transformer.ID, representation.Transformer.Params); err == nil {
				entry.fn = fn
			}
		}
		companions[pathID] = entry
	}
	idx.companionTransformers = companions
	return nil
}

// unknownUnicodeLoweringCompanion reports whether pathID is a ToLower,
// EmailDomain or URLHost companion that this reader cannot answer exactly: the
// index carries a trigram_unicode_version token whose tables this reader does
// not have, so the companion terms may hold a lowercase mapping it cannot
// reproduce. Callers return every row group.
//
// Reader-upgrade advisory: v1.6.0 readers prune lowering companions under an
// unknown token. Upgrade all readers to this release before any writer emits a
// new token (#102).
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
		out, ok := companion.fn(prepareTransformerValue(raw))
		return out, ok
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, ok := companion.fn(prepareTransformerValue(item))
		if !ok {
			return nil, false
		}
		out[i] = value
	}
	return out, true
}

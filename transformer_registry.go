package gin

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/pkg/errors"
)

type TransformerID uint8

const (
	TransformerUnknown TransformerID = iota
	TransformerISODateToEpochMs
	TransformerDateToEpochMs
	TransformerCustomDateToEpochMs
	TransformerToLower
	TransformerIPv4ToInt
	TransformerSemVerToInt
	TransformerRegexExtract
	TransformerRegexExtractInt
	TransformerDurationToMs
	TransformerEmailDomain
	TransformerURLHost
	TransformerNumericBucket
	TransformerBoolNormalize
)

var transformerNames = map[TransformerID]string{
	TransformerUnknown:             "unknown",
	TransformerISODateToEpochMs:    "iso_date_to_epoch_ms",
	TransformerDateToEpochMs:       "date_to_epoch_ms",
	TransformerCustomDateToEpochMs: "custom_date_to_epoch_ms",
	TransformerToLower:             "to_lower",
	TransformerIPv4ToInt:           "ipv4_to_int",
	TransformerSemVerToInt:         "semver_to_int",
	TransformerRegexExtract:        "regex_extract",
	TransformerRegexExtractInt:     "regex_extract_int",
	TransformerDurationToMs:        "duration_to_ms",
	TransformerEmailDomain:         "email_domain",
	TransformerURLHost:             "url_host",
	TransformerNumericBucket:       "numeric_bucket",
	TransformerBoolNormalize:       "bool_normalize",
}

type TransformerSpec struct {
	Path        string            `json:"path"`
	Alias       string            `json:"alias,omitempty"`
	TargetPath  string            `json:"target_path,omitempty"`
	FailureMode IngestFailureMode `json:"failure_mode,omitempty"`
	ID          TransformerID     `json:"id"`
	Name        string            `json:"name"`
	Params      json.RawMessage   `json:"params,omitempty"`
}

type CustomDateParams struct {
	Layout string `json:"layout"`
}

type RegexParams struct {
	Pattern string `json:"pattern"`
	Group   int    `json:"group"`
}

type NumericBucketParams struct {
	Size float64 `json:"size"`
}

// regexCompileFunc and regexCompileDeadline are test hooks. Production code
// never changes them: every regex compile has a 100 ms timeout.
var (
	regexCompileFunc     = regexp.Compile
	regexCompileDeadline = 100 * time.Millisecond
)

// regexCompileError tells a regex compile failure from a compile timeout
// without reading the message text.
type regexCompileError struct {
	timeout bool
	cause   error
}

func (e *regexCompileError) Error() string { return e.cause.Error() }

func (e *regexCompileError) Cause() error { return e.cause }

func (e *regexCompileError) Unwrap() error { return e.cause }

// compileRegexWithTimeout compiles pattern in a goroutine. The channel is
// buffered, so a compile that ends after the timeout is dropped and never
// returned. Both failures return a *regexCompileError.
func compileRegexWithTimeout(pattern string, timeout time.Duration) (*regexp.Regexp, error) {
	type result struct {
		re  *regexp.Regexp
		err error
	}
	compile := regexCompileFunc
	ch := make(chan result, 1)
	go func() {
		re, err := compile(pattern)
		ch <- result{re, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, &regexCompileError{cause: r.err}
		}
		return r.re, nil
	case <-timer.C:
		return nil, &regexCompileError{timeout: true, cause: errors.New("regex compile timeout (possible ReDoS pattern)")}
	}
}

func ReconstructTransformer(id TransformerID, params json.RawMessage) (FieldTransformer, error) {
	switch id {
	case TransformerISODateToEpochMs:
		return ISODateToEpochMs, nil
	case TransformerDateToEpochMs:
		return DateToEpochMs, nil
	case TransformerCustomDateToEpochMs:
		var p CustomDateParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, errors.Wrap(err, "unmarshal custom date params")
		}
		if p.Layout == "" {
			return nil, errors.New("custom date layout required")
		}
		return CustomDateToEpochMs(p.Layout), nil
	case TransformerToLower:
		return ToLower, nil
	case TransformerIPv4ToInt:
		return IPv4ToInt, nil
	case TransformerSemVerToInt:
		return SemVerToInt, nil
	case TransformerRegexExtract, TransformerRegexExtractInt:
		var p RegexParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, errors.Wrap(err, "unmarshal regex params")
		}
		if p.Group < 0 {
			return nil, errors.New("regex group must be non-negative")
		}
		re, err := compileRegexWithTimeout(p.Pattern, regexCompileDeadline)
		if err != nil {
			return nil, errors.Wrap(err, "compile regex pattern")
		}
		return regexExtractFunc(re, p.Group, id == TransformerRegexExtractInt), nil
	case TransformerDurationToMs:
		return DurationToMs, nil
	case TransformerEmailDomain:
		return EmailDomain, nil
	case TransformerURLHost:
		return URLHost, nil
	case TransformerNumericBucket:
		var p NumericBucketParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, errors.Wrap(err, "unmarshal numeric bucket params")
		}
		if p.Size <= 0 {
			return nil, errors.New("numeric bucket size must be positive")
		}
		return NumericBucket(p.Size), nil
	case TransformerBoolNormalize:
		return BoolNormalize, nil
	default:
		return nil, errors.Errorf("unknown transformer ID: %d", id)
	}
}

// regexExtractFunc returns the RegexExtract (numeric=false) or
// RegexExtractInt (numeric=true) function for a compiled pattern.
func regexExtractFunc(re *regexp.Regexp, group int, numeric bool) FieldTransformer {
	return func(v any) (any, bool) {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		matches := re.FindStringSubmatch(s)
		if len(matches) <= group {
			return nil, false
		}
		if !numeric {
			return matches[group], true
		}
		n, err := parseFloat(matches[group])
		if err != nil {
			return nil, false
		}
		return n, true
	}
}

// parseFloat accepts plain decimal numbers only: an optional leading '-',
// digits, and at most one '.'. Scientific notation ("1.5e3"), hex, and
// Inf/NaN literals are rejected. Used by the RegexExtractInt closure
// reconstructed from a serialized index; captures that do not match this
// shape cause the document to be skipped (the closure returns ok=false).
func parseFloat(s string) (float64, error) {
	var n float64
	_, err := parseFloatInto(s, &n)
	return n, err
}

func parseFloatInto(s string, n *float64) (bool, error) {
	f, err := parseFloatSimple(s)
	if err != nil {
		return false, err
	}
	*n = f
	return true, nil
}

func parseFloatSimple(s string) (float64, error) {
	var negative bool
	if len(s) > 0 && s[0] == '-' {
		negative = true
		s = s[1:]
	}
	var intPart, fracPart float64
	var fracDiv float64 = 1
	digits := 0
	seenDot := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if seenDot {
				return 0, errors.New("invalid number")
			}
			seenDot = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, errors.New("invalid number")
		}
		digits++
		if seenDot {
			fracPart = fracPart*10 + float64(c-'0')
			fracDiv *= 10
		} else {
			intPart = intPart*10 + float64(c-'0')
		}
	}
	if digits == 0 {
		return 0, errors.New("invalid number")
	}
	result := intPart + fracPart/fracDiv
	if negative {
		result = -result
	}
	return result, nil
}

func NewTransformerSpec(path string, id TransformerID, params json.RawMessage) TransformerSpec {
	return TransformerSpec{
		Path:   path,
		ID:     id,
		Name:   transformerNames[id],
		Params: params,
	}
}

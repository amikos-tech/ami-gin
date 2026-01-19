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
	Path   string          `json:"path"`
	ID     TransformerID   `json:"id"`
	Name   string          `json:"name"`
	Params json.RawMessage `json:"params,omitempty"`
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

const regexCompileTimeout = 100 * time.Millisecond

func compileRegexWithTimeout(pattern string, timeout time.Duration) (*regexp.Regexp, error) {
	type result struct {
		re  *regexp.Regexp
		err error
	}
	ch := make(chan result, 1)
	go func() {
		re, err := regexp.Compile(pattern)
		ch <- result{re, err}
	}()
	select {
	case r := <-ch:
		return r.re, r.err
	case <-time.After(timeout):
		return nil, errors.New("regex compile timeout (possible ReDoS pattern)")
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
	case TransformerRegexExtract:
		var p RegexParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, errors.Wrap(err, "unmarshal regex params")
		}
		re, err := compileRegexWithTimeout(p.Pattern, regexCompileTimeout)
		if err != nil {
			return nil, errors.Wrap(err, "compile regex pattern")
		}
		return func(v any) (any, bool) {
			s, ok := v.(string)
			if !ok {
				return nil, false
			}
			matches := re.FindStringSubmatch(s)
			if len(matches) <= p.Group {
				return nil, false
			}
			return matches[p.Group], true
		}, nil
	case TransformerRegexExtractInt:
		var p RegexParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, errors.Wrap(err, "unmarshal regex params")
		}
		re, err := compileRegexWithTimeout(p.Pattern, regexCompileTimeout)
		if err != nil {
			return nil, errors.Wrap(err, "compile regex pattern")
		}
		return func(v any) (any, bool) {
			s, ok := v.(string)
			if !ok {
				return nil, false
			}
			matches := re.FindStringSubmatch(s)
			if len(matches) <= p.Group {
				return nil, false
			}
			n, err := parseFloat(matches[p.Group])
			if err != nil {
				return nil, false
			}
			return n, true
		}, nil
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
		if seenDot {
			fracPart = fracPart*10 + float64(c-'0')
			fracDiv *= 10
		} else {
			intPart = intPart*10 + float64(c-'0')
		}
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

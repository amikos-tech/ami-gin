package gin

import (
	"math"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// ISODateToEpochMs parses RFC3339/ISO8601 strings to Unix milliseconds.
func ISODateToEpochMs(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, false
		}
	}
	return float64(t.UnixMilli()), true
}

// DateToEpochMs parses "2006-01-02" format to Unix milliseconds (midnight UTC).
func DateToEpochMs(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, false
	}
	return float64(t.UnixMilli()), true
}

// CustomDateToEpochMs returns a transformer for custom date formats.
func CustomDateToEpochMs(layout string) FieldTransformer {
	return func(v any) (any, bool) {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		t, err := time.Parse(layout, s)
		if err != nil {
			return nil, false
		}
		return float64(t.UnixMilli()), true
	}
}

// ToLower normalizes strings to lowercase for case-insensitive queries.
func ToLower(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	return strings.ToLower(s), true
}

// IPv4ToInt converts IPv4 address strings to uint32 (as float64) for range queries.
func IPv4ToInt(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, false
	}
	return float64(uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])), true
}

// SemVerToInt encodes semantic versions as integers: major*1000000 + minor*1000 + patch.
// Supports formats: "1.2.3", "v1.2.3", "1.2", "v1.2", "1.2.3-beta" (pre-release suffix ignored).
func SemVerToInt(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 || major > 999 {
		return nil, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 || minor > 999 {
		return nil, false
	}
	patch := 0
	if len(parts) == 3 {
		patchStr := strings.Split(parts[2], "-")[0]
		patch, err = strconv.Atoi(patchStr)
		if err != nil || patch < 0 || patch > 999 {
			return nil, false
		}
	}
	return float64(major*1000000 + minor*1000 + patch), true
}

// RegexExtract returns a transformer that extracts a substring via regex capture group.
// Pattern is compiled once at config time. Group 0 = full match, group 1+ = capture groups.
func RegexExtract(pattern string, group int) FieldTransformer {
	re := regexp.MustCompile(pattern)
	return func(v any) (any, bool) {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		matches := re.FindStringSubmatch(s)
		if len(matches) <= group {
			return nil, false
		}
		return matches[group], true
	}
}

// RegexExtractInt extracts a substring via regex and converts it to float64.
func RegexExtractInt(pattern string, group int) FieldTransformer {
	re := regexp.MustCompile(pattern)
	return func(v any) (any, bool) {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		matches := re.FindStringSubmatch(s)
		if len(matches) <= group {
			return nil, false
		}
		n, err := strconv.ParseFloat(matches[group], 64)
		if err != nil {
			return nil, false
		}
		return n, true
	}
}

// DurationToMs parses Go duration strings (e.g., "1h30m", "500ms") to milliseconds.
func DurationToMs(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, false
	}
	return float64(d.Milliseconds()), true
}

// EmailDomain extracts and lowercases the domain from an email address.
func EmailDomain(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	parts := strings.Split(s, "@")
	if len(parts) != 2 || parts[1] == "" {
		return nil, false
	}
	return strings.ToLower(parts[1]), true
}

// URLHost extracts and lowercases the host from a URL.
func URLHost(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return nil, false
	}
	return strings.ToLower(u.Host), true
}

// NumericBucket returns a transformer that buckets numeric values by size.
// Example: NumericBucket(100) transforms 150 -> 100, 250 -> 200.
func NumericBucket(size float64) FieldTransformer {
	return func(v any) (any, bool) {
		f, ok := v.(float64)
		if !ok {
			return nil, false
		}
		return math.Floor(f/size) * size, true
	}
}

// BoolNormalize normalizes various boolean-like values to actual booleans.
// Handles: bool, "true"/"false"/"yes"/"no"/"1"/"0"/"on"/"off", float64 (0 = false).
func BoolNormalize(v any) (any, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		switch strings.ToLower(val) {
		case "true", "yes", "1", "on":
			return true, true
		case "false", "no", "0", "off":
			return false, true
		}
	case float64:
		return val != 0, true
	}
	return nil, false
}

// CIDRToRange parses a CIDR notation string and returns the start and end IP addresses
// as float64 values suitable for use with GTE/LTE predicates on IPv4ToInt-transformed fields.
// Example: CIDRToRange("192.168.1.0/24") returns (3232235776, 3232236031, nil)
func CIDRToRange(cidr string) (start, end float64, err error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0, 0, err
	}

	ip4 := network.IP.To4()
	if ip4 == nil {
		return 0, 0, errors.New("IPv6 not supported")
	}

	// Start IP is the network address
	startIP := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])

	// Calculate end IP from mask
	ones, bits := network.Mask.Size()
	if bits != 32 {
		return 0, 0, errors.New("IPv6 not supported")
	}

	// Host bits are the inverse of the mask
	hostBits := uint32(32 - ones)
	endIP := startIP | (1<<hostBits - 1)

	return float64(startIP), float64(endIP), nil
}

// InSubnet creates predicates to check if an IP field (transformed with IPv4ToInt)
// falls within a CIDR subnet range.
// Example: InSubnet("$.client_ip", "192.168.1.0/24") returns predicates for 192.168.1.0-255
// Panics if CIDR is invalid - use CIDRToRange for error handling.
func InSubnet(path, cidr string) []Predicate {
	start, end, err := CIDRToRange(cidr)
	if err != nil {
		panic("invalid CIDR: " + err.Error())
	}
	return []Predicate{
		{Path: path, Operator: OpGTE, Value: start},
		{Path: path, Operator: OpLTE, Value: end},
	}
}

package gin

import (
	"fmt"

	"github.com/ohler55/ojg/jp"
)

type JSONPathError struct {
	Path    string
	Message string
}

func (e *JSONPathError) Error() string {
	return fmt.Sprintf("invalid JSONPath %q: %s", e.Path, e.Message)
}

// ValidateJSONPath validates a JSONPath expression and ensures it only uses
// features supported by the GIN index (dot notation, wildcards).
// Unsupported: array indices [0], filters [?()], recursive descent .., scripts
func ValidateJSONPath(path string) error {
	if path == "" {
		return &JSONPathError{Path: path, Message: "empty path"}
	}

	if path[0] != '$' {
		return &JSONPathError{Path: path, Message: "path must start with '$'"}
	}

	expr, err := jp.ParseString(path)
	if err != nil {
		return &JSONPathError{Path: path, Message: err.Error()}
	}

	if len(expr) == 0 {
		return &JSONPathError{Path: path, Message: "empty expression"}
	}

	// First fragment must be Root ($)
	if _, ok := expr[0].(jp.Root); !ok {
		return &JSONPathError{Path: path, Message: "path must start with '$'"}
	}

	for _, frag := range expr {
		switch f := frag.(type) {
		case jp.Root:
			// $ - OK
		case jp.Child:
			// .field - OK
		case jp.Wildcard:
			// [*] - OK
		case jp.Bracket:
			// ['field'] bracket notation - OK (equivalent to .field)
		case jp.Nth:
			// [0], [1] - NOT supported
			return &JSONPathError{
				Path:    path,
				Message: fmt.Sprintf("array index [%d] not supported, use [*] for array elements", f),
			}
		case *jp.Slice:
			// [0:5], [::2] - NOT supported
			return &JSONPathError{
				Path:    path,
				Message: "slice notation not supported",
			}
		case *jp.Filter:
			// [?(@.price < 10)] - NOT supported
			return &JSONPathError{
				Path:    path,
				Message: "filter expressions not supported",
			}
		case jp.Descent:
			// .. recursive descent - NOT supported
			return &JSONPathError{
				Path:    path,
				Message: "recursive descent (..) not supported",
			}
		case jp.Union:
			// [name1, name2] - NOT supported
			return &JSONPathError{
				Path:    path,
				Message: "union notation not supported",
			}
		default:
			return &JSONPathError{
				Path:    path,
				Message: fmt.Sprintf("unsupported path fragment type: %T", frag),
			}
		}
	}

	return nil
}

func MustValidateJSONPath(path string) string {
	if err := ValidateJSONPath(path); err != nil {
		panic(err)
	}
	return path
}

func IsValidJSONPath(path string) bool {
	return ValidateJSONPath(path) == nil
}

// ParseJSONPath parses and validates a JSONPath, returning the parsed expression.
func ParseJSONPath(path string) (jp.Expr, error) {
	if err := ValidateJSONPath(path); err != nil {
		return nil, err
	}
	return jp.ParseString(path)
}

func canonicalizeSupportedPath(path string) (string, error) {
	if err := ValidateJSONPath(path); err != nil {
		return "", err
	}
	return NormalizePath(path), nil
}

// NormalizePath converts a JSONPath to a canonical dot-notation form without
// validating that the path uses only GIN-supported JSONPath features.
// Callers handling untrusted input should use ValidateJSONPath or
// canonicalizeSupportedPath first.
func NormalizePath(path string) string {
	expr, err := jp.ParseString(path)
	if err != nil {
		return path
	}
	return expr.String()
}

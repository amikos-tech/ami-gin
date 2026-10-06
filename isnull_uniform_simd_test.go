//go:build simdjson

package gin

import "testing"

func TestIsNullUniformRootPresenceSIMD(t *testing.T) {
	parser := newTestSIMDParser(t)
	assertRootPresenceAllDocShapes(t, parser)
}

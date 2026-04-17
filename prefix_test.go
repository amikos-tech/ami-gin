package gin

import (
	"math"
	"strings"
	"testing"
)

func TestNewPrefixCompressorRejectsOverflowBlockSize(t *testing.T) {
	_, err := NewPrefixCompressor(math.MaxUint16 + 1)
	if err == nil {
		t.Fatal("NewPrefixCompressor(MaxUint16+1) error = nil, want overflow rejection")
	}
	if !strings.Contains(err.Error(), "blockSize") {
		t.Fatalf("err = %v, want blockSize context", err)
	}
	if !strings.Contains(err.Error(), "65535") {
		t.Fatalf("err = %v, want reference to max 65535", err)
	}
}

func TestNewPrefixCompressorAcceptsMaxUint16(t *testing.T) {
	pc, err := NewPrefixCompressor(math.MaxUint16)
	if err != nil {
		t.Fatalf("NewPrefixCompressor(MaxUint16) error = %v, want nil", err)
	}
	if pc.BlockSize() != math.MaxUint16 {
		t.Fatalf("BlockSize() = %d, want %d", pc.BlockSize(), math.MaxUint16)
	}
}

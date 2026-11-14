package gin

import (
	"bytes"
	"encoding/binary"
	stderrors "errors"
	"math"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/pkg/errors"
)

// RFC 8878 frame builders. They let the tests craft frames that the zstd
// encoder never writes (all-ones content size, false content size, unknown
// dictionary, unknown size with huge output, and so on).

const (
	zstdFrameMagic     = 0xFD2FB528
	zstdSkippableMagic = 0x184D2A50

	// zstdTestWindowByte is a window descriptor for a 128 KiB window
	// (exponent 7: 2^(10+7)). Blocks are at most 128 KiB.
	zstdTestWindowByte = 7 << 3
	// zstdHugeWindowByte is a window descriptor for a 128 MiB window
	// (exponent 17: 2^(10+17)), above maxDecodedIndexSize.
	zstdHugeWindowByte = 17 << 3

	zstdTestBlockSize = 1<<17 - 1
)

type zstdFCSMode int

const (
	zstdFCSNone zstdFCSMode = iota
	zstdFCS1                // single segment, 1 byte
	zstdFCS2                // 2 bytes, value - 256
	zstdFCS4                // 4 bytes
	zstdFCS8                // 8 bytes
)

type zstdBlockSpec struct {
	rle  bool
	data []byte // raw block content
	size int    // rle: regenerated size
	b    byte   // rle: repeated byte
}

type zstdFrameSpec struct {
	fcsMode    zstdFCSMode
	fcs        uint64
	windowByte byte
	dictSize   int // 0, 1, 2 or 4
	dictID     uint32
	blocks     []zstdBlockSpec
}

// rawBlocks splits data into raw blocks. It always returns at least one block.
func rawBlocks(data []byte) []zstdBlockSpec {
	var out []zstdBlockSpec
	for len(data) > zstdTestBlockSize {
		out = append(out, zstdBlockSpec{data: data[:zstdTestBlockSize]})
		data = data[zstdTestBlockSize:]
	}
	return append(out, zstdBlockSpec{data: data})
}

// rleBlocks returns RLE blocks that regenerate total bytes of b. Each block
// costs 4 bytes in the frame, so a 64 MiB frame stays a couple of KiB.
func rleBlocks(total int, b byte) []zstdBlockSpec {
	var out []zstdBlockSpec
	for total > zstdTestBlockSize {
		out = append(out, zstdBlockSpec{rle: true, size: zstdTestBlockSize, b: b})
		total -= zstdTestBlockSize
	}
	return append(out, zstdBlockSpec{rle: true, size: total, b: b})
}

func buildZstdFrame(spec zstdFrameSpec) []byte {
	var desc byte
	singleSegment := false
	switch spec.fcsMode {
	case zstdFCSNone:
	case zstdFCS1:
		singleSegment = true
	case zstdFCS2:
		desc |= 1 << 6
	case zstdFCS4:
		desc |= 2 << 6
	case zstdFCS8:
		desc |= 3 << 6
	}
	if singleSegment {
		desc |= 1 << 5
	}
	switch spec.dictSize {
	case 0:
	case 1:
		desc |= 1
	case 2:
		desc |= 2
	case 4:
		desc |= 3
	default:
		panic("buildZstdFrame: bad dictSize")
	}

	out := binary.LittleEndian.AppendUint32(nil, zstdFrameMagic)
	out = append(out, desc)
	if !singleSegment {
		out = append(out, spec.windowByte)
	}
	switch spec.dictSize {
	case 1:
		out = append(out, byte(spec.dictID))
	case 2:
		out = binary.LittleEndian.AppendUint16(out, uint16(spec.dictID))
	case 4:
		out = binary.LittleEndian.AppendUint32(out, spec.dictID)
	}
	switch spec.fcsMode {
	case zstdFCS1:
		out = append(out, byte(spec.fcs))
	case zstdFCS2:
		out = binary.LittleEndian.AppendUint16(out, uint16(spec.fcs-256))
	case zstdFCS4:
		out = binary.LittleEndian.AppendUint32(out, uint32(spec.fcs))
	case zstdFCS8:
		out = binary.LittleEndian.AppendUint64(out, spec.fcs)
	}

	for i, blk := range spec.blocks {
		last := uint32(0)
		if i == len(spec.blocks)-1 {
			last = 1
		}
		if blk.rle {
			out = appendBlockHeader(out, last, 1, blk.size)
			out = append(out, blk.b)
			continue
		}
		out = appendBlockHeader(out, last, 0, len(blk.data))
		out = append(out, blk.data...)
	}
	return out
}

func appendBlockHeader(out []byte, last, typ uint32, size int) []byte {
	h := last | typ<<1 | uint32(size)<<3
	return append(out, byte(h), byte(h>>8), byte(h>>16))
}

func buildSkippableFrame(data []byte) []byte {
	out := binary.LittleEndian.AppendUint32(nil, zstdSkippableMagic)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(data)))
	return append(out, data...)
}

// knownFrame is a frame with a 4-byte content size equal to len(data).
func knownFrame(data []byte) []byte {
	return buildZstdFrame(zstdFrameSpec{
		fcsMode:    zstdFCS4,
		fcs:        uint64(len(data)),
		windowByte: zstdTestWindowByte,
		blocks:     rawBlocks(data),
	})
}

// unknownFrame is a frame without a content size.
func unknownFrame(data []byte) []byte {
	return buildZstdFrame(zstdFrameSpec{
		fcsMode:    zstdFCSNone,
		windowByte: zstdTestWindowByte,
		blocks:     rawBlocks(data),
	})
}

type zstdShape struct {
	hasFCS    bool
	fcs       uint64
	skippable bool
	dictID    uint32
}

func assertZstdShape(t *testing.T, name string, payload []byte, want zstdShape) {
	t.Helper()
	var h zstd.Header
	if err := h.Decode(payload); err != nil {
		t.Fatalf("%s: Header.Decode() error = %v", name, err)
	}
	if h.Skippable != want.skippable {
		t.Fatalf("%s: Skippable = %v, want %v", name, h.Skippable, want.skippable)
	}
	if want.skippable {
		return
	}
	if h.HasFCS != want.hasFCS {
		t.Fatalf("%s: HasFCS = %v, want %v", name, h.HasFCS, want.hasFCS)
	}
	if want.hasFCS && h.FrameContentSize != want.fcs {
		t.Fatalf("%s: FrameContentSize = %d, want %d", name, h.FrameContentSize, want.fcs)
	}
	if h.DictionaryID != want.dictID {
		t.Fatalf("%s: DictionaryID = %d, want %d", name, h.DictionaryID, want.dictID)
	}
}

// Shared fixtures.

func mustSmallIndex(t testing.TB) *GINIndex {
	t.Helper()
	builder, err := NewBuilder(DefaultConfig(), 1)
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}
	if err := builder.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}
	return builder.Finalize()
}

// smallIndexRaw returns the uncompressed index body (no magic).
func smallIndexRaw(t testing.TB) []byte {
	t.Helper()
	raw, err := EncodeWithLevel(mustSmallIndex(t), CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}
	return raw[len(uncompressedMagic):]
}

// smallIndexCompressed returns the compressed index payload (no magic).
func smallIndexCompressed(t testing.TB) []byte {
	t.Helper()
	data, err := Encode(mustSmallIndex(t))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if string(data[:4]) != compressedMagic {
		t.Fatalf("Encode() magic = %q, want %q", data[:4], compressedMagic)
	}
	return data[4:]
}

// Test-local copy of the decode path as it was before #109. It must not call
// production helpers.

func legacyDecodeZstd(payload []byte) ([]byte, error) {
	decoder, err := sharedZstdDecoder()
	if err != nil {
		return nil, errors.Wrap(err, "create zstd decoder")
	}
	out, err := decoder.DecodeAll(payload, make([]byte, 0, maxDecodedIndexSize))
	if err != nil {
		if stderrors.Is(err, zstd.ErrDecoderSizeExceeded) {
			return nil, errors.Wrap(ErrDecodedSizeExceedsLimit, "decompress data")
		}
		return nil, errors.Wrap(err, "decompress data")
	}
	return out, nil
}

// legacyDecode runs the pre-#109 decompression and then the shared index
// reader through the uncompressed magic.
func legacyDecode(payload []byte) (*GINIndex, error) {
	out, err := legacyDecodeZstd(payload)
	if err != nil {
		return nil, err
	}
	return Decode(append([]byte(uncompressedMagic), out...))
}

func legacyDecodeError(payload []byte) error {
	_, err := legacyDecode(payload)
	return err
}

var differentialSentinels = []struct {
	name string
	err  error
}{
	{"ErrDecodedSizeExceedsLimit", ErrDecodedSizeExceedsLimit},
	{"ErrInvalidFormat", ErrInvalidFormat},
	{"zstd.ErrWindowSizeExceeded", zstd.ErrWindowSizeExceeded},
	{"zstd.ErrUnknownDictionary", zstd.ErrUnknownDictionary},
	{"zstd.ErrCRCMismatch", zstd.ErrCRCMismatch},
	{"zstd.ErrDecoderSizeExceeded", zstd.ErrDecoderSizeExceeded},
}

type differentialFixture struct {
	name    string
	payload []byte
	large   bool // 64 MiB-plus decode work; skipped under -short
	// wantOK: 1 must accept, -1 must reject, 0 differential only.
	wantOK int
	// wantSentinel is checked with errors.Is when wantOK == -1.
	wantSentinel error
}

func crcCorruptPayload(t *testing.T, raw []byte) []byte {
	t.Helper()
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderCRC(true))
	if err != nil {
		t.Fatalf("zstd.NewWriter() error = %v", err)
	}
	defer enc.Close()
	payload := enc.EncodeAll(raw, nil)
	var h zstd.Header
	if err := h.Decode(payload); err != nil {
		t.Fatalf("Header.Decode() error = %v", err)
	}
	if !h.HasCheckSum {
		t.Fatal("CRC fixture: HasCheckSum = false, want true")
	}
	payload[len(payload)-1] ^= 0xFF
	return payload
}

func differentialFixtures(t *testing.T) []differentialFixture {
	t.Helper()
	raw := smallIndexRaw(t)
	real := smallIndexCompressed(t)
	half := len(raw) / 2
	zeros1k := make([]byte, 1024)

	var fixtures []differentialFixture
	add := func(f differentialFixture) { fixtures = append(fixtures, f) }

	add(differentialFixture{name: "real index", payload: real, wantOK: 1})

	twoKnownSmall := append(knownFrame(zeros1k), knownFrame(zeros1k)...)
	assertZstdShape(t, "two known 1KiB", twoKnownSmall, zstdShape{hasFCS: true, fcs: 1024})
	add(differentialFixture{name: "two known-size 1KiB frames", payload: twoKnownSmall})

	splitKnown := append(knownFrame(raw[:half]), knownFrame(raw[half:])...)
	assertZstdShape(t, "split known", splitKnown, zstdShape{hasFCS: true, fcs: uint64(half)})
	add(differentialFixture{name: "valid index split over two known-size frames", payload: splitKnown, wantOK: 1})

	knownUnknown := append(knownFrame(raw[:half]), unknownFrame(raw[half:])...)
	assertZstdShape(t, "known+unknown", knownUnknown, zstdShape{hasFCS: true, fcs: uint64(half)})
	add(differentialFixture{name: "known frame then unknown-size frame", payload: knownUnknown, wantOK: 1})

	skipKnown := append(buildSkippableFrame([]byte("meta")), knownFrame(raw)...)
	assertZstdShape(t, "skippable+known", skipKnown, zstdShape{skippable: true})
	add(differentialFixture{name: "skippable then known-size frame", payload: skipKnown, wantOK: 1})

	allOnes := buildZstdFrame(zstdFrameSpec{
		fcsMode: zstdFCS8, fcs: math.MaxUint64, windowByte: zstdTestWindowByte, blocks: rawBlocks(raw),
	})
	assertZstdShape(t, "all-ones", allOnes, zstdShape{hasFCS: true, fcs: math.MaxUint64})
	add(differentialFixture{name: "all-ones content size around valid index", payload: allOnes})

	fcsHugeWindow := buildZstdFrame(zstdFrameSpec{
		fcsMode: zstdFCS8, fcs: maxDecodedIndexSize + 1, windowByte: zstdHugeWindowByte, blocks: rawBlocks(raw),
	})
	assertZstdShape(t, "fcs>limit window>limit", fcsHugeWindow, zstdShape{hasFCS: true, fcs: maxDecodedIndexSize + 1})
	add(differentialFixture{name: "content size above limit with window above limit", payload: fcsHugeWindow, wantOK: -1})

	fcsUnknownDict := buildZstdFrame(zstdFrameSpec{
		fcsMode: zstdFCS8, fcs: 2 * maxDecodedIndexSize, windowByte: zstdTestWindowByte,
		dictSize: 4, dictID: 0xDEADBEEF, blocks: rawBlocks(raw),
	})
	assertZstdShape(t, "fcs>limit dict", fcsUnknownDict,
		zstdShape{hasFCS: true, fcs: 2 * maxDecodedIndexSize, dictID: 0xDEADBEEF})
	add(differentialFixture{
		name: "content size above limit with unknown dictionary", payload: fcsUnknownDict,
		wantOK: -1, wantSentinel: zstd.ErrUnknownDictionary,
	})

	falseSmall := buildZstdFrame(zstdFrameSpec{
		fcsMode: zstdFCS4, fcs: 16, windowByte: zstdTestWindowByte, blocks: rawBlocks(raw),
	})
	assertZstdShape(t, "false small", falseSmall, zstdShape{hasFCS: true, fcs: 16})
	add(differentialFixture{name: "false small content size", payload: falseSmall})

	falseLarge := buildZstdFrame(zstdFrameSpec{
		fcsMode: zstdFCS4, fcs: uint64(len(raw)) + 4096, windowByte: zstdTestWindowByte, blocks: rawBlocks(raw),
	})
	assertZstdShape(t, "false large", falseLarge, zstdShape{hasFCS: true, fcs: uint64(len(raw)) + 4096})
	add(differentialFixture{name: "false large content size at most limit", payload: falseLarge})

	crc := crcCorruptPayload(t, raw)
	add(differentialFixture{name: "CRC failure", payload: crc, wantOK: -1, wantSentinel: zstd.ErrCRCMismatch})

	add(differentialFixture{name: "truncated mid payload", payload: real[:len(real)/2], wantOK: -1})
	add(differentialFixture{name: "truncated inside header", payload: real[:6], wantOK: -1})

	emptyFCS0 := buildZstdFrame(zstdFrameSpec{fcsMode: zstdFCS1, fcs: 0, blocks: rawBlocks(nil)})
	assertZstdShape(t, "fcs0", emptyFCS0, zstdShape{hasFCS: true, fcs: 0})
	add(differentialFixture{
		name: "empty FCS 0 frame then valid index frame", payload: append(append([]byte{}, emptyFCS0...), knownFrame(raw)...), wantOK: 1,
	})

	add(differentialFixture{name: "header parse error garbage", payload: []byte("not a zstd frame at all"), wantOK: -1})
	add(differentialFixture{name: "empty payload", payload: []byte{}, wantOK: -1})

	known64MiB1 := buildZstdFrame(zstdFrameSpec{
		fcsMode: zstdFCS8, fcs: maxDecodedIndexSize + 1, windowByte: zstdTestWindowByte,
		blocks: rleBlocks(maxDecodedIndexSize+1, 0),
	})
	assertZstdShape(t, "known 64MiB+1", known64MiB1, zstdShape{hasFCS: true, fcs: maxDecodedIndexSize + 1})
	add(differentialFixture{
		name: "known-size oversized frame (64 MiB + 1)", payload: known64MiB1, large: true,
		wantOK: -1, wantSentinel: ErrDecodedSizeExceedsLimit,
	})

	unknown64MiB1 := buildZstdFrame(zstdFrameSpec{
		fcsMode: zstdFCSNone, windowByte: zstdTestWindowByte, blocks: rleBlocks(maxDecodedIndexSize+1, 0),
	})
	assertZstdShape(t, "unknown 64MiB+1", unknown64MiB1, zstdShape{hasFCS: false})
	add(differentialFixture{
		name: "unknown-size oversized frame (64 MiB + 1)", payload: unknown64MiB1, large: true,
		wantOK: -1, wantSentinel: ErrDecodedSizeExceedsLimit,
	})

	known64MiB := buildZstdFrame(zstdFrameSpec{
		fcsMode: zstdFCS8, fcs: maxDecodedIndexSize, windowByte: zstdTestWindowByte,
		blocks: rleBlocks(maxDecodedIndexSize, 0),
	})
	assertZstdShape(t, "known 64MiB", known64MiB, zstdShape{hasFCS: true, fcs: maxDecodedIndexSize})
	add(differentialFixture{name: "known-size frame of exactly 64 MiB", payload: known64MiB, large: true})

	return fixtures
}

func TestDecodeZstdDifferential(t *testing.T) {
	rawEnc, err := EncodeWithLevel(mustSmallIndex(t), CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	for _, fx := range differentialFixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			if fx.large && testing.Short() {
				t.Skip("64 MiB-plus fixture skipped in short mode")
			}

			wantIdx, wantErr := legacyDecode(fx.payload)
			gotIdx, gotErr := Decode(append([]byte(compressedMagic), fx.payload...))

			if (wantErr == nil) != (gotErr == nil) {
				t.Fatalf("accept mismatch: legacy err = %v, new err = %v", wantErr, gotErr)
			}
			switch {
			case fx.wantOK == 1 && gotErr != nil:
				t.Fatalf("Decode() error = %v, want accept", gotErr)
			case fx.wantOK == -1 && gotErr == nil:
				t.Fatal("Decode() error = nil, want reject")
			}

			if gotErr != nil {
				for _, s := range differentialSentinels {
					if stderrors.Is(wantErr, s.err) != stderrors.Is(gotErr, s.err) {
						t.Errorf("errors.Is(%s): legacy = %v, new = %v (legacy err %q, new err %q)",
							s.name, stderrors.Is(wantErr, s.err), stderrors.Is(gotErr, s.err), wantErr, gotErr)
					}
				}
				if wantErr.Error() != gotErr.Error() {
					t.Errorf("error text: legacy = %q, new = %q", wantErr, gotErr)
				}
				if classifySerializeError(wantErr) != classifySerializeError(gotErr) {
					t.Errorf("classify: legacy = %q, new = %q", classifySerializeError(wantErr), classifySerializeError(gotErr))
				}
				if fx.wantSentinel != nil && !stderrors.Is(gotErr, fx.wantSentinel) {
					t.Errorf("Decode() error = %v, want errors.Is %v", gotErr, fx.wantSentinel)
				}
				t.Logf("rejected: %v", gotErr)
				return
			}

			gotEnc, err := EncodeWithLevel(gotIdx, CompressionNone)
			if err != nil {
				t.Fatalf("re-encode new index: %v", err)
			}
			wantEnc, err := EncodeWithLevel(wantIdx, CompressionNone)
			if err != nil {
				t.Fatalf("re-encode legacy index: %v", err)
			}
			if !bytes.Equal(gotEnc, wantEnc) {
				t.Error("new and legacy decoded indexes re-encode differently")
			}
			if fx.wantOK == 1 && !bytes.Equal(gotEnc, rawEnc) {
				t.Error("decoded index does not re-encode to the original encoding")
			}
			t.Log("accepted")
		})
	}
}

func TestZstdFirstAttemptCap(t *testing.T) {
	real := smallIndexCompressed(t)
	var realHdr zstd.Header
	if err := realHdr.Decode(real); err != nil || !realHdr.HasFCS {
		t.Fatalf("real index header: err = %v, HasFCS = %v", err, realHdr.HasFCS)
	}
	headerOnly := func(mode zstdFCSMode, fcs uint64) []byte {
		return buildZstdFrame(zstdFrameSpec{
			fcsMode: mode, fcs: fcs, windowByte: zstdTestWindowByte, blocks: rawBlocks(nil),
		})
	}

	tests := []struct {
		name      string
		payload   []byte
		wantN     int
		wantSized bool
	}{
		{"real index", real, int(realHdr.FrameContentSize), true},
		{"fcs exactly limit", headerOnly(zstdFCS8, maxDecodedIndexSize), maxDecodedIndexSize, true},
		{"fcs above limit", headerOnly(zstdFCS8, maxDecodedIndexSize+1), 0, false},
		{"all-ones fcs", headerOnly(zstdFCS8, math.MaxUint64), maxDecodedIndexSize, false},
		{"no fcs", headerOnly(zstdFCSNone, 0), maxDecodedIndexSize, false},
		{"skippable first", append(buildSkippableFrame([]byte("x")), real...), maxDecodedIndexSize, false},
		{"garbage", []byte("not a zstd frame at all"), maxDecodedIndexSize, false},
		{"empty", nil, maxDecodedIndexSize, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, sized := zstdFirstAttemptCap(tc.payload)
			if n != tc.wantN || sized != tc.wantSized {
				t.Fatalf("zstdFirstAttemptCap() = (%d, %v), want (%d, %v)", n, sized, tc.wantN, tc.wantSized)
			}
		})
	}
}

func TestDecompressIndexPayloadFirstAttemptCapacity(t *testing.T) {
	raw := smallIndexRaw(t)
	payload := smallIndexCompressed(t)
	var h zstd.Header
	if err := h.Decode(payload); err != nil || !h.HasFCS {
		t.Fatalf("header: err = %v, HasFCS = %v", err, h.HasFCS)
	}

	decoder, err := sharedZstdDecoder()
	if err != nil {
		t.Fatalf("sharedZstdDecoder() error = %v", err)
	}
	out, err := decompressIndexPayload(decoder, payload)
	if err != nil {
		t.Fatalf("decompressIndexPayload() error = %v", err)
	}
	if cap(out) != int(h.FrameContentSize) {
		t.Fatalf("cap(out) = %d, want frame content size %d", cap(out), h.FrameContentSize)
	}
	if !bytes.Equal(out, raw) {
		t.Fatal("decompressed bytes differ from the raw index")
	}
}

func TestDecodeCompressedSmallIndexAllocation(t *testing.T) {
	raw := smallIndexRaw(t)
	if len(raw) < 16<<10 || len(raw) > 64<<10 {
		t.Fatalf("fixture raw size = %d, want between 16 KiB and 64 KiB", len(raw))
	}
	data, err := Encode(mustSmallIndex(t))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if _, err := Decode(data); err != nil { // warm the shared decoder
		t.Fatalf("Decode() error = %v", err)
	}

	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Decode(data); err != nil {
				b.Fatal(err)
			}
		}
	})
	if res.N == 0 {
		t.Fatal("benchmark did not run")
	}
	t.Logf("Decode of %d byte index: %d B/op, %d allocs/op", len(raw), res.AllocedBytesPerOp(), res.AllocsPerOp())
	if got := res.AllocedBytesPerOp(); got >= 1<<20 {
		t.Fatalf("Decode allocates %d B/op, want under 1 MiB", got)
	}
}

func TestDecodeZstdConcurrentMixed(t *testing.T) {
	raw := smallIndexRaw(t)
	real := smallIndexCompressed(t)
	half := len(raw) / 2

	type input struct {
		name string
		data []byte
		ok   bool
		err  string
	}
	payloads := []struct {
		name    string
		payload []byte
	}{
		{"valid", real},
		{"retry split frames", append(knownFrame(raw[:half]), knownFrame(raw[half:])...)},
		{"crc corrupt", crcCorruptPayload(t, raw)},
		{"unknown dictionary", buildZstdFrame(zstdFrameSpec{
			fcsMode: zstdFCS8, fcs: 2 * maxDecodedIndexSize, windowByte: zstdTestWindowByte,
			dictSize: 4, dictID: 0xDEADBEEF, blocks: rawBlocks(raw),
		})},
		{"false small fcs", buildZstdFrame(zstdFrameSpec{
			fcsMode: zstdFCS4, fcs: 16, windowByte: zstdTestWindowByte, blocks: rawBlocks(raw),
		})},
	}

	inputs := make([]input, 0, len(payloads))
	for _, p := range payloads {
		in := input{name: p.name, data: append([]byte(compressedMagic), p.payload...)}
		if err := legacyDecodeError(p.payload); err != nil {
			in.err = err.Error()
		} else {
			in.ok = true
		}
		inputs = append(inputs, in)
	}
	if !inputs[0].ok || !inputs[1].ok {
		t.Fatal("valid and split-frame inputs must be accepted by the reference path")
	}
	if inputs[2].ok || inputs[3].ok {
		t.Fatal("CRC and dictionary inputs must be rejected by the reference path")
	}

	const goroutines, iterations = 8, 50
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				for _, in := range inputs {
					_, err := Decode(in.data)
					switch {
					case in.ok && err != nil:
						t.Errorf("%s: Decode() error = %v, want success", in.name, err)
					case !in.ok && err == nil:
						t.Errorf("%s: Decode() error = nil, want %q", in.name, in.err)
					case !in.ok && err.Error() != in.err:
						t.Errorf("%s: Decode() error = %q, want %q", in.name, err, in.err)
					}
				}
			}
		}()
	}
	wg.Wait()
}

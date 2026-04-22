package gin

import (
	"context"
	stderrors "errors"
	"os"
	"testing"

	pkgerrors "github.com/pkg/errors"
)

func TestClassifySerializeError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "canceled", err: context.Canceled, want: "other"},
		{name: "deadline", err: context.DeadlineExceeded, want: "other"},
		{name: "invalid format", err: pkgerrors.Wrap(ErrInvalidFormat, "decode"), want: "invalid_format"},
		{name: "version mismatch", err: pkgerrors.Wrap(ErrVersionMismatch, "decode"), want: "deserialization"},
		{name: "integrity", err: pkgerrors.Wrap(ErrDecodedSizeExceedsLimit, "decode"), want: "integrity"},
		{name: "other", err: stderrors.New("boom"), want: "other"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifySerializeError(tc.err); got != tc.want {
				t.Fatalf("classifySerializeError(%v) = %q; want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestClassifyParquetError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "canceled", err: context.Canceled, want: "other"},
		{name: "deadline", err: context.DeadlineExceeded, want: "other"},
		{name: "not found sentinel", err: pkgerrors.Wrap(os.ErrNotExist, "open parquet"), want: "io"},
		{name: "missing column", err: stderrors.New(`column "json" not found in parquet file`), want: "config"},
		{name: "open failure", err: stderrors.New("open parquet file: permission denied"), want: "io"},
		{name: "builder failure", err: stderrors.New("create builder: bad config"), want: "config"},
		{name: "other", err: stderrors.New("unexpected"), want: "other"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyParquetError(tc.err); got != tc.want {
				t.Fatalf("classifyParquetError(%v) = %q; want %q", tc.err, got, tc.want)
			}
		})
	}
}

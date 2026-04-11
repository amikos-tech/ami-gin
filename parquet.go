package gin

import (
	"bytes"
	"encoding/base64"
	stderrors "errors"
	"io"
	"os"
	"strings"

	"github.com/parquet-go/parquet-go"
	"github.com/pkg/errors"
)

const DefaultMetadataKey = "gin.index"

type ParquetConfig struct {
	MetadataKey string
}

func DefaultParquetConfig() ParquetConfig {
	return ParquetConfig{
		MetadataKey: DefaultMetadataKey,
	}
}

func SidecarPath(parquetFile string) string {
	return parquetFile + ".gin"
}

// Keep in sync with cmd/gin-index/main.go:artifactFileMode.
func artifactFileMode(mode os.FileMode) os.FileMode {
	return mode.Perm() & 0o666
}

func parquetFileMode(parquetFile string) (os.FileMode, error) {
	info, err := os.Stat(parquetFile)
	if err != nil {
		return 0, errors.Wrap(err, "stat parquet file")
	}

	return artifactFileMode(info.Mode()), nil
}

func writeFileWithMode(path string, data []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, data, mode); err != nil {
		return errors.Wrap(err, "write file with mode")
	}
	if err := os.Chmod(path, mode); err != nil {
		return errors.Wrap(err, "chmod file with mode")
	}

	return nil
}

func WriteSidecar(parquetFile string, idx *GINIndex) error {
	data, err := Encode(idx)
	if err != nil {
		return errors.Wrap(err, "encode index")
	}
	mode, err := parquetFileMode(parquetFile)
	if err != nil {
		return err
	}
	sidecar := SidecarPath(parquetFile)
	if err := writeFileWithMode(sidecar, data, mode); err != nil {
		return errors.Wrap(err, "write sidecar")
	}

	return nil
}

func ReadSidecar(parquetFile string) (*GINIndex, error) {
	sidecar := SidecarPath(parquetFile)
	// #nosec G304 -- the sidecar path is deterministically derived from the caller-selected parquet path.
	data, err := os.ReadFile(sidecar)
	if err != nil {
		return nil, errors.Wrap(err, "read sidecar")
	}
	return Decode(data)
}

func HasSidecar(parquetFile string) bool {
	sidecar := SidecarPath(parquetFile)
	_, err := os.Stat(sidecar)
	return err == nil
}

func openParquetFile(path string) (*parquet.File, *os.File, error) {
	// #nosec G304 -- parquet files are intentionally opened from caller-selected paths.
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "open file")
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, errors.Wrap(err, "stat file")
	}
	pf, err := parquet.OpenFile(f, stat.Size())
	if err != nil {
		_ = f.Close()
		return nil, nil, errors.Wrap(err, "open parquet")
	}
	return pf, f, nil
}

func BuildFromParquet(parquetFile string, jsonColumn string, config GINConfig) (*GINIndex, error) {
	return BuildFromParquetReader(parquetFile, jsonColumn, config, nil, 0)
}

func BuildFromParquetReader(parquetFile string, jsonColumn string, config GINConfig, reader io.ReaderAt, size int64) (*GINIndex, error) {
	var pf *parquet.File
	var fileToClose *os.File
	var err error

	if reader != nil {
		pf, err = parquet.OpenFile(reader, size)
		if err != nil {
			return nil, errors.Wrap(err, "open parquet from reader")
		}
	} else {
		pf, fileToClose, err = openParquetFile(parquetFile)
		if err != nil {
			return nil, err
		}
		defer func() { _ = fileToClose.Close() }()
	}

	colIdx := -1
	schema := pf.Schema()
	for i, field := range schema.Fields() {
		if field.Name() == jsonColumn {
			colIdx = i
			break
		}
	}
	if colIdx < 0 {
		return nil, errors.Errorf("column %q not found in parquet file", jsonColumn)
	}

	numRowGroups := len(pf.RowGroups())
	builder, err := NewBuilder(config, numRowGroups)
	if err != nil {
		return nil, errors.Wrap(err, "create builder")
	}

	for rgID, rg := range pf.RowGroups() {
		chunk := rg.ColumnChunks()[colIdx]
		pages := chunk.Pages()

		for {
			page, err := pages.ReadPage()
			if stderrors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				_ = pages.Close()
				return nil, errors.Wrapf(err, "read page in row group %d", rgID)
			}

			numValues := page.NumValues()
			values := page.Values()
			data := make([]parquet.Value, numValues)
			n, err := values.ReadValues(data)
			if err != nil && !stderrors.Is(err, io.EOF) {
				_ = pages.Close()
				return nil, errors.Wrapf(err, "read values in row group %d", rgID)
			}
			data = data[:n]

			for _, val := range data {
				if val.IsNull() {
					continue
				}
				jsonBytes := val.ByteArray()
				if err := builder.AddDocument(DocID(rgID), jsonBytes); err != nil {
					_ = pages.Close()
					return nil, errors.Wrapf(err, "add document in row group %d", rgID)
				}
			}
		}
		_ = pages.Close()
	}

	return builder.Finalize(), nil
}

func EncodeToMetadata(idx *GINIndex, cfg ParquetConfig) (key string, value string, err error) {
	data, err := Encode(idx)
	if err != nil {
		return "", "", errors.Wrap(err, "encode index")
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	if cfg.MetadataKey == "" {
		cfg.MetadataKey = DefaultMetadataKey
	}
	return cfg.MetadataKey, encoded, nil
}

func DecodeFromMetadata(value string) (*GINIndex, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, errors.Wrap(err, "decode base64")
	}
	return Decode(data)
}

func ReadFromParquetMetadata(parquetFile string, cfg ParquetConfig) (*GINIndex, error) {
	return ReadFromParquetMetadataReader(parquetFile, cfg, nil, 0)
}

func ReadFromParquetMetadataReader(parquetFile string, cfg ParquetConfig, reader io.ReaderAt, size int64) (*GINIndex, error) {
	var pf *parquet.File
	var fileToClose *os.File
	var err error

	if reader != nil {
		pf, err = parquet.OpenFile(reader, size)
		if err != nil {
			return nil, errors.Wrap(err, "open parquet from reader")
		}
	} else {
		pf, fileToClose, err = openParquetFile(parquetFile)
		if err != nil {
			return nil, err
		}
		defer func() { _ = fileToClose.Close() }()
	}

	if cfg.MetadataKey == "" {
		cfg.MetadataKey = DefaultMetadataKey
	}

	metadata := pf.Metadata()
	for _, kv := range metadata.KeyValueMetadata {
		if kv.Key == cfg.MetadataKey {
			return DecodeFromMetadata(kv.Value)
		}
	}

	return nil, errors.Errorf("metadata key %q not found", cfg.MetadataKey)
}

func HasGINIndex(parquetFile string, cfg ParquetConfig) (bool, error) {
	return HasGINIndexReader(parquetFile, cfg, nil, 0)
}

func HasGINIndexReader(parquetFile string, cfg ParquetConfig, reader io.ReaderAt, size int64) (bool, error) {
	var pf *parquet.File
	var fileToClose *os.File
	var err error

	if reader != nil {
		pf, err = parquet.OpenFile(reader, size)
		if err != nil {
			return false, errors.Wrap(err, "open parquet from reader")
		}
	} else {
		pf, fileToClose, err = openParquetFile(parquetFile)
		if err != nil {
			return false, err
		}
		defer func() { _ = fileToClose.Close() }()
	}

	if cfg.MetadataKey == "" {
		cfg.MetadataKey = DefaultMetadataKey
	}

	metadata := pf.Metadata()
	for _, kv := range metadata.KeyValueMetadata {
		if kv.Key == cfg.MetadataKey {
			return true, nil
		}
	}

	return false, nil
}

func RebuildWithIndex(parquetFile string, idx *GINIndex, cfg ParquetConfig) error {
	pf, srcFile, err := openParquetFile(parquetFile)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	key, value, err := EncodeToMetadata(idx, cfg)
	if err != nil {
		return errors.Wrap(err, "encode metadata")
	}

	schema := pf.Schema()
	var rows []parquet.Row
	for _, rg := range pf.RowGroups() {
		reader := parquet.NewRowGroupReader(rg)
		for {
			row := make([]parquet.Value, len(schema.Fields()))
			_, err := reader.ReadRows([]parquet.Row{row})
			if stderrors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return errors.Wrap(err, "read rows")
			}
			rows = append(rows, row)
		}
	}

	_ = srcFile.Close()

	tmpFile := parquetFile + ".tmp"
	mode, err := parquetFileMode(parquetFile)
	if err != nil {
		return errors.Wrap(err, "resolve file mode")
	}
	// Remove any crash-surviving temp file so the recreated inode gets the current mode.
	if err := os.Remove(tmpFile); err != nil && !stderrors.Is(err, os.ErrNotExist) {
		return errors.Wrap(err, "remove temp file")
	}
	// #nosec G304 -- the temporary file path is derived from the caller-selected parquet path.
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return errors.Wrap(err, "create temp file")
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmpFile)
	}()

	writer := parquet.NewGenericWriter[parquet.Row](f,
		schema,
		parquet.KeyValueMetadata(key, value),
	)

	if _, err := writer.WriteRows(rows); err != nil {
		return errors.Wrap(err, "write rows")
	}

	if err := writer.Close(); err != nil {
		return errors.Wrap(err, "close writer")
	}
	if err := f.Close(); err != nil {
		return errors.Wrap(err, "close file")
	}

	if err := os.Rename(tmpFile, parquetFile); err != nil {
		return errors.Wrap(err, "rename temp file")
	}

	return nil
}

func LoadIndex(parquetFile string, cfg ParquetConfig) (*GINIndex, error) {
	return LoadIndexReader(parquetFile, cfg, nil, 0)
}

func LoadIndexReader(parquetFile string, cfg ParquetConfig, reader io.ReaderAt, size int64) (*GINIndex, error) {
	if reader != nil {
		idx, err := ReadFromParquetMetadataReader(parquetFile, cfg, reader, size)
		if err == nil {
			return idx, nil
		}
	} else {
		idx, err := ReadFromParquetMetadata(parquetFile, cfg)
		if err == nil {
			return idx, nil
		}
		if HasSidecar(parquetFile) {
			return ReadSidecar(parquetFile)
		}
	}

	return nil, errors.New("no GIN index found (checked embedded metadata and sidecar)")
}

type ParquetIndexWriter struct {
	schema    *parquet.Schema
	buffer    *bytes.Buffer
	builder   *GINBuilder
	jsonCol   int
	rowGroup  int
	rowCount  int
	rowsPerRG int
	ginConfig GINConfig
	pqConfig  ParquetConfig
}

func NewParquetIndexWriter(w io.Writer, schema *parquet.Schema, jsonColumn string, numRowGroups int, ginConfig GINConfig, pqConfig ParquetConfig) (*ParquetIndexWriter, error) {
	colIdx := -1
	for i, field := range schema.Fields() {
		if field.Name() == jsonColumn {
			colIdx = i
			break
		}
	}
	if colIdx < 0 {
		return nil, errors.Errorf("column %q not found in schema", jsonColumn)
	}

	buf := &bytes.Buffer{}

	builder, err := NewBuilder(ginConfig, numRowGroups)
	if err != nil {
		return nil, errors.Wrap(err, "create builder")
	}

	return &ParquetIndexWriter{
		schema:    schema,
		buffer:    buf,
		builder:   builder,
		jsonCol:   colIdx,
		rowGroup:  0,
		rowCount:  0,
		rowsPerRG: 0,
		ginConfig: ginConfig,
		pqConfig:  pqConfig,
	}, nil
}

func IsS3Path(path string) bool {
	return strings.HasPrefix(path, "s3://")
}

func ParseS3Path(path string) (bucket, key string, err error) {
	if !IsS3Path(path) {
		return "", "", errors.Errorf("not an S3 path: %s", path)
	}
	trimmed := strings.TrimPrefix(path, "s3://")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) < 2 {
		return parts[0], "", nil
	}
	return parts[0], parts[1], nil
}

func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func ListParquetFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrap(err, "read directory")
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".parquet") {
			files = append(files, dir+"/"+entry.Name())
		}
	}
	return files, nil
}

func ListGINFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrap(err, "read directory")
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".gin") {
			files = append(files, dir+"/"+entry.Name())
		}
	}
	return files, nil
}

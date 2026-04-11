package gin

import (
	"os"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"
)

type testRecord struct {
	ID         int64  `parquet:"id"`
	Attributes string `parquet:"attributes"`
}

func createTestParquetFile(t *testing.T, path string, records []testRecord, rowsPerRG int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()

	writer := parquet.NewGenericWriter[testRecord](f,
		parquet.MaxRowsPerRowGroup(rowsPerRG),
	)

	for _, r := range records {
		if _, err := writer.Write([]testRecord{r}); err != nil {
			t.Fatalf("write record: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
}

func TestSidecarPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"data.parquet", "data.parquet.gin"},
		{"/path/to/file.parquet", "/path/to/file.parquet.gin"},
		{"s3://bucket/key.parquet", "s3://bucket/key.parquet.gin"},
	}

	for _, tt := range tests {
		got := SidecarPath(tt.input)
		if got != tt.expected {
			t.Errorf("SidecarPath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSidecarReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	parquetFile := tmpDir + "/data.parquet"

	records := []testRecord{
		{ID: 1, Attributes: `{"status": "ok", "count": 10}`},
		{ID: 2, Attributes: `{"status": "error", "count": 0}`},
		{ID: 3, Attributes: `{"status": "ok", "count": 5}`},
		{ID: 4, Attributes: `{"status": "warn", "count": 3}`},
	}
	createTestParquetFile(t, parquetFile, records, 2)

	idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
	if err != nil {
		t.Fatalf("BuildFromParquet: %v", err)
	}

	if err := WriteSidecar(parquetFile, idx); err != nil {
		t.Fatalf("WriteSidecar: %v", err)
	}

	if !HasSidecar(parquetFile) {
		t.Fatal("HasSidecar returned false")
	}

	loaded, err := ReadSidecar(parquetFile)
	if err != nil {
		t.Fatalf("ReadSidecar: %v", err)
	}

	if loaded.Header.NumRowGroups != idx.Header.NumRowGroups {
		t.Errorf("NumRowGroups = %d, want %d", loaded.Header.NumRowGroups, idx.Header.NumRowGroups)
	}
	if loaded.Header.NumPaths != idx.Header.NumPaths {
		t.Errorf("NumPaths = %d, want %d", loaded.Header.NumPaths, idx.Header.NumPaths)
	}
}

func TestWriteSidecarPreservesParquetPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceMode os.FileMode
		wantMode   os.FileMode
	}{
		{name: "preserve rw bits", sourceMode: 0o640, wantMode: 0o640},
		{name: "drop execute bits", sourceMode: 0o755, wantMode: 0o644},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			parquetFile := tmpDir + "/mode-data.parquet"

			records := []testRecord{
				{ID: 1, Attributes: `{"status": "ok"}`},
				{ID: 2, Attributes: `{"status": "warn"}`},
			}
			createTestParquetFile(t, parquetFile, records, 1)

			if err := os.Chmod(parquetFile, tt.sourceMode); err != nil {
				t.Fatalf("chmod parquet file: %v", err)
			}

			idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
			if err != nil {
				t.Fatalf("BuildFromParquet: %v", err)
			}

			if err := WriteSidecar(parquetFile, idx); err != nil {
				t.Fatalf("WriteSidecar: %v", err)
			}

			info, err := os.Stat(SidecarPath(parquetFile))
			if err != nil {
				t.Fatalf("stat sidecar: %v", err)
			}

			if got := info.Mode().Perm(); got != tt.wantMode {
				t.Fatalf("sidecar mode = %o, want %o", got, tt.wantMode)
			}
		})
	}
}

func TestWriteSidecarRefreshesExistingSidecarPermissions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parquetFile := tmpDir + "/mode-data.parquet"

	records := []testRecord{
		{ID: 1, Attributes: `{"status": "ok"}`},
		{ID: 2, Attributes: `{"status": "warn"}`},
	}
	createTestParquetFile(t, parquetFile, records, 1)

	if err := os.Chmod(parquetFile, 0o755); err != nil {
		t.Fatalf("chmod parquet file: %v", err)
	}

	idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
	if err != nil {
		t.Fatalf("BuildFromParquet: %v", err)
	}

	sidecar := SidecarPath(parquetFile)
	if err := os.WriteFile(sidecar, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale sidecar: %v", err)
	}
	if err := os.Chmod(sidecar, 0o600); err != nil {
		t.Fatalf("chmod stale sidecar: %v", err)
	}

	if err := WriteSidecar(parquetFile, idx); err != nil {
		t.Fatalf("WriteSidecar: %v", err)
	}

	info, err := os.Stat(sidecar)
	if err != nil {
		t.Fatalf("stat sidecar: %v", err)
	}

	if got, want := info.Mode().Perm(), os.FileMode(0o644); got != want {
		t.Fatalf("sidecar mode = %o, want %o", got, want)
	}

	loaded, err := ReadSidecar(parquetFile)
	if err != nil {
		t.Fatalf("ReadSidecar: %v", err)
	}
	if loaded.Header.NumDocs != idx.Header.NumDocs {
		t.Fatalf("sidecar docs = %d, want %d", loaded.Header.NumDocs, idx.Header.NumDocs)
	}
}

func TestArtifactFileMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   os.FileMode
		want os.FileMode
	}{
		{name: "preserve rw bits", in: 0o640, want: 0o640},
		{name: "drop execute bits", in: 0o755, want: 0o644},
		{name: "preserve group write and world read", in: 0o664, want: 0o664},
		{name: "mask world writable and execute bits", in: 0o777, want: 0o666},
		{name: "high bits do not affect rw mask", in: 0o4755, want: 0o644},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := artifactFileMode(tt.in); got != tt.want {
				t.Fatalf("artifactFileMode(%o) = %o, want %o", tt.in, got, tt.want)
			}
		})
	}
}

func TestWriteFileWithModeWrapsErrors(t *testing.T) {
	t.Parallel()

	err := writeFileWithMode(t.TempDir(), []byte("data"), 0o600)
	if err == nil {
		t.Fatal("writeFileWithMode(directory) returned nil error")
	}
	if !strings.Contains(err.Error(), "write file with mode") {
		t.Fatalf("writeFileWithMode(directory) error = %q, want write file with mode context", err)
	}
}

func TestBuildFromParquet(t *testing.T) {
	tmpDir := t.TempDir()
	parquetFile := tmpDir + "/data.parquet"

	records := []testRecord{
		{ID: 1, Attributes: `{"name": "alice", "age": 30}`},
		{ID: 2, Attributes: `{"name": "bob", "age": 25}`},
		{ID: 3, Attributes: `{"name": "carol", "age": 35}`},
		{ID: 4, Attributes: `{"name": "dave", "age": 28}`},
	}
	createTestParquetFile(t, parquetFile, records, 2)

	idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
	if err != nil {
		t.Fatalf("BuildFromParquet: %v", err)
	}

	if idx.Header.NumRowGroups != 2 {
		t.Errorf("NumRowGroups = %d, want 2", idx.Header.NumRowGroups)
	}

	result := idx.Evaluate([]Predicate{EQ("$.name", "alice")})
	if result.IsEmpty() {
		t.Error("Expected to find alice in some row group")
	}

	result = idx.Evaluate([]Predicate{GT("$.age", 30.0)})
	rgs := result.ToSlice()
	if len(rgs) == 0 {
		t.Error("Expected to find age > 30 in some row group")
	}
}

func TestBuildFromParquetColumnNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	parquetFile := tmpDir + "/data.parquet"

	records := []testRecord{{ID: 1, Attributes: `{}`}}
	createTestParquetFile(t, parquetFile, records, 1)

	_, err := BuildFromParquet(parquetFile, "nonexistent", DefaultConfig())
	if err == nil {
		t.Error("Expected error for nonexistent column")
	}
}

func TestMetadataEncoding(t *testing.T) {
	builder, err := NewBuilder(DefaultConfig(), 3)
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}
	builder.AddDocument(0, []byte(`{"a": 1}`))
	builder.AddDocument(1, []byte(`{"a": 2}`))
	builder.AddDocument(2, []byte(`{"a": 3}`))
	idx := builder.Finalize()

	cfg := DefaultParquetConfig()
	key, value, err := EncodeToMetadata(idx, cfg)
	if err != nil {
		t.Fatalf("EncodeToMetadata: %v", err)
	}

	if key != DefaultMetadataKey {
		t.Errorf("key = %q, want %q", key, DefaultMetadataKey)
	}
	if value == "" {
		t.Error("value is empty")
	}

	decoded, err := DecodeFromMetadata(value)
	if err != nil {
		t.Fatalf("DecodeFromMetadata: %v", err)
	}

	if decoded.Header.NumRowGroups != idx.Header.NumRowGroups {
		t.Errorf("NumRowGroups = %d, want %d", decoded.Header.NumRowGroups, idx.Header.NumRowGroups)
	}
}

func TestRebuildWithIndex(t *testing.T) {
	tmpDir := t.TempDir()
	parquetFile := tmpDir + "/data.parquet"

	records := []testRecord{
		{ID: 1, Attributes: `{"x": 1}`},
		{ID: 2, Attributes: `{"x": 2}`},
	}
	createTestParquetFile(t, parquetFile, records, 1)

	idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
	if err != nil {
		t.Fatalf("BuildFromParquet: %v", err)
	}

	cfg := DefaultParquetConfig()
	if err := RebuildWithIndex(parquetFile, idx, cfg); err != nil {
		t.Fatalf("RebuildWithIndex: %v", err)
	}

	hasIdx, err := HasGINIndex(parquetFile, cfg)
	if err != nil {
		t.Fatalf("HasGINIndex: %v", err)
	}
	if !hasIdx {
		t.Error("HasGINIndex returned false after embedding")
	}

	loaded, err := ReadFromParquetMetadata(parquetFile, cfg)
	if err != nil {
		t.Fatalf("ReadFromParquetMetadata: %v", err)
	}

	if loaded.Header.NumRowGroups != idx.Header.NumRowGroups {
		t.Errorf("NumRowGroups = %d, want %d", loaded.Header.NumRowGroups, idx.Header.NumRowGroups)
	}
}

func TestRebuildWithIndexPreservesParquetPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceMode os.FileMode
		wantMode   os.FileMode
	}{
		{name: "preserve rw bits", sourceMode: 0o640, wantMode: 0o640},
		{name: "drop execute bits", sourceMode: 0o755, wantMode: 0o644},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			parquetFile := tmpDir + "/mode-embedded.parquet"

			records := []testRecord{
				{ID: 1, Attributes: `{"x": 1}`},
				{ID: 2, Attributes: `{"x": 2}`},
			}
			createTestParquetFile(t, parquetFile, records, 1)

			if err := os.Chmod(parquetFile, tt.sourceMode); err != nil {
				t.Fatalf("chmod parquet file: %v", err)
			}

			idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
			if err != nil {
				t.Fatalf("BuildFromParquet: %v", err)
			}

			if err := RebuildWithIndex(parquetFile, idx, DefaultParquetConfig()); err != nil {
				t.Fatalf("RebuildWithIndex: %v", err)
			}

			info, err := os.Stat(parquetFile)
			if err != nil {
				t.Fatalf("stat rebuilt parquet: %v", err)
			}

			if got := info.Mode().Perm(); got != tt.wantMode {
				t.Fatalf("rebuilt parquet mode = %o, want %o", got, tt.wantMode)
			}
		})
	}
}

func TestRebuildWithIndexRefreshesModeFromSourceDespiteStaleTempFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parquetFile := tmpDir + "/stale-temp.parquet"

	records := []testRecord{
		{ID: 1, Attributes: `{"x": 1}`},
		{ID: 2, Attributes: `{"x": 2}`},
	}
	createTestParquetFile(t, parquetFile, records, 1)
	if err := os.Chmod(parquetFile, 0o640); err != nil {
		t.Fatalf("chmod parquet file: %v", err)
	}

	idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
	if err != nil {
		t.Fatalf("BuildFromParquet: %v", err)
	}

	tmpFile := parquetFile + ".tmp"
	if err := os.WriteFile(tmpFile, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale temp file: %v", err)
	}
	if err := os.Chmod(tmpFile, 0o600); err != nil {
		t.Fatalf("chmod stale temp file: %v", err)
	}

	if err := RebuildWithIndex(parquetFile, idx, DefaultParquetConfig()); err != nil {
		t.Fatalf("RebuildWithIndex: %v", err)
	}

	info, err := os.Stat(parquetFile)
	if err != nil {
		t.Fatalf("stat rebuilt parquet: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
		t.Fatalf("rebuilt parquet mode = %o, want %o", got, want)
	}

	loaded, err := ReadFromParquetMetadata(parquetFile, DefaultParquetConfig())
	if err != nil {
		t.Fatalf("ReadFromParquetMetadata: %v", err)
	}
	if loaded.Header.NumDocs != idx.Header.NumDocs {
		t.Fatalf("rebuilt parquet docs = %d, want %d", loaded.Header.NumDocs, idx.Header.NumDocs)
	}
}

func TestLoadIndex(t *testing.T) {
	tmpDir := t.TempDir()
	parquetFile := tmpDir + "/data.parquet"

	records := []testRecord{
		{ID: 1, Attributes: `{"y": 1}`},
		{ID: 2, Attributes: `{"y": 2}`},
	}
	createTestParquetFile(t, parquetFile, records, 1)

	idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
	if err != nil {
		t.Fatalf("BuildFromParquet: %v", err)
	}

	cfg := DefaultParquetConfig()

	if err := RebuildWithIndex(parquetFile, idx, cfg); err != nil {
		t.Fatalf("RebuildWithIndex: %v", err)
	}

	loaded, err := LoadIndex(parquetFile, cfg)
	if err != nil {
		t.Fatalf("LoadIndex (embedded): %v", err)
	}
	if loaded.Header.NumRowGroups != idx.Header.NumRowGroups {
		t.Errorf("NumRowGroups = %d, want %d", loaded.Header.NumRowGroups, idx.Header.NumRowGroups)
	}

	parquetFile2 := tmpDir + "/data2.parquet"
	createTestParquetFile(t, parquetFile2, records, 1)
	idx2, _ := BuildFromParquet(parquetFile2, "attributes", DefaultConfig())
	WriteSidecar(parquetFile2, idx2)

	loaded2, err := LoadIndex(parquetFile2, cfg)
	if err != nil {
		t.Fatalf("LoadIndex (sidecar): %v", err)
	}
	if loaded2.Header.NumRowGroups != idx2.Header.NumRowGroups {
		t.Errorf("NumRowGroups = %d, want %d", loaded2.Header.NumRowGroups, idx2.Header.NumRowGroups)
	}
}

func TestIsS3Path(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"s3://bucket/key", true},
		{"s3://bucket/path/to/file.parquet", true},
		{"/local/path/file.parquet", false},
		{"./relative/path.parquet", false},
		{"file.parquet", false},
	}

	for _, tt := range tests {
		got := IsS3Path(tt.path)
		if got != tt.expected {
			t.Errorf("IsS3Path(%q) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}

func TestParseS3Path(t *testing.T) {
	tests := []struct {
		path           string
		expectedBucket string
		expectedKey    string
		expectError    bool
	}{
		{"s3://bucket/key", "bucket", "key", false},
		{"s3://bucket/path/to/file.parquet", "bucket", "path/to/file.parquet", false},
		{"s3://bucket-name/", "bucket-name", "", false},
		{"/local/path", "", "", true},
	}

	for _, tt := range tests {
		bucket, key, err := ParseS3Path(tt.path)
		if tt.expectError {
			if err == nil {
				t.Errorf("ParseS3Path(%q) expected error", tt.path)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseS3Path(%q) error: %v", tt.path, err)
			continue
		}
		if bucket != tt.expectedBucket {
			t.Errorf("ParseS3Path(%q) bucket = %q, want %q", tt.path, bucket, tt.expectedBucket)
		}
		if key != tt.expectedKey {
			t.Errorf("ParseS3Path(%q) key = %q, want %q", tt.path, key, tt.expectedKey)
		}
	}
}

func TestDefaultParquetConfig(t *testing.T) {
	cfg := DefaultParquetConfig()
	if cfg.MetadataKey != DefaultMetadataKey {
		t.Errorf("MetadataKey = %q, want %q", cfg.MetadataKey, DefaultMetadataKey)
	}
}

func TestQueryAfterBuild(t *testing.T) {
	tmpDir := t.TempDir()
	parquetFile := tmpDir + "/query_test.parquet"

	records := []testRecord{
		{ID: 1, Attributes: `{"status": "success", "count": 10}`},
		{ID: 2, Attributes: `{"status": "error", "count": 0}`},
		{ID: 3, Attributes: `{"status": "success", "count": 5}`},
		{ID: 4, Attributes: `{"status": "error", "count": 0}`},
	}
	createTestParquetFile(t, parquetFile, records, 2)

	idx, err := BuildFromParquet(parquetFile, "attributes", DefaultConfig())
	if err != nil {
		t.Fatalf("BuildFromParquet: %v", err)
	}

	result := idx.Evaluate([]Predicate{EQ("$.status", "error")})
	if result.IsEmpty() {
		t.Error("Expected to find error status")
	}

	result = idx.Evaluate([]Predicate{EQ("$.status", "nonexistent")})
	if !result.IsEmpty() {
		t.Error("Should not find nonexistent status")
	}

	result = idx.Evaluate([]Predicate{GT("$.count", 5.0)})
	if result.IsEmpty() {
		t.Error("Expected to find count > 5")
	}

	result = idx.Evaluate([]Predicate{
		EQ("$.status", "success"),
		GT("$.count", 7.0),
	})
	if result.IsEmpty() {
		t.Error("Expected to find success with count > 7")
	}
}

func TestIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	if !IsDirectory(tmpDir) {
		t.Errorf("IsDirectory(%q) = false, want true", tmpDir)
	}

	tmpFile := tmpDir + "/test.txt"
	os.WriteFile(tmpFile, []byte("test"), 0644)
	if IsDirectory(tmpFile) {
		t.Errorf("IsDirectory(%q) = true, want false", tmpFile)
	}

	if IsDirectory("/nonexistent/path") {
		t.Error("IsDirectory for nonexistent path should be false")
	}
}

func TestListParquetFiles(t *testing.T) {
	tmpDir := t.TempDir()

	records := []testRecord{{ID: 1, Attributes: `{}`}}
	createTestParquetFile(t, tmpDir+"/a.parquet", records, 1)
	createTestParquetFile(t, tmpDir+"/b.parquet", records, 1)
	os.WriteFile(tmpDir+"/c.txt", []byte("not parquet"), 0644)
	os.WriteFile(tmpDir+"/d.gin", []byte("index"), 0644)

	files, err := ListParquetFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListParquetFiles: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("ListParquetFiles returned %d files, want 2", len(files))
	}

	for _, f := range files {
		if !strings.HasSuffix(f, ".parquet") {
			t.Errorf("File %q does not have .parquet suffix", f)
		}
	}
}

func TestListGINFiles(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(tmpDir+"/a.gin", []byte("index1"), 0644)
	os.WriteFile(tmpDir+"/b.gin", []byte("index2"), 0644)
	os.WriteFile(tmpDir+"/c.txt", []byte("not gin"), 0644)
	os.WriteFile(tmpDir+"/d.parquet", []byte("parquet"), 0644)

	files, err := ListGINFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListGINFiles: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("ListGINFiles returned %d files, want 2", len(files))
	}

	for _, f := range files {
		if !strings.HasSuffix(f, ".gin") {
			t.Errorf("File %q does not have .gin suffix", f)
		}
	}
}

func TestListParquetFilesEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := ListParquetFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListParquetFiles: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("ListParquetFiles returned %d files for empty dir, want 0", len(files))
	}
}

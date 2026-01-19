package gin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/parquet-go/parquet-go"
	"github.com/pkg/errors"
)

type S3Config struct {
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	PathStyle bool
}

func S3ConfigFromEnv() S3Config {
	cfg := S3Config{
		Endpoint:  os.Getenv("AWS_ENDPOINT_URL"),
		Region:    os.Getenv("AWS_REGION"),
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		PathStyle: os.Getenv("AWS_S3_PATH_STYLE") == "true",
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = os.Getenv("AWS_S3_ENDPOINT")
	}
	if cfg.Region == "" {
		cfg.Region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg
}

type S3Client struct {
	client *s3.Client
	cfg    S3Config
}

func NewS3Client(cfg S3Config) (*S3Client, error) {
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(cfg.Region))

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "load AWS config")
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}
	if cfg.PathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	return &S3Client{client: client, cfg: cfg}, nil
}

func NewS3ClientFromEnv() (*S3Client, error) {
	return NewS3Client(S3ConfigFromEnv())
}

type s3ReaderAt struct {
	client *S3Client
	bucket string
	key    string
	size   int64
}

func (r *s3ReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= r.size {
		return 0, io.EOF
	}
	end := off + int64(len(p)) - 1
	if end >= r.size {
		end = r.size - 1
	}
	rangeHeader := fmt.Sprintf("bytes=%d-%d", off, end)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := r.client.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(r.key),
		Range:  aws.String(rangeHeader),
	})
	if err != nil {
		return 0, errors.Wrap(err, "s3 get object")
	}
	defer func() { _ = out.Body.Close() }()

	return io.ReadFull(out.Body, p[:end-off+1])
}

func (c *S3Client) GetObjectSize(bucket, key string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, errors.Wrap(err, "head object")
	}
	if out.ContentLength == nil {
		return 0, errors.New("content length is nil")
	}
	return *out.ContentLength, nil
}

func (c *S3Client) OpenParquet(bucket, key string) (*parquet.File, io.ReaderAt, int64, error) {
	size, err := c.GetObjectSize(bucket, key)
	if err != nil {
		return nil, nil, 0, err
	}

	reader := &s3ReaderAt{
		client: c,
		bucket: bucket,
		key:    key,
		size:   size,
	}

	pf, err := parquet.OpenFile(reader, size)
	if err != nil {
		return nil, nil, 0, errors.Wrap(err, "open parquet")
	}

	return pf, reader, size, nil
}

func (c *S3Client) ReadFile(bucket, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, errors.Wrap(err, "get object")
	}
	defer func() { _ = out.Body.Close() }()

	return io.ReadAll(out.Body)
}

func (c *S3Client) WriteFile(bucket, key string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return errors.Wrap(err, "put object")
	}
	return nil
}

func (c *S3Client) Exists(bucket, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *S3Client) BuildFromParquet(bucket, key, jsonColumn string, ginCfg GINConfig) (*GINIndex, error) {
	_, reader, size, err := c.OpenParquet(bucket, key)
	if err != nil {
		return nil, err
	}
	return BuildFromParquetReader("", jsonColumn, ginCfg, reader, size)
}

func (c *S3Client) WriteSidecar(bucket, parquetKey string, idx *GINIndex) error {
	data, err := Encode(idx)
	if err != nil {
		return errors.Wrap(err, "encode index")
	}
	sidecarKey := parquetKey + ".gin"
	return c.WriteFile(bucket, sidecarKey, data)
}

func (c *S3Client) ReadSidecar(bucket, parquetKey string) (*GINIndex, error) {
	sidecarKey := parquetKey + ".gin"
	data, err := c.ReadFile(bucket, sidecarKey)
	if err != nil {
		return nil, errors.Wrap(err, "read sidecar")
	}
	return Decode(data)
}

func (c *S3Client) HasSidecar(bucket, parquetKey string) (bool, error) {
	sidecarKey := parquetKey + ".gin"
	return c.Exists(bucket, sidecarKey)
}

func (c *S3Client) ReadFromParquetMetadata(bucket, key string, cfg ParquetConfig) (*GINIndex, error) {
	_, reader, size, err := c.OpenParquet(bucket, key)
	if err != nil {
		return nil, err
	}
	return ReadFromParquetMetadataReader("", cfg, reader, size)
}

func (c *S3Client) HasGINIndex(bucket, key string, cfg ParquetConfig) (bool, error) {
	_, reader, size, err := c.OpenParquet(bucket, key)
	if err != nil {
		return false, err
	}
	return HasGINIndexReader("", cfg, reader, size)
}

func (c *S3Client) LoadIndex(bucket, parquetKey string, cfg ParquetConfig) (*GINIndex, error) {
	idx, err := c.ReadFromParquetMetadata(bucket, parquetKey, cfg)
	if err == nil {
		return idx, nil
	}

	hasSidecar, err := c.HasSidecar(bucket, parquetKey)
	if err != nil {
		return nil, err
	}
	if hasSidecar {
		return c.ReadSidecar(bucket, parquetKey)
	}

	return nil, errors.New("no GIN index found (checked embedded metadata and sidecar)")
}

func (c *S3Client) ListParquetFiles(bucket, prefix string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var keys []string
	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "list objects")
		}
		for _, obj := range page.Contents {
			if obj.Key != nil && strings.HasSuffix(*obj.Key, ".parquet") {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}

func (c *S3Client) ListGINFiles(bucket, prefix string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var keys []string
	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "list objects")
		}
		for _, obj := range page.Contents {
			if obj.Key != nil && strings.HasSuffix(*obj.Key, ".gin") {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}

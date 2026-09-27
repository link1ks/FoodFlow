package storage

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Store interface {
	Put(context.Context, string, io.Reader, string) error
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

var safeKey = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,199}$`)

func validate(key string) error {
	if !safeKey.MatchString(key) || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return errors.New("invalid object key")
	}
	return nil
}

type Local struct{ Root string }

func (s Local) path(key string) (string, error) {
	if e := validate(key); e != nil {
		return "", e
	}
	return filepath.Join(s.Root, filepath.FromSlash(key)), nil
}
func (s Local) Put(_ context.Context, key string, src io.Reader, _ string) error {
	path, e := s.path(key)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	n, e := io.Copy(f, io.LimitReader(src, (20<<20)+1))
	if e != nil {
		f.Close()
		return e
	}
	if n > 20<<20 {
		f.Close()
		return errors.New("object exceeds 20 MiB")
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func (s Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, e := s.path(key)
	if e != nil {
		return nil, e
	}
	return os.Open(path)
}
func (s Local) Delete(_ context.Context, key string) error {
	path, e := s.path(key)
	if e != nil {
		return e
	}
	return os.Remove(path)
}

type S3 struct {
	Client *s3.Client
	Bucket string
}

func NewS3(endpoint, region, bucket, accessKey, secretKey string) (S3, error) {
	if endpoint == "" || region == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return S3{}, errors.New("incomplete S3 configuration")
	}
	cfg := aws.Config{Region: region, Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""))}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true })
	return S3{Client: client, Bucket: bucket}, nil
}
func (s S3) Put(ctx context.Context, key string, src io.Reader, contentType string) error {
	if e := validate(key); e != nil {
		return e
	}
	_, e := s.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key), Body: src, ContentType: aws.String(contentType)})
	return e
}
func (s S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if e := validate(key); e != nil {
		return nil, e
	}
	out, e := s.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	if e != nil {
		return nil, e
	}
	return out.Body, nil
}
func (s S3) Delete(ctx context.Context, key string) error {
	if e := validate(key); e != nil {
		return e
	}
	_, e := s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	return e
}

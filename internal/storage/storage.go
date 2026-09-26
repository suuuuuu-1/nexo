package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectStorage interface {
	// PresignGet 为已通过权益校验的资源生成短期访问地址。
	PresignGet(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error)
	// PresignPut 为运营人员生成短期上传地址；对象 key 由服务端生成。
	PresignPut(ctx context.Context, bucket, objectKey, contentType string, ttl time.Duration) (string, error)
	// HeadObject 在发布 Episode 前确认对象确实存在，并读取服务端保存的元数据。
	HeadObject(ctx context.Context, bucket, objectKey string) (ObjectInfo, error)
}

var ErrUploadUnsupported = errors.New("object storage does not support uploads")

// ObjectInfo 是对象存储返回的必要元数据，不包含对象内容。
type ObjectInfo struct {
	Size        int64
	ContentType string
}

// MockStorage 用于本地开发和未配置 R2 时验证业务链路，不代表真实的视频播放地址。
type MockStorage struct{}

func (MockStorage) PresignGet(_ context.Context, bucket, objectKey string, _ time.Duration) (string, error) {
	return "mock://" + url.PathEscape(strings.Trim(bucket+"/"+objectKey, "/")), nil
}

func (MockStorage) PresignPut(context.Context, string, string, string, time.Duration) (string, error) {
	return "", ErrUploadUnsupported
}

func (MockStorage) HeadObject(context.Context, string, string) (ObjectInfo, error) {
	return ObjectInfo{}, ErrUploadUnsupported
}

type R2Storage struct {
	// R2 兼容 S3 API，使用预签名客户端生成临时 GET URL。
	presigner *s3.PresignClient
	client    *s3.Client
}

// New 根据配置选择 Mock Storage 或 Cloudflare R2 实现。
func New(provider, endpoint, accessKey, secretKey string) (ObjectStorage, error) {
	if strings.ToLower(provider) != "r2" {
		return MockStorage{}, nil
	}
	if endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("r2 storage requires endpoint, access key and secret key")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	return &R2Storage{presigner: s3.NewPresignClient(client), client: client}, nil
}

// PresignGet 生成指定对象的短期下载地址，避免把永久公开 URL 写入数据库。
func (s *R2Storage) PresignGet(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error) {
	result, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}, func(options *s3.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// PresignPut 生成短期 PUT 地址；上传的 Content-Type 会在发布前通过 HeadObject 校验。
func (s *R2Storage) PresignPut(ctx context.Context, bucket, objectKey, contentType string, ttl time.Duration) (string, error) {
	result, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}, func(options *s3.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// HeadObject 只读取对象元数据，用于阻止不存在或超出限制的视频被发布。
func (s *R2Storage) HeadObject(ctx context.Context, bucket, objectKey string) (ObjectInfo, error) {
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	info := ObjectInfo{ContentType: aws.ToString(result.ContentType)}
	if result.ContentLength != nil {
		info.Size = *result.ContentLength
	}
	return info, nil
}

package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ObjectInfo 是对象存储返回的必要元数据，不包含对象内容。
type ObjectInfo struct {
	Size        int64
	ContentType string
}

type R2Storage struct {
	// R2 兼容 S3 API，使用预签名客户端生成临时 GET URL。
	presigner *s3.PresignClient
	client    *s3.Client
}

// NewR2 创建 Cloudflare R2 客户端；v1 明确要求配置真实对象存储。
func NewR2(endpoint, accessKey, secretKey string) (*R2Storage, error) {
	endpoint = strings.TrimSpace(endpoint)
	accessKey = strings.TrimSpace(accessKey)
	secretKey = strings.TrimSpace(secretKey)
	if endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("R2 requires endpoint, access key and secret key")
	}
	parsedEndpoint, err := url.ParseRequestURI(endpoint)
	if err != nil || parsedEndpoint.Scheme != "https" || parsedEndpoint.Host == "" {
		return nil, fmt.Errorf("R2 endpoint must be an HTTPS URL")
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

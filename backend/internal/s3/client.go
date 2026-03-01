package s3

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"github.com/pushkit/backend/internal/config"
)

type Client struct {
	s3     *s3.Client
	presigner *s3.PresignClient
	bucket string
	ttl    int
}

func NewClient(cfg *config.Config) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.AWSRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretKey, ""),
		),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.S3EndpointURL != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.S3EndpointURL)
			o.UsePathStyle = true // Required for MinIO/LocalStack.
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &Client{
		s3:        client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.S3Bucket,
		ttl:       cfg.PresignTTLSecs,
	}, nil
}

func (c *Client) GenerateUploadURL(ctx context.Context, s3Key, contentType string) (string, error) {
	req, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(s3Key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(time.Duration(c.ttl)*time.Second))
	if err != nil {
		return "", fmt.Errorf("presign PUT: %w", err)
	}
	return req.URL, nil
}

func (c *Client) GenerateDownloadURL(ctx context.Context, s3Key, originalFilename string) (string, error) {
	safeName := sanitizeFilenameForHeader(originalFilename)
	disposition := fmt.Sprintf(`attachment; filename="%s"`, safeName)

	req, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(c.bucket),
		Key:                        aws.String(s3Key),
		ResponseContentDisposition: aws.String(disposition),
	}, s3.WithPresignExpires(time.Duration(c.ttl)*time.Second))
	if err != nil {
		return "", fmt.Errorf("presign GET: %w", err)
	}
	return req.URL, nil
}

func (c *Client) DeleteObject(ctx context.Context, s3Key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return fmt.Errorf("delete object %s: %w", s3Key, err)
	}
	return nil
}

func (c *Client) TTL() int {
	return c.ttl
}

// BuildS3Key generates the S3 key: uploads/{userId}/{yyyy}/{mm}/{dd}/{uuid}-{sanitizedFilename}
func BuildS3Key(userID, filename string) string {
	now := time.Now().UTC()
	sanitized := SanitizeFilename(filename)
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}
	return fmt.Sprintf("uploads/%s/%04d/%02d/%02d/%s-%s",
		userID, now.Year(), now.Month(), now.Day(),
		uuid.New().String(), sanitized,
	)
}

var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// SanitizeFilename replaces unsafe characters with underscores.
func SanitizeFilename(name string) string {
	// Remove path separators first.
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.TrimSpace(name)
	name = safeFilenameRe.ReplaceAllString(name, "_")
	if name == "" {
		name = "file"
	}
	return name
}

func sanitizeFilenameForHeader(name string) string {
	return SanitizeFilename(name)
}

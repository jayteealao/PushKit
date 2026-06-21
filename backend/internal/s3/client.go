package s3

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/google/uuid"

	"github.com/pushkit/backend/internal/config"
)

// stripSDKHeaders removes AWS SDK telemetry and encoding headers that break
// SigV4 signing on S3-compatible backends (GCS, MinIO, R2):
//
//   - Amz-Sdk-Invocation-Id / Amz-Sdk-Request: AWS-internal telemetry not
//     understood by non-AWS S3 implementations.
//   - Accept-Encoding: GCS's front-end (GFE) appends ",gzip(gfe)" after
//     signing, causing SignatureDoesNotMatch errors.
func stripSDKHeaders(req *smithyhttp.Request) {
	req.Header.Del("Amz-Sdk-Invocation-Id")
	req.Header.Del("Amz-Sdk-Request")
	req.Header.Del("Accept-Encoding")
}

// stripSDKHeadersMiddleware returns a named FinalizeMiddleware that calls
// stripSDKHeaders before the request is signed.
func stripSDKHeadersMiddleware() middleware.FinalizeMiddleware {
	return middleware.FinalizeMiddlewareFunc("StripSDKHeaders",
		func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
			if req, ok := in.Request.(*smithyhttp.Request); ok {
				stripSDKHeaders(req)
			}
			return next.HandleFinalize(ctx, in)
		},
	)
}

type Client struct {
	s3        *s3.Client
	presigner *s3.PresignClient
	bucket    string
	ttl       int
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
			o.UsePathStyle = true // Required for MinIO/GCS/R2.
			// Disable automatic checksums — S3-compatible services (GCS, MinIO, R2)
			// don't support the x-amz-checksum-* headers that AWS SDK v2 adds by
			// default, which causes SignatureDoesNotMatch errors.
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
			// Strip headers that break S3-compatible signing before they're
			// included in the SigV4 canonical request.
			o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
				// Add the strip middleware to the head of the Finalize step.
				// Unlike Insert, Add is not coupled to an internal step ID, so it
				// runs on BOTH the normal API pipeline (which has a "Signing"
				// step) and the presign pipeline (which has none). The presign
				// path MUST strip too: these headers are injected in the earlier
				// Build step, so left in place they land in the presigned URL's
				// X-Amz-SignedHeaders, and any non-SDK client (Android, curl, the
				// MCP CLI) that later PUTs to that URL fails with
				// SignatureDoesNotMatch. Head-of-Finalize is before signing in
				// both pipelines because the headers come from Build and are not
				// re-added within Finalize. Add only errors on a duplicate ID, so
				// propagate that rather than swallow it.
				return stack.Finalize.Add(stripSDKHeadersMiddleware(), middleware.Before)
			})
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

// HeadObject checks whether an S3 object exists. Returns (true, nil) if it exists,
// (false, nil) if it does not, or (false, err) on unexpected errors.
func (c *Client) HeadObject(ctx context.Context, s3Key string) (bool, error) {
	_, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		// Check typed S3 not-found errors first (AWS SDK v2 canonical path).
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return false, nil
		}
		var nf *types.NotFound
		if errors.As(err, &nf) {
			return false, nil
		}
		// S3-compatible backends (MinIO, R2, GCS) may surface a 404 wrapped
		// in a smithy ResponseError rather than a typed S3 error. Inspect the
		// HTTP status code directly so we don't rely on error strings.
		var re *smithyhttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 404 {
			return false, nil
		}
		return false, fmt.Errorf("head object %s: %w", s3Key, err)
	}
	return true, nil
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

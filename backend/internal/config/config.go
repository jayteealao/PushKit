package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	S3Bucket       string
	AWSRegion      string
	AWSAccessKeyID string
	AWSSecretKey   string
	S3EndpointURL  string // optional, for MinIO/R2
	DatabaseURL    string
	ListenAddr     string
	APIKeyMap      map[string]string // key -> user_id
	PresignTTLSecs int
	// TLS is optional. When both TLSCertFile and TLSKeyFile are set, the
	// server starts in TLS mode (HTTPS). Leave either empty for plain HTTP.
	TLSCertFile string // TLS_CERT_FILE env var
	TLSKeyFile  string // TLS_KEY_FILE env var
}

func Load() (*Config, error) {
	cfg := &Config{
		AWSRegion:      getEnvDefault("AWS_REGION", "us-east-1"),
		S3Bucket:       os.Getenv("S3_BUCKET"),
		AWSAccessKeyID: os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretKey:   os.Getenv("AWS_SECRET_ACCESS_KEY"),
		S3EndpointURL:  os.Getenv("S3_ENDPOINT_URL"),
		DatabaseURL:    getEnvDefault("DATABASE_URL", "pushkit.db"),
		ListenAddr:     getEnvDefault("LISTEN_ADDR", ":8000"),
		PresignTTLSecs: 900,
		TLSCertFile:    os.Getenv("TLS_CERT_FILE"),
		TLSKeyFile:     os.Getenv("TLS_KEY_FILE"),
	}

	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}
	if cfg.AWSAccessKeyID == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID is required")
	}
	if cfg.AWSSecretKey == "" {
		return nil, fmt.Errorf("AWS_SECRET_ACCESS_KEY is required")
	}

	apiKeys := os.Getenv("API_KEYS")
	if apiKeys == "" {
		return nil, fmt.Errorf("API_KEYS is required (format: key1:user1,key2:user2)")
	}

	keyMap, err := parseAPIKeys(apiKeys)
	if err != nil {
		return nil, fmt.Errorf("invalid API_KEYS: %w", err)
	}
	cfg.APIKeyMap = keyMap

	return cfg, nil
}

func parseAPIKeys(raw string) (map[string]string, error) {
	m := make(map[string]string)
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid key:user pair: %q", pair)
		}
		m[parts[0]] = parts[1]
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("at least one key:user pair is required")
	}
	return m, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

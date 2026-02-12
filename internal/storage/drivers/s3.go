package drivers

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/net/http2"
)

// S3HTTPConfig contains HTTP client configuration for S3 connections
type S3HTTPConfig struct {
	MaxIdleConns          int `json:"max_idle_conns,omitempty"`              // Max idle connections across all hosts (default: 100)
	MaxIdleConnsPerHost   int `json:"max_idle_conns_per_host,omitempty"`     // Max idle connections per host (default: 100)
	MaxConnsPerHost       int `json:"max_conns_per_host,omitempty"`          // Max total connections per host (default: 0 = unlimited)
	IdleConnTimeout       int `json:"idle_conn_timeout_sec,omitempty"`       // Idle connection timeout in seconds (default: 90)
	ConnectTimeout        int `json:"connect_timeout_sec,omitempty"`         // Connection timeout in seconds (default: 10)
	RequestTimeout        int `json:"request_timeout_sec,omitempty"`         // Full request timeout in seconds (default: 30)
	ResponseHeaderTimeout int `json:"response_header_timeout_sec,omitempty"` // Response header timeout in seconds (default: 10)
}

type S3Client struct {
	client *s3.Client
	bucket string
}

// createOptimizedHTTPClient creates an HTTP client with optimized connection pooling and timeouts
func createOptimizedHTTPClient(httpConfig *S3HTTPConfig) *http.Client {
	// Set sensible defaults if config is nil or values not specified
	maxIdleConns := 100
	maxIdleConnsPerHost := 100
	maxConnsPerHost := 0 // 0 = unlimited
	idleConnTimeout := 90
	connectTimeout := 10
	requestTimeout := 30
	responseHeaderTimeout := 10

	if httpConfig != nil {
		if httpConfig.MaxIdleConns > 0 {
			maxIdleConns = httpConfig.MaxIdleConns
		}
		if httpConfig.MaxIdleConnsPerHost > 0 {
			maxIdleConnsPerHost = httpConfig.MaxIdleConnsPerHost
		}
		if httpConfig.MaxConnsPerHost > 0 {
			maxConnsPerHost = httpConfig.MaxConnsPerHost
		}
		if httpConfig.IdleConnTimeout > 0 {
			idleConnTimeout = httpConfig.IdleConnTimeout
		}
		if httpConfig.ConnectTimeout > 0 {
			connectTimeout = httpConfig.ConnectTimeout
		}
		if httpConfig.RequestTimeout > 0 {
			requestTimeout = httpConfig.RequestTimeout
		}
		if httpConfig.ResponseHeaderTimeout > 0 {
			responseHeaderTimeout = httpConfig.ResponseHeaderTimeout
		}
	}

	// Create custom transport with optimized settings
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(connectTimeout) * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true, // Enable HTTP/2
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		MaxConnsPerHost:       maxConnsPerHost,
		IdleConnTimeout:       time.Duration(idleConnTimeout) * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: time.Duration(responseHeaderTimeout) * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Configure HTTP/2
	if err := http2.ConfigureTransport(transport); err != nil {
		log.Printf("[S3 Storage] Warning: Failed to configure HTTP/2: %v", err)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(requestTimeout) * time.Second,
	}

	log.Printf("[S3 Storage] HTTP client configured: MaxIdleConns=%d, MaxIdleConnsPerHost=%d, MaxConnsPerHost=%d, ConnectTimeout=%ds, RequestTimeout=%ds",
		maxIdleConns, maxIdleConnsPerHost, maxConnsPerHost, connectTimeout, requestTimeout)

	return client
}

func NewS3Client(region, accessKey, secretKey, bucket, baseURL string, httpConfig *S3HTTPConfig) (*S3Client, error) {
	var s3Client *s3.Client

	// Create optimized HTTP client with config
	httpClient := createOptimizedHTTPClient(httpConfig)

	if baseURL != "" {
		log.Printf("[S3 Storage] Initializing S3-compatible storage: endpoint=%s, bucket=%s, region=%s", baseURL, bucket, region)
		// For S3-compatible storage (MinIO, etc.), use a simplified config
		// to avoid AWS-specific credential validation
		s3Client = s3.New(s3.Options{
			Region:       region,
			Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
			BaseEndpoint: aws.String(baseURL),
			UsePathStyle: true,
			HTTPClient:   httpClient,
		})
	} else {
		log.Printf("[S3 Storage] Initializing AWS S3 storage: bucket=%s, region=%s", bucket, region)
		// For standard AWS S3, use the full config loader
		configOpts := []func(*config.LoadOptions) error{
			config.WithRegion(region),
			config.WithHTTPClient(httpClient),
		}

		if accessKey != "" && secretKey != "" {
			configOpts = append(configOpts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
			))
		}

		cfg, err := config.LoadDefaultConfig(context.TODO(), configOpts...)
		if err != nil {
			return nil, err
		}

		s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	log.Printf("[S3 Storage] Client initialized successfully for bucket: %s", bucket)
	return &S3Client{
		client: s3Client,
		bucket: bucket,
	}, nil
}

func (s *S3Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	log.Printf("[S3 Storage] Fetching object: bucket=%s, key=%s", s.bucket, key)
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Printf("[S3 Storage] Error fetching object: bucket=%s, key=%s, error=%v", s.bucket, key, err)
		return nil, err
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("[S3 Storage] Error reading object body: bucket=%s, key=%s, error=%v", s.bucket, key, err)
		return nil, err
	}
	log.Printf("[S3 Storage] Successfully fetched object: bucket=%s, key=%s, size=%d bytes", s.bucket, key, len(data))
	return data, nil
}

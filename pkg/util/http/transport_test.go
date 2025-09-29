package http

import (
	"testing"
	"time"
)

func TestDefaultTransportConfig(t *testing.T) {
	config := DefaultTransportConfig()

	if config.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", config.MaxIdleConns)
	}
	if config.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 10, got %d", config.MaxIdleConnsPerHost)
	}
	if config.MaxConnsPerHost != 50 {
		t.Errorf("Expected MaxConnsPerHost to be 50, got %d", config.MaxConnsPerHost)
	}
	if config.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", config.IdleConnTimeout)
	}
}

func TestNewTransport_DefaultConfig(t *testing.T) {
	transport := NewTransport("https://api.github.com", false)

	if transport == nil {
		t.Fatal("Expected transport to be non-nil")
	}

	if transport.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 10, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.MaxConnsPerHost != 50 {
		t.Errorf("Expected MaxConnsPerHost to be 50, got %d", transport.MaxConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", transport.IdleConnTimeout)
	}
	if transport.Proxy == nil {
		t.Error("Expected Proxy to be set to http.ProxyFromEnvironment")
	}
}

func TestNewTransport_InsecureSkipVerify(t *testing.T) {
	transport := NewTransport("https://api.github.com", true)

	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLSClientConfig to be non-nil")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
}

func TestNewTransport_SecureWithoutResolver(t *testing.T) {
	// Reset cert resolver
	SetCertResolver(nil)

	transport := NewTransport("https://api.github.com", false)

	if transport.TLSClientConfig != nil {
		t.Error("Expected TLSClientConfig to be nil when no cert resolver is set")
	}
}

func TestNewTransport_WithCertResolver(t *testing.T) {
	testCert := `-----BEGIN CERTIFICATE-----
MIICpDCCAYwCCQDU+pQ3ZUD30jANBgkqhkiG9w0BAQsFADAUMRIwEAYDVQQDDAls
b2NhbGhvc3QwHhcNMjQwMTAxMDAwMDAwWhcNMjUwMTAxMDAwMDAwWjAUMRIwEAYD
VQQDDAlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC7
VJTUt9Us8cKjMzEfYyjiWA4R4/M2bS1+fWIcPm15A8+raZ4QgHmWC3gLgTJzWJ6l
K7dLHHIj3Hg6xpKmgpNiwDPBjvKLHQElvLOtA9qL5pJQjSc7gXqj5bqoWL0HfmfL
Fg3PUxGdW1+uU3PJjUjLbZMh7zBh+pjw8IIhQKJhHcBmPVjLkK0oVmMfQWbqZh5V
v6hVjwJQcHVQN2r2bphgTIWjJMYLFVKLKGqWmQXVn5pTiMtU+pJHv0xCxVnqMqVZ
1PHZLvQYXJcWYqNmTNxPMvXqGaHhTe3B7bLYPl8fP1m2w8VhQhVMdLKJJ4vKLjWL
q4HsEpH0gLHJcNW3RsVpAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAI8bfVn9CJKd
yLlPQGqJHqo8B/NM4RmTxwSVKGSWXfFmzrVnJGKLhLcCfVnZhPGhCqwVOEJxXvqN
jJCdOPwLjR7EFrYYXqVvCW7HqPVKvJcHvBrBPDrKpJLFlBhScYYTF6e9qEwXBZj8
xCLSQQ3BfPBsVWFVKGZFJ4hV3eNQqJfBPqDTHqGqjcJQLmDqyqCTvCOJWKYmYXEe
8wQ7VONJxTLVGMWMNGM0McF7NOvZVqPLqELQKHlLYq9LQHH8xqJPVhCZnLqJYBnc
bqcZCUoRqZQxPpYQKCYCPRDQWzLGHJLYHDnGfPLqZGMKhLHMGLBYJNWBLGHJLYHD
nGfPLqZGMKhLHMGLBYJNWBLGHJLYHDnGfPL=
-----END CERTIFICATE-----`

	// Set up cert resolver that returns a test certificate
	SetCertResolver(func(serverName string) ([]string, error) {
		return []string{testCert}, nil
	})
	defer SetCertResolver(nil) // Clean up

	transport := NewTransport("https://api.github.com", false)

	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLSClientConfig to be non-nil when cert resolver is set")
	}
	if transport.TLSClientConfig.RootCAs == nil {
		t.Error("Expected RootCAs to be set")
	}
}

func TestNewTransport_InvalidURL(t *testing.T) {
	SetCertResolver(func(serverName string) ([]string, error) {
		return []string{"cert"}, nil
	})
	defer SetCertResolver(nil)

	transport := NewTransport("://invalid-url", false)

	if transport == nil {
		t.Fatal("Expected transport to be non-nil even with invalid URL")
	}
	// Should still have default connection settings
	if transport.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100 even with invalid URL, got %d", transport.MaxIdleConns)
	}
}

func TestNewTransportWithConfig_CustomConfig(t *testing.T) {
	customConfig := &TransportConfig{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     120 * time.Second,
	}

	transport := NewTransportWithConfig("https://api.github.com", false, customConfig)

	if transport.MaxIdleConns != 200 {
		t.Errorf("Expected MaxIdleConns to be 200, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 20 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 20, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.MaxConnsPerHost != 100 {
		t.Errorf("Expected MaxConnsPerHost to be 100, got %d", transport.MaxConnsPerHost)
	}
	if transport.IdleConnTimeout != 120*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 120s, got %v", transport.IdleConnTimeout)
	}
}

func TestNewTransportWithConfig_NilConfig(t *testing.T) {
	transport := NewTransportWithConfig("https://api.github.com", false, nil)

	// Should use default config when nil is passed
	if transport.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100 with nil config, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 10 with nil config, got %d", transport.MaxIdleConnsPerHost)
	}
}

func TestNewTransportWithConfig_InsecureWithCustomConfig(t *testing.T) {
	customConfig := &TransportConfig{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     25,
		IdleConnTimeout:     60 * time.Second,
	}

	transport := NewTransportWithConfig("https://api.github.com", true, customConfig)

	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLSClientConfig to be non-nil")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
	if transport.MaxIdleConns != 50 {
		t.Errorf("Expected MaxIdleConns to be 50, got %d", transport.MaxIdleConns)
	}
}

func TestGetCertPoolFromPEMData(t *testing.T) {
	testCerts := []string{
		`-----BEGIN CERTIFICATE-----
MIICpDCCAYwCCQDU+pQ3ZUD30jANBgkqhkiG9w0BAQsFADAUMRIwEAYDVQQDDAls
b2NhbGhvc3QwHhcNMjQwMTAxMDAwMDAwWhcNMjUwMTAxMDAwMDAwWjAUMRIwEAYD
VQQDDAlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC7
VJTUt9Us8cKjMzEfYyjiWA4R4/M2bS1+fWIcPm15A8+raZ4QgHmWC3gLgTJzWJ6l
K7dLHHIj3Hg6xpKmgpNiwDPBjvKLHQElvLOtA9qL5pJQjSc7gXqj5bqoWL0HfmfL
Fg3PUxGdW1+uU3PJjUjLbZMh7zBh+pjw8IIhQKJhHcBmPVjLkK0oVmMfQWbqZh5V
v6hVjwJQcHVQN2r2bphgTIWjJMYLFVKLKGqWmQXVn5pTiMtU+pJHv0xCxVnqMqVZ
1PHZLvQYXJcWYqNmTNxPMvXqGaHhTe3B7bLYPl8fP1m2w8VhQhVMdLKJJ4vKLjWL
q4HsEpH0gLHJcNW3RsVpAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAI8bfVn9CJKd
yLlPQGqJHqo8B/NM4RmTxwSVKGSWXfFmzrVnJGKLhLcCfVnZhPGhCqwVOEJxXvqN
jJCdOPwLjR7EFrYYXqVvCW7HqPVKvJcHvBrBPDrKpJLFlBhScYYTF6e9qEwXBZj8
xCLSQQ3BfPBsVWFVKGZFJ4hV3eNQqJfBPqDTHqGqjcJQLmDqyqCTvCOJWKYmYXEe
8wQ7VONJxTLVGMWMNGM0McF7NOvZVqPLqELQKHlLYq9LQHH8xqJPVhCZnLqJYBnc
bqcZCUoRqZQxPpYQKCYCPRDQWzLGHJLYHDnGfPLqZGMKhLHMGLBYJNWBLGHJLYHD
nGfPLqZGMKhLHMGLBYJNWBLGHJLYHDnGfPL=
-----END CERTIFICATE-----`,
	}

	certPool := getCertPoolFromPEMData(testCerts)

	if certPool == nil {
		t.Fatal("Expected certPool to be non-nil")
	}
}

func TestGetCertPoolFromPEMData_EmptyList(t *testing.T) {
	certPool := getCertPoolFromPEMData([]string{})

	if certPool == nil {
		t.Fatal("Expected certPool to be non-nil even with empty list")
	}
}

func TestSetCertResolver(t *testing.T) {
	originalResolver := certResolver
	defer func() { certResolver = originalResolver }()

	testResolver := func(serverName string) ([]string, error) {
		return []string{"test"}, nil
	}

	SetCertResolver(testResolver)

	if certResolver == nil {
		t.Error("Expected certResolver to be set")
	}
}

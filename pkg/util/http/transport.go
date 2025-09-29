package http

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"time"
)

var certResolver func(serverName string) ([]string, error)

// TransportConfig holds configuration for HTTP transport connection management
type TransportConfig struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
}

// DefaultTransportConfig returns sensible defaults for connection management
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
	}
}

func SetCertResolver(resolver func(serverName string) ([]string, error)) {
	certResolver = resolver
}

// NewTransport creates an HTTP transport with default connection limits
func NewTransport(rawURL string, insecureSkipVerify bool) *http.Transport {
	return NewTransportWithConfig(rawURL, insecureSkipVerify, DefaultTransportConfig())
}

// NewTransportWithConfig creates an HTTP transport with custom connection limits
func NewTransportWithConfig(rawURL string, insecureSkipVerify bool, config *TransportConfig) *http.Transport {
	if config == nil {
		config = DefaultTransportConfig()
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
	}

	if insecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	} else if certResolver != nil {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return transport
		}
		serverCertificatePem, err := certResolver(parsedURL.Host)
		if err != nil {
			return transport
		}
		if len(serverCertificatePem) > 0 {
			transport.TLSClientConfig = &tls.Config{
				RootCAs: getCertPoolFromPEMData(serverCertificatePem),
			}
		}
	}

	return transport
}

func getCertPoolFromPEMData(pemData []string) *x509.CertPool {
	certPool := x509.NewCertPool()
	for _, pem := range pemData {
		certPool.AppendCertsFromPEM([]byte(pem))
	}
	return certPool
}
package http

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

type TransportOptions struct {
	MaxIdleConns        int    `json:"maxIdleConns"`
	MaxIdleConnsPerHost int    `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int    `json:"maxConnsPerHost"`
	IdleConnTimeout     string `json:"idleConnTimeout"`
}

var certResolver func(serverName string) ([]string, error)

func SetCertResolver(resolver func(serverName string) ([]string, error)) {
	certResolver = resolver
}

func NewTransport(tp TransportOptions, rawURL string, insecureSkipVerify bool) *http.Transport {
	// Parse IdleConnTimeout from string if provided
	var idleConnTimeout time.Duration
	if tp.IdleConnTimeout != "" {
		dur, err := time.ParseDuration(tp.IdleConnTimeout)
		if err == nil {
			idleConnTimeout = dur
		}
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        tp.MaxIdleConns,
		MaxIdleConnsPerHost: tp.MaxIdleConnsPerHost,
		MaxConnsPerHost:     tp.MaxConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
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
		} else if len(serverCertificatePem) > 0 {
			transport.TLSClientConfig = &tls.Config{
				RootCAs: getCertPoolFromPEMData(serverCertificatePem),
			}
		}
	}

	return transport
}

func NewServiceHTTPClient(tp TransportOptions, insecureSkipVerify bool, apiURL string, serviceName string) (*http.Client, error) {
	transport := NewTransport(tp, apiURL, insecureSkipVerify)
	return &http.Client{
		Transport: NewLoggingRoundTripper(transport, log.WithField("service", serviceName)),
	}, nil
}

func getCertPoolFromPEMData(pemData []string) *x509.CertPool {
	certPool := x509.NewCertPool()
	for _, pem := range pemData {
		certPool.AppendCertsFromPEM([]byte(pem))
	}
	return certPool
}

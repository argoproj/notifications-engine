package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

var certResolver func(serverName string) ([]string, error)

func SetCertResolver(resolver func(serverName string) ([]string, error)) {
	certResolver = resolver
}

func NewTransport(rawURL string, maxIdleConns int, maxIdleConnsPerHost int, maxConnsPerHost int, idleConnTimeout time.Duration, insecureSkipVerify bool) *http.Transport {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		MaxConnsPerHost:     maxConnsPerHost,
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

func NewServiceHTTPClient(maxIdleConns, maxIdleConnsPerHost, maxConnsPerHost int, idleConnTimeout string, insecureSkipVerify bool, apiURL string, serviceName string) (client *http.Client, err error) {
	var timeout time.Duration
	if idleConnTimeout != "" {
		timeout, err = time.ParseDuration(idleConnTimeout)
		if err != nil {
			return nil, fmt.Errorf("failed to parse idle connection timeout: %w", err)
		}
	}
	transport := NewTransport(apiURL, maxIdleConns, maxIdleConnsPerHost, maxConnsPerHost, timeout, insecureSkipVerify)
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

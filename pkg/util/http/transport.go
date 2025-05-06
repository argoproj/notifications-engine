package http

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"time"
)

type HTTPTransportSettings struct {
	MaxIdleConns        int           `json:"maxIdleConns"`
	MaxIdleConnsPerHost int           `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int           `json:"maxConnsPerHost"`
	IdleConnTimeout     time.Duration `json:"idleConnTimeout"`
	InsecureSkipVerify  bool          `json:"insecureSkipVerify"`
}

var certResolver func(serverName string) ([]string, error)

func SetCertResolver(resolver func(serverName string) ([]string, error)) {
	certResolver = resolver
}

func NewTransport(rawURL string, set HTTPTransportSettings) *http.Transport {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        set.MaxIdleConns,
		MaxIdleConnsPerHost: set.MaxIdleConnsPerHost,
		MaxConnsPerHost:     set.MaxConnsPerHost,
		IdleConnTimeout:     set.IdleConnTimeout,
	}
	if set.InsecureSkipVerify {
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

func getCertPoolFromPEMData(pemData []string) *x509.CertPool {
	certPool := x509.NewCertPool()
	for _, pem := range pemData {
		certPool.AppendCertsFromPEM([]byte(pem))
	}
	return certPool
}

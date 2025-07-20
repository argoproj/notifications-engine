package http

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
)

var certResolver func(serverName string) ([]string, error)

func SetCertResolver(resolver func(serverName string) ([]string, error)) {
	certResolver = resolver
}

func GetTLSConfig(rawURL string, insecureSkipVerify bool) (*tls.Config, error) {
	if insecureSkipVerify {
		return &tls.Config{
			InsecureSkipVerify: true,
		}, nil
	}
	if certResolver != nil {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		return GetTLSConfigFromHost(parsedURL.Host, insecureSkipVerify)
	}

	return nil, nil
}

func GetTLSConfigFromHost(host string, insecureSkipVerify bool) (*tls.Config, error) {
	if insecureSkipVerify {
		return &tls.Config{
			InsecureSkipVerify: true,
		}, nil
	}
	if certResolver != nil {
		serverCertificatePem, err := certResolver(host)
		if err != nil {
			return nil, err
		} else if len(serverCertificatePem) > 0 {
			return &tls.Config{
				RootCAs: getCertPoolFromPEMData(serverCertificatePem),
			}, nil
		}
	}

	return nil, nil
}

func NewTransport(rawURL string, insecureSkipVerify bool) *http.Transport {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	config, err := GetTLSConfig(rawURL, insecureSkipVerify)
	if err != nil {
		return transport
	}

	transport.TLSClientConfig = config
	return transport
}

func getCertPoolFromPEMData(pemData []string) *x509.CertPool {
	certPool := x509.NewCertPool()
	for _, pem := range pemData {
		certPool.AppendCertsFromPEM([]byte(pem))
	}
	return certPool
}

package client

import (
	"crypto/tls"
	"fmt"
)

type TLSConfig struct {
	Cert string
	Key  string
}

// toTLSConfig builds the tls.Config object based on the content of the grafana-tls secret
func (t TLSConfig) toTLSConfig() (*tls.Config, error) {
	loadedCrt, err := tls.X509KeyPair([]byte(t.Cert), []byte(t.Key))
	if err != nil {
		return nil, fmt.Errorf("failed to parse grafana tls certificate : %w", err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{loadedCrt},
	}, nil
}

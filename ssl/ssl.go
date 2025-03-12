package ssl

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"
)

type CertManager struct {
	sync.RWMutex
	certs    map[string]*tls.Certificate
	certFile string
	keyFile  string
}

func NewCertManager(certFile, keyFile string) (*CertManager, error) {
	cm := &CertManager{
		certs:    make(map[string]*tls.Certificate),
		certFile: certFile,
		keyFile:  keyFile,
	}
	if err := cm.loadCertificate(); err != nil {
		return nil, fmt.Errorf("failed to load initial certificate: %v", err)
	}
	return cm, nil
}

func (cm* CertManager) GetTLSConfig() *tls.Config{
	GetCertificate:= cm.GetCertificate
	return 	&tls.Config{
		GetCertificate: GetCertificate,
		MinVersion:     tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
}

func (cm *CertManager) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cm.RLock()
	defer cm.RUnlock()

	if cert, ok := cm.certs[clientHello.ServerName]; ok {
		return cert, nil
	}

	for _, cert := range cm.certs {
		return cert, nil
	}

	return nil, fmt.Errorf("no certificate found for %s", clientHello.ServerName)
}

func (cm *CertManager) loadCertificate() error {
	cert, err := tls.LoadX509KeyPair(cm.certFile, cm.keyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %v", err)
	}

	cm.Lock()
	defer cm.Unlock()

	if len(cert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %v", err)
		}
		
		var domain string
		if len(x509Cert.DNSNames) > 0 {
			domain = x509Cert.DNSNames[0]
		} else {
			domain = x509Cert.Subject.CommonName
		}
		
		cm.certs[domain] = &cert
	}

	return nil
}



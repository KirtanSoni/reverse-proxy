package ssl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Helper function to create test certificates
func createTestCert(t *testing.T, domain string) (certFile, keyFile string) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		DNSNames:              []string{domain},
		NotBefore:            time.Now(),
		NotAfter:             time.Now().Add(time.Hour * 24),
		KeyUsage:             x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:          []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Create temporary directory for test certificates
	tmpDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Write certificate to file
	certFile = filepath.Join(tmpDir, "cert.pem")
	certOut, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("Failed to create cert.pem: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		t.Fatalf("Failed to write cert.pem: %v", err)
	}
	certOut.Close()

	// Write private key to file
	keyFile = filepath.Join(tmpDir, "key.pem")
	keyOut, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("Failed to create key.pem: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}); err != nil {
		t.Fatalf("Failed to write key.pem: %v", err)
	}
	keyOut.Close()

	return certFile, keyFile
}

func TestNewCertManager(t *testing.T) {
	// Create test certificate
	certFile, keyFile := createTestCert(t, "example.com")
	defer os.RemoveAll(filepath.Dir(certFile))

	tests := []struct {
		name        string
		certFile    string
		keyFile     string
		expectError bool
	}{
		{
			name:        "Valid certificate files",
			certFile:    certFile,
			keyFile:     keyFile,
			expectError: false,
		},
		{
			name:        "Invalid certificate file",
			certFile:    "nonexistent.pem",
			keyFile:     keyFile,
			expectError: true,
		},
		{
			name:        "Invalid key file",
			certFile:    certFile,
			keyFile:     "nonexistent.pem",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := NewCertManager(tt.certFile, tt.keyFile)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if cm == nil {
					t.Error("Expected CertManager but got nil")
				}
			}
		})
	}
}

func TestGetCertificate(t *testing.T) {
	domain := "example.com"
	certFile, keyFile := createTestCert(t, domain)
	defer os.RemoveAll(filepath.Dir(certFile))

	cm, err := NewCertManager(certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to create CertManager: %v", err)
	}

	tests := []struct {
		name        string
		serverName  string
		expectError bool
	}{
		{
			name:        "Matching domain",
			serverName:  domain,
			expectError: false,
		},
		{
			name:        "Non-matching domain",
			serverName:  "other.com",
			expectError: false, // Should return default cert
		},
		{
			name:        "Empty server name",
			serverName:  "",
			expectError: false, // Should return default cert
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientHello := &tls.ClientHelloInfo{
				ServerName: tt.serverName,
			}
			cert, err := cm.GetCertificate(clientHello)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if cert == nil {
					t.Error("Expected certificate but got nil")
				}
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	domain := "example.com"
	certFile, keyFile := createTestCert(t, domain)
	defer os.RemoveAll(filepath.Dir(certFile))

	cm, err := NewCertManager(certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to create CertManager: %v", err)
	}

	// Number of concurrent goroutines
	numGoroutines := 100
	// Number of operations per goroutine
	numOperations := 100

	// Create a WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Multiply by 2 because we have two types of goroutines

	// Start goroutines that get certificates
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			clientHello := &tls.ClientHelloInfo{
				ServerName: domain,
			}
			for j := 0; j < numOperations; j++ {
				cert, err := cm.GetCertificate(clientHello)
				if err != nil {
					t.Errorf("Unexpected error getting certificate: %v", err)
				}
				if cert == nil {
					t.Error("Got nil certificate")
				}
			}
		}()
	}

	// Start goroutines that reload certificates
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				err := cm.loadCertificate()
				if err != nil {
					t.Errorf("Unexpected error reloading certificate: %v", err)
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

func TestRaceConditions(t *testing.T) {
	domain := "example.com"
	certFile, keyFile := createTestCert(t, domain)
	defer os.RemoveAll(filepath.Dir(certFile))

	cm, err := NewCertManager(certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to create CertManager: %v", err)
	}

	// Run multiple operations concurrently to try to trigger race conditions
	var wg sync.WaitGroup
	operations := []func(){
		func() {
			clientHello := &tls.ClientHelloInfo{ServerName: domain}
			cm.GetCertificate(clientHello)
		},
		func() {
			cm.loadCertificate()
		},
		func() {
			clientHello := &tls.ClientHelloInfo{ServerName: "other.com"}
			cm.GetCertificate(clientHello)
		},
	}

	numIterations := 100
	wg.Add(len(operations) * numIterations)

	for i := 0; i < numIterations; i++ {
		for _, op := range operations {
			go func(operation func()) {
				defer wg.Done()
				operation()
			}(op)
		}
	}

	wg.Wait()
}

func TestLoadCertificate(t *testing.T) {
	domain := "example.com"
	certFile, keyFile := createTestCert(t, domain)
	defer os.RemoveAll(filepath.Dir(certFile))

	cm := &CertManager{
		certs:    make(map[string]*tls.Certificate),
		certFile: certFile,
		keyFile:  keyFile,
	}

	if err := cm.loadCertificate(); err != nil {
		t.Errorf("Unexpected error loading certificate: %v", err)
	}

	// Verify certificate was loaded for the correct domain
	cm.RLock()
	cert, ok := cm.certs[domain]
	cm.RUnlock()

	if !ok {
		t.Errorf("Certificate not found for domain %s", domain)
	}
	if cert == nil {
		t.Error("Certificate is nil")
	}

	// Test loading invalid certificate
	cm.certFile = "nonexistent.pem"
	if err := cm.loadCertificate(); err == nil {
		t.Error("Expected error loading invalid certificate but got nil")
	}
}
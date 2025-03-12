package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kirtansoni/reverse-proxy-go/proxy"
	"golang.org/x/crypto/acme/autocert"
)

var (
	httpAddr  = flag.String("http", ":80", "HTTP address")
	httpsAddr = flag.String("https", ":443", "HTTPS address")
	domain    = flag.String("domain", "", "Domain name (required)")
	certDir   = flag.String("certdir", "./certs", "Directory to store Let's Encrypt certificates")
	


	readTimeout     = flag.Duration("read-timeout", 5*time.Second, "Read timeout")
	writeTimeout    = flag.Duration("write-timeout", 10*time.Second, "Write timeout")
	idleTimeout     = flag.Duration("idle-timeout", 120*time.Second, "Idle timeout")
	maxHeaderBytes  = flag.Int("max-header-bytes", 1<<20, "Max header bytes")
	shutdownTimeout = flag.Duration("shutdown-timeout", 30*time.Second, "Shutdown timeout")
)

func main() {
	flag.Parse()

	if *domain == "" {
		log.Fatal("Domain name is required")
	}


	if err := os.MkdirAll(*certDir, 0700); err != nil {
		log.Fatalf("Failed to create cert directory: %v", err)
	}


	proxy := proxy.NewRuntimeMux()
	

	mux := http.NewServeMux()
	secureHandler := securityHeadersMiddleware(mux)
	

	mux.HandleFunc("/", PortfolioHandler)
	mux.Handle("/projects/", http.StripPrefix("/projects", proxy.GetMux()))

	if err := setupProxies(proxy); err != nil {
		log.Fatalf("Failed to setup proxies: %v", err)
	}


	go proxy.CLI()

	certManager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(*domain),
		Cache:      autocert.DirCache(*certDir),
		Email:      "1kirtansoni@gmail.com", 
	}

	httpServer := createHTTPServer(*httpAddr, certManager.HTTPHandler(nil))
	httpsServer := createHTTPSServer(*httpsAddr, secureHandler, certManager)

	serverErrors := make(chan error, 2)
	go func() {
		log.Printf("Starting HTTP server on %s", *httpAddr)
		serverErrors <- httpServer.ListenAndServe()
	}()

	go func() {
		log.Printf("Starting HTTPS server on %s", *httpsAddr)
		serverErrors <- httpsServer.ListenAndServeTLS("", "")
	}()

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrors:
		log.Printf("Server error: %v", err)
	case <-stop:
		log.Println("Shutdown signal received")
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer cancel()

	// Shutdown both servers
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	if err := httpsServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTPS server shutdown error: %v", err)
	}

	log.Println("Servers shutdown completed")
}

func createHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       *readTimeout,
		WriteTimeout:      *writeTimeout,
		IdleTimeout:       *idleTimeout,
		MaxHeaderBytes:    *maxHeaderBytes,
		ReadHeaderTimeout: *readTimeout,
	}
}

func createHTTPSServer(addr string, handler http.Handler, certManager *autocert.Manager) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       *readTimeout,
		WriteTimeout:      *writeTimeout,
		IdleTimeout:       *idleTimeout,
		MaxHeaderBytes:    *maxHeaderBytes,
		ReadHeaderTimeout: *readTimeout,
		TLSConfig:        certManager.TLSConfig(),
	}
}

func setupProxies(proxyProjects *proxy.RuntimeMux) error {
	services := []struct {
		name string
		path string
		url  string
	}{
		{"CodeVis", "/wordsweave", "https://words-weave.com/"},
	}

	for _, s := range services {
		service, err := proxy.NewService(s.name, s.path, s.url)
		if err != nil {
			return err
		}
		if err := proxyProjects.AddProxy(service); err != nil {
			return err
		}
	}
	return nil
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		next.ServeHTTP(w, r)
	})
}
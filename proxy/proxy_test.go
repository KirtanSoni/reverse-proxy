package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestNewRuntimeMux(t *testing.T) {
	mux := NewRuntimeMux()
	if mux.mux == nil {
		t.Error("mux should not be nil")
	}
	if mux.proxyServers == nil {
		t.Error("proxyServers should not be nil")
	}
}

func TestAddProxy(t *testing.T) {
	mux := NewRuntimeMux()
	Service,err := NewService("test","/test","http://www.localhost:8080/") 
	mux.AddProxy(Service)
	if err != nil {
		t.Errorf("addProxy failed: %v", err)
	}
	if _, exists := mux.proxyServers["/test"]; !exists {
		t.Error("proxy should be registered")
	}
}

func TestInvalidURL(t *testing.T) {
	_,err := NewService("test","/test","--calhost-:=-8080/") 
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestRemoveHandler(t *testing.T) {
	mux := NewRuntimeMux()

	Service, _ := NewService("test","/test","http://www.localhost:8080/") 
	mux.AddProxy(Service)
	mux.removeHandler(Service)

	if s, exists := mux.proxyServers["/test"]; exists && s!=nil  {
		t.Error("proxy should be removed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	mux := NewRuntimeMux()
	var wg sync.WaitGroup
	Service, _ := NewService("test","/test","http://www.localhost:8080/") 
	// Concurrent adds and removes
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			
			mux.AddProxy(Service)
		}()
		go func() {
			defer wg.Done()
			mux.removeHandler(Service)
		}()
	}
	wg.Wait()
}

func TestFallbackHandler(t *testing.T) {
	mux := NewRuntimeMux()
	server := httptest.NewServer(mux.mux)
	defer server.Close()
	Service, _ := NewService("test","/test","http://localhost.com" ) 
	mux.AddProxy(Service)
	mux.removeHandler(Service)
	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.StatusCode)
	}
}
func TestUpdateServiceURL(t *testing.T) {
    // Create test backends
    oldBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("old backend"))
    }))
    defer oldBackend.Close()
    
    newBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("new backend"))
    }))
    defer newBackend.Close()

    mux := NewRuntimeMux()

    service, _ := NewService("test", "/test/", oldBackend.URL)
    mux.AddProxy(service)
    
    // Update URL
    service, _ = NewService("test", "/test/", newBackend.URL)
	mux.AddProxy(service)
    // Test routing to new backend
    server := httptest.NewServer(mux.mux)
    defer server.Close()
    
    resp, _ := http.Get(server.URL + "/test/")
    body, _ := io.ReadAll(resp.Body)
    if string(body) != "new backend" {
        t.Error("request not routed to new backend "+string(body))
    }
}
func TestProxyRouting(t *testing.T) {
	// Create a test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend response"))
	}))
	defer backend.Close()

	mux := NewRuntimeMux()

	Service, _ := NewService("test","/test/", backend.URL) 
	mux.AddProxy(Service)

	// Create a test server using our mux
	server := httptest.NewServer(mux.mux)
	defer server.Close()

	// Test proxy routing
	resp, err := http.Get(server.URL + "/test/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.StatusCode)
	}
}


func TestRuntimeMuxRace(t *testing.T) {
	mux := NewRuntimeMux()
	
	service1, _ := NewService("service1", "/api/v1/", "http://localhost:8081")
	service2, _ := NewService("service2", "/api/v2/", "http://localhost:8082")
	

	wg := sync.WaitGroup{}
	wg.Add(4)
	
	go func() {
		defer wg.Done()
		mux.AddProxy(service1)
	}()
	
	go func() {
		defer wg.Done()
		mux.AddProxy(service2)
	}()
	
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			w := httptest.NewRecorder()
			mux.mux.ServeHTTP(w, req)
		}
	}()
	
	// Test concurrent removals and prints
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			mux.removeHandler(service1)
			mux.PrintPaths()
			mux.AddProxy(service1)
		}
	}()
	
	wg.Wait()
}
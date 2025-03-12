package proxy

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
)

type Service struct{
	Name string `json:"name"`
	*httputil.ReverseProxy `json:"-"`
	Path string `json:"path"`
	Url string	`json:"url"`
}

func NewService(name string, Path string,Url string) (*Service, error){
	ServiceURL, err :=  url.Parse(Url)
	if err != nil {
		return nil, errors.New("Service URL invalid" + err.Error())
	}
	return &Service{
		Name:name,
		Path: Path,
		Url: Url,
		ReverseProxy: httputil.NewSingleHostReverseProxy(ServiceURL),
	}, nil
}

func (s *Service) Json()([]byte,error){
	return json.Marshal(s)
}

type RuntimeMux struct{
	sync.RWMutex
	mux          *http.ServeMux
	proxyServers map[string]*Service
	FallbackHandler http.HandlerFunc
}

func (ph *RuntimeMux )GetMux() *http.ServeMux{
	return ph.mux
}

func NewRuntimeMux() *RuntimeMux{
	return &RuntimeMux{
		mux: http.NewServeMux(),
		proxyServers: make(map[string]*Service),
		FallbackHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("fallback: Path not found" + r.URL.Path))
		}),
	}
}

func (ph *RuntimeMux) AddProxy(Service *Service) error{
	ph.Lock()
	defer ph.Unlock()

	_ , exists := ph.proxyServers[Service.Path]
	ph.proxyServers[Service.Path] = Service
	path := Service.Path
	if !exists{
		ph.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			ph.RLock()
			service,exists := ph.proxyServers[path]
			ph.RUnlock()
			if exists && service!=nil {
				service.ServeHTTP(w, r)
				} else{
					ph.FallbackHandler.ServeHTTP(w,r)
				}
			})

		}
	return nil
}

func (ph *RuntimeMux) removeHandler(Service *Service){
	ph.Lock()
	defer ph.Unlock()
	ph.proxyServers[Service.Path] = nil
}

//no tests for this
func (ph *RuntimeMux) PrintPaths(){
	ph.RLock()
	defer ph.RUnlock()

	fmt.Println("===== Services of RunTime Mux ========")
	for  _,service := range ph.proxyServers{
		if service != nil{
			json,_:= service.Json()
			fmt.Println(string(json))
		}
	}
	fmt.Println("======================================")
}

func (ph *RuntimeMux) CLI() {
	fmt.Println("Proxy Management CLI")
	fmt.Println("Available commands: add, remove, list, exit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		cmd := scanner.Text()
		args := strings.Fields(cmd)
		if len(args) == 0 {
			continue
		}

		switch args[0] {
		case "add":
			if len(args) != 4 {
				fmt.Println("Usage: add <name> <path> <url>")
				continue
			}
			service, err := NewService(args[1], args[2], args[3])
			if err != nil {
				fmt.Printf("Error creating service: %v\n", err)
				continue
			}
			if err := ph.AddProxy(service); err != nil {
				fmt.Printf("Error adding proxy: %v\n", err)
				continue
			}
			fmt.Printf("Added service %s at path %s\n", args[1], args[2])

		case "remove":
			if len(args) != 2 {
				fmt.Println("Usage: remove <path>")
				continue
			}
			if service, exists := ph.proxyServers[args[1]]; exists {
				ph.removeHandler(service)
				fmt.Printf("Removed service at path %s\n", args[1])
			} else {
				fmt.Printf("No service found at path %s\n", args[1])
			}

		case "list":
			ph.PrintPaths()

		case "exit":
			return

		default:
			fmt.Println("Unknown command. Available commands: add, remove, list, exit")
		}
	}
}
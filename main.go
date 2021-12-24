package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)




// context data stored with each requesattemptst
const (
    Attempts int = iota
    Retry
)


// a single backend server
type Backend struct {
	URL *url.URL
	Alive bool
	ReverseProxy *httputil.ReverseProxy
	mux sync.RWMutex
}

func (b* Backend) SetAlive(alive bool) {
    b.mux.Lock()
    b.Alive = alive
    b.mux.Unlock()
}

func (b* Backend) IsAlive() (alive bool) {
    b.mux.RLock()
    alive = b.Alive
    b.mux.RUnlock()
    return
}


// pool of backend servers
type ServerPool struct {
	current uint64
	backends []*Backend
}

func (s *ServerPool) AddBackend(backend *Backend) {
    s.backends = append(s.backends, backend)
}

func (s *ServerPool) NextIndex() int {
    return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool) {
	for _, b := range s.backends {
		if backendUrl.String() == b.URL.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (s *ServerPool) GetNextPeer() *Backend {
	next := s.NextIndex() // get the next backend
	l := len(s.backends) + next // start from next and move a full cycle
	for i := next; i < l; i++ {
		idx := i % len(s.backends)
		if s.backends[idx].IsAlive() { // we found a backend to use
			if i != next {
                // store the backend if its not the current server
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}


func GetAttemptsFromContext(r *http.Request) int {
     if attempts, ok := r.Context().Value(Attempts).(int); ok {
         return attempts
     }
     return 1
 }

func GetRetryFromContext(r *http.Request) int {
     if retry, ok := r.Context().Value(Retry).(int); ok {
         return retry
     }
     return 0
 }


// load balance incoming requests
func loadBalancer(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.GetNextPeer()
	if peer != nil {
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}







var serverPool ServerPool

func main() {
    // u, _ := url.Parse("http://localhost:8080")
    // rp := httputil.NewSingleHostReverseProxy(u)
    // proxy := http.HandlerFunc(rp.ServeHTTP)
    var serverList string
    var port int

    flag.StringVar(&serverList, "backends", "", "Comma-separated backends")
    flag.IntVar(&port, "port", 3030, "Port to serve traffic")
    flag.Parse()

    if len(serverList) == 0 {
        log.Fatal("No backends provided...")
    }

    var tokens = strings.Split(serverList, ",")

    for _, tok := range tokens {
        serverUrl, err := url.Parse(tok)
        if err != nil {
            log.Fatal(err)
        }

        proxy := httputil.NewSingleHostReverseProxy(serverUrl)

		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
			log.Printf("[%s] %s\n", serverUrl.Host, e.Error())
			retries := GetRetryFromContext(request)

			if retries < 3 {
				select {
				case <-time.After(10 * time.Millisecond):
					ctx := context.WithValue(request.Context(), Retry, retries + 1)
					proxy.ServeHTTP(writer, request.WithContext(ctx))
				}
				return
			}

			// after 3 failed attempts, set backend status to down
			serverPool.MarkBackendStatus(serverUrl, false)

			attempts := GetAttemptsFromContext(request)
			log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
			ctx := context.WithValue(request.Context(), Attempts, attempts + 1)
			loadBalancer(writer, request.WithContext(ctx))
		}

		serverPool.AddBackend(&Backend {
			URL: serverUrl,
			Alive: true,
			ReverseProxy: proxy,
		})

		log.Printf("Configured server: %s\n", serverUrl)
    }

    // spin up new http server
    server := http.Server {
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(loadBalancer),
	}

    // TODO: health checks
    // go healthCheck()

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

}












package main

import (
    "flag"
    "log"
    "net/http/httputil"
    "net/url"
    "strings"
    "sync"
    "sync/atomic"
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
    }





}
















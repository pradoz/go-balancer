package main

import (
    "net/http/httputil"
    "net/url"
    "sync"
)




// a single backend server
type Backend struct {
	URL *url.URL
	Alive bool
	ReverseProxy *httputil.ReverseProxy
	mux sync.RWMutex
}


// pool of backend servers
type ServerPool struct {
	current uint64
	backends []*Backend
}










func main() {}





package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/google/uuid"
)

type ReverseProxy struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
}

func NewReverseProxy(targetURL string) (*ReverseProxy, error) {
	url, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	// Create Go's built-in reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)

	// Modify request so it is forwarded with correct path and headers
	proxy.Rewrite = func(r *httputil.ProxyRequest) {
		r.SetURL(url)
		r.Out.Header.Set("X-Forwarded-Host", r.In.Header.Get("Host"))

		// Inject & forward Request Correlation ID for distributed logging
		corID := r.In.Header.Get("X-Correlation-ID")
		if corID == "" {
			corID = uuid.New().String()
		}
		r.Out.Header.Set("X-Correlation-ID", corID)
	}

	return &ReverseProxy{
		target: url,
		proxy:  proxy,
	}, nil
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}

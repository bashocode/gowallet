package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type ReverseProxy struct {
	proxy *httputil.ReverseProxy
}

func NewReverseProxy(targetURL string) (*ReverseProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.RequestURI = ""
			pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)

			// Forward correlation ID
			if correlationID := pr.In.Header.Get("X-Correlation-ID"); correlationID != "" {
				pr.Out.Header.Set("X-Correlation-ID", correlationID)
			}

			// Propagate OpenTelemetry trace context to downstream service
			otel.GetTextMapPropagator().Inject(pr.In.Context(), propagation.HeaderCarrier(pr.Out.Header))
		},
	}

	return &ReverseProxy{proxy: rp}, nil
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}

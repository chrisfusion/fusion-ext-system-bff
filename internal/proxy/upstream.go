package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

type ctxKey string

const (
	ctxKeySystemID   ctxKey = "fusion-ext-system-bff/system-id"
	ctxKeySystemName ctxKey = "fusion-ext-system-bff/system-name"
)

// SetSystemContext stores the resolved system identity in the request context
// so the proxy Rewrite function can inject it as trusted upstream headers.
func SetSystemContext(r *http.Request, systemID, systemName string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxKeySystemID, systemID)
	ctx = context.WithValue(ctx, ctxKeySystemName, systemName)
	return r.WithContext(ctx)
}

// UpstreamProxy proxies requests to a single upstream service.
type UpstreamProxy struct {
	rp          *httputil.ReverseProxy
	forcedQuery url.Values // keys are overwritten in every upstream request, ignoring client values
}

// Option configures an UpstreamProxy.
type Option func(*UpstreamProxy)

// WithForcedQuery sets query parameters that are always written into the upstream
// request, overriding any client-supplied values for those keys. Use this to
// lock down public routes to a specific subset of data (e.g. type=streamlit).
func WithForcedQuery(q url.Values) Option {
	return func(p *UpstreamProxy) { p.forcedQuery = q }
}

// NewUpstreamProxy builds a proxy that:
//   - strips stripPrefix from the inbound path before forwarding
//   - injects X-System-ID and X-System-Name from the request context
//   - strips client-supplied identity headers to prevent spoofing
//   - strips CORS headers from upstream responses
func NewUpstreamProxy(baseURL, stripPrefix string, opts ...Option) (*UpstreamProxy, error) {
	target, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing upstream URL %q: %w", baseURL, err)
	}

	p := &UpstreamProxy{}
	for _, opt := range opts {
		opt(p)
	}

	p.rp = &httputil.ReverseProxy{
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Del("Access-Control-Allow-Credentials")
			resp.Header.Del("Access-Control-Allow-Methods")
			resp.Header.Del("Access-Control-Allow-Headers")
			resp.Header.Del("Access-Control-Expose-Headers")
			resp.Header.Del("Access-Control-Max-Age")
			return nil
		},
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)

			path := strings.TrimPrefix(pr.In.URL.Path, stripPrefix)
			if path == "" {
				path = "/"
			} else if path[0] != '/' {
				path = "/" + path
			}
			rawPath := ""
			if pr.In.URL.RawPath != "" {
				rawPath = strings.TrimPrefix(pr.In.URL.RawPath, stripPrefix)
				if rawPath == "" {
					rawPath = "/"
				} else if rawPath[0] != '/' {
					rawPath = "/" + rawPath
				}
			}
			pr.Out.URL.Path = path
			pr.Out.URL.RawPath = rawPath

			// Override forced query params — prevents clients from circumventing
			// restrictions (e.g. using public endpoint to fetch non-streamlit types).
			if len(p.forcedQuery) > 0 {
				q := pr.Out.URL.Query()
				for k, vs := range p.forcedQuery {
					q[k] = vs
				}
				pr.Out.URL.RawQuery = q.Encode()
			}

			// Strip client-supplied identity headers to prevent spoofing.
			pr.Out.Header.Del("X-System-ID")
			pr.Out.Header.Del("X-System-Name")

			if id, ok := pr.In.Context().Value(ctxKeySystemID).(string); ok && id != "" {
				pr.Out.Header.Set("X-System-ID", id)
			}
			if name, ok := pr.In.Context().Value(ctxKeySystemName).(string); ok && name != "" {
				pr.Out.Header.Set("X-System-Name", name)
			}
		},
	}

	return p, nil
}

// Handler returns a gin.HandlerFunc that proxies the request upstream.
func (u *UpstreamProxy) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		u.rp.ServeHTTP(c.Writer, c.Request)
	}
}

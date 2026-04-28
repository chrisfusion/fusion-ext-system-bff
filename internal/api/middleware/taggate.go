package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

// versionPathRe matches public version-scoped paths and captures the artifact ID
// and semver string, e.g. /api/public/index/api/v1/artifacts/42/versions/1.2.3/...
var versionPathRe = regexp.MustCompile(`^/api/public/index/api/v1/artifacts/(\d+)/versions/([^/]+)`)

type versionTagsBody struct {
	Tags []struct {
		Tag string `json:"tag"`
	} `json:"tags"`
}

// TagGate blocks version-scoped requests on the public endpoint unless the
// requested version carries requiredTag. When requiredTag is empty the middleware
// is a no-op and all requests pass through.
func TagGate(indexBaseURL, requiredTag string) gin.HandlerFunc {
	client := &http.Client{Timeout: 5 * time.Second}

	return func(c *gin.Context) {
		if requiredTag == "" {
			c.Next()
			return
		}

		m := versionPathRe.FindStringSubmatch(c.Request.URL.Path)
		if m == nil {
			c.Next()
			return
		}

		artifactID, semver := m[1], m[2]

		preflightURL := fmt.Sprintf("%s/api/v1/artifacts/%s/versions/%s", indexBaseURL, artifactID, semver)
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, preflightURL, nil)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "upstream error"})
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "upstream unavailable"})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if resp.StatusCode != http.StatusOK {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "upstream error"})
			return
		}

		var body versionTagsBody
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "upstream error"})
			return
		}

		for _, t := range body.Tags {
			if t.Tag == requiredTag {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}

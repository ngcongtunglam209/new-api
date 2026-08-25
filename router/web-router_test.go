package router

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A misrouted API client (base URL missing the `/v1` prefix) must get a JSON
// 404, never the SPA shell with HTTP 200: a streaming client that receives
// index.html sees a well-formed 200 whose body carries no events, which
// surfaces as an unexplained truncated stream instead of a routing error.
func TestWebRouterFallbackRejectsNonNavigationMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	indexPage := []byte("<!doctype html><title>new-api</title>")
	newEngine := func() *gin.Engine {
		engine := gin.New()
		SetWebRouter(engine, WebAssets{BuildFS: embed.FS{}, IndexPage: indexPage})
		return engine
	}

	cases := []struct {
		name         string
		method       string
		path         string
		expectStatus int
		expectBody   string
	}{
		{
			name:         "codex responses without v1 prefix",
			method:       http.MethodPost,
			path:         "/responses",
			expectStatus: http.StatusNotFound,
			expectBody:   `"type":"invalid_request_error"`,
		},
		{
			name:         "chat completions without v1 prefix",
			method:       http.MethodPost,
			path:         "/chat/completions",
			expectStatus: http.StatusNotFound,
			expectBody:   `Invalid URL (POST /chat/completions)`,
		},
		{
			name:         "relay path keeps its json 404",
			method:       http.MethodPost,
			path:         "/v1/does-not-exist",
			expectStatus: http.StatusNotFound,
			expectBody:   `Invalid URL (POST /v1/does-not-exist)`,
		},
		{
			name:         "browser navigation still gets the spa shell",
			method:       http.MethodGet,
			path:         "/console/channel",
			expectStatus: http.StatusOK,
			expectBody:   "<!doctype html>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			recorder := httptest.NewRecorder()
			newEngine().ServeHTTP(recorder, req)

			require.Equal(t, tc.expectStatus, recorder.Code)
			assert.True(t, strings.Contains(recorder.Body.String(), tc.expectBody),
				"body %q should contain %q", recorder.Body.String(), tc.expectBody)
		})
	}
}

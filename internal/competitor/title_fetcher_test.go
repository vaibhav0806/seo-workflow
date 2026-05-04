package competitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTitlePrefersOGTitle(t *testing.T) {
	html := `<html><head><title>Fallback Title</title><meta property="og:title" content="Coding Agents · AI Gateway"></head></html>`

	title, err := extractPageTitle(strings.NewReader(html))

	require.NoError(t, err)
	require.Equal(t, "Coding Agents · AI Gateway", title)
}

func TestExtractTitleFallsBackToTitleTag(t *testing.T) {
	html := `<html><head><title>Claude Code · AI Gateway</title></head></html>`

	title, err := extractPageTitle(strings.NewReader(html))

	require.NoError(t, err)
	require.Equal(t, "Claude Code · AI Gateway", title)
}

func TestTitleFetcherReturnsEmptyOnHTTPError(t *testing.T) {
	fetcher := &TitleFetcher{
		httpClient: &http.Client{Transport: titleStubTransport{
			status: http.StatusTooManyRequests,
			body:   "rate limited",
		}},
	}

	title, err := fetcher.FetchTitle(context.Background(), "https://vercel.com/docs/ai-gateway/coding-agents")

	require.NoError(t, err)
	require.Empty(t, title)
}

type titleStubTransport struct {
	status int
	body   string
}

func (t titleStubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == "" {
		return nil, fmt.Errorf("empty url")
	}
	return &http.Response{
		StatusCode: t.status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Request:    req,
	}, nil
}

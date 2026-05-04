package competitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const maxTitleHTMLBytes = 1 << 20

type TitleFetcher struct {
	httpClient *http.Client
}

func NewTitleFetcher(timeoutSecs int) *TitleFetcher {
	return &TitleFetcher{httpClient: &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}}
}

func (f *TitleFetcher) FetchTitle(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(pageURL), nil)
	if err != nil {
		return "", fmt.Errorf("build title request: %w", err)
	}
	req.Header.Set("User-Agent", "seo-workflow/0.1")

	client := f.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch title: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil
	}

	title, err := extractPageTitle(io.LimitReader(resp.Body, maxTitleHTMLBytes))
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(title), nil
}

func extractPageTitle(r io.Reader) (string, error) {
	z := html.NewTokenizer(r)
	var title string

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return strings.TrimSpace(title), nil
			}
			return "", z.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			switch strings.ToLower(tok.Data) {
			case "meta":
				if isOGTitle(tok) {
					if content := attrValue(tok, "content"); content != "" {
						return strings.TrimSpace(content), nil
					}
				}
			case "title":
				if title == "" {
					text, err := readTitleText(z)
					if err != nil {
						return "", err
					}
					title = text
				}
			}
		}
	}
}

func readTitleText(z *html.Tokenizer) (string, error) {
	var b strings.Builder
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return b.String(), nil
			}
			return "", z.Err()
		case html.TextToken:
			b.WriteString(z.Token().Data)
		case html.EndTagToken:
			if strings.EqualFold(z.Token().Data, "title") {
				return b.String(), nil
			}
		}
	}
}

func isOGTitle(tok html.Token) bool {
	return strings.EqualFold(attrValue(tok, "property"), "og:title") || strings.EqualFold(attrValue(tok, "name"), "og:title")
}

func attrValue(tok html.Token, key string) string {
	for _, attr := range tok.Attr {
		if strings.EqualFold(attr.Key, key) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

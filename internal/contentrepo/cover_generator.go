package contentrepo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openRouterChatEndpoint = "https://openrouter.ai/api/v1/chat/completions"

type GeneratedCover struct {
	URL   string
	Asset CoverAsset
}

type openRouterCoverRequest struct {
	Model       string                `json:"model"`
	Messages    []openRouterMessage   `json:"messages"`
	Modalities  []string              `json:"modalities"`
	Temperature float64               `json:"temperature,omitempty"`
	Stream      bool                  `json:"stream"`
	ImageConfig openRouterImageConfig `json:"image_config,omitempty"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterImageConfig struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
}

type openRouterCoverResponse struct {
	Choices []struct {
		Message struct {
			Images []struct {
				Type     string `json:"type"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"images"`
		} `json:"message"`
	} `json:"choices"`
}

func GenerateOpenRouterCover(ctx context.Context, apiKey string, model string, post BlogPost, assetBaseURL string) (GeneratedCover, error) {
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	assetBaseURL = strings.TrimRight(strings.TrimSpace(assetBaseURL), "/")
	if apiKey == "" || model == "" {
		return GeneratedCover{}, nil
	}
	if assetBaseURL == "" {
		return GeneratedCover{}, fmt.Errorf("CONTENT_COVER_ASSET_BASE_URL is required when OPENROUTER_COVER_MODEL is set")
	}

	request := openRouterCoverRequest{
		Model: model,
		Messages: []openRouterMessage{
			{Role: "user", Content: coverPrompt(post)},
		},
		Modalities:  []string{"image", "text"},
		Temperature: 0.4,
		Stream:      false,
		ImageConfig: openRouterImageConfig{AspectRatio: "16:9"},
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return GeneratedCover{}, fmt.Errorf("marshal openrouter cover request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterChatEndpoint, bytes.NewReader(payload))
	if err != nil {
		return GeneratedCover{}, fmt.Errorf("build openrouter cover request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return GeneratedCover{}, fmt.Errorf("execute openrouter cover request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return GeneratedCover{}, fmt.Errorf("openrouter cover status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openRouterCoverResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return GeneratedCover{}, fmt.Errorf("decode openrouter cover response: %w", err)
	}
	dataURL := firstOpenRouterImageDataURL(parsed)
	if dataURL == "" {
		return GeneratedCover{}, fmt.Errorf("openrouter cover response had no image data")
	}
	mimeType, content, err := decodeDataURL(dataURL)
	if err != nil {
		return GeneratedCover{}, err
	}
	assetPath := "covers/" + post.Slug + extensionForMime(mimeType)
	return GeneratedCover{
		URL: assetBaseURL + "/" + assetPath,
		Asset: CoverAsset{
			Path:    assetPath,
			Content: content,
		},
	}, nil
}

func firstOpenRouterImageDataURL(response openRouterCoverResponse) string {
	for _, choice := range response.Choices {
		for _, image := range choice.Message.Images {
			url := strings.TrimSpace(image.ImageURL.URL)
			if strings.HasPrefix(url, "data:image/") {
				return url
			}
		}
	}
	return ""
}

func decodeDataURL(dataURL string) (string, []byte, error) {
	const marker = ";base64,"
	header, encoded, ok := strings.Cut(strings.TrimSpace(dataURL), marker)
	if !ok || !strings.HasPrefix(header, "data:") || encoded == "" {
		return "", nil, fmt.Errorf("unsupported openrouter cover image data URL")
	}
	mimeType := strings.TrimPrefix(header, "data:")
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", nil, fmt.Errorf("decode openrouter cover image: %w", err)
	}
	return mimeType, content, nil
}

func coverPrompt(post BlogPost) string {
	return strings.Join([]string{
		"Create a 1200x630 editorial blog cover image for CreateOS.",
		"Visual direction: unified intelligent workspace, ideas moving from concept to deployed application, execution layer, clean product-led SaaS aesthetic.",
		"Do not include readable text, logos, UI screenshots, robots, generic circuit brains, or stock-photo people.",
		"Use a distinctive but professional composition suitable for a technical startup blog.",
		"Article title: " + post.Title,
		"Description: " + post.Description,
	}, "\n")
}

func extensionForMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

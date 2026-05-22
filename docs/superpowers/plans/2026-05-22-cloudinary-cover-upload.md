# Cloudinary Cover Upload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upload OpenRouter-generated cover images to Cloudinary and write the returned public HTTPS URL into generated blog frontmatter.

**Architecture:** Keep OpenRouter image generation responsible only for producing image bytes and a local asset path. Add a small `CoverUploader` boundary in `internal/contentrepo`, then make `cmd/worker/content_pr.go` choose Cloudinary upload when configured and fall back to the existing repo-asset behavior or default cover URL. Config and docs expose the new Cloudinary credentials without making them required.

**Tech Stack:** Go, standard `net/http` multipart uploads, Cloudinary Upload API, existing `testify/require` tests.

---

## File Structure

- Create `internal/contentrepo/cover_uploader.go`: shared uploader interface, upload result type, and `CoverUploadAsset` input type.
- Create `internal/contentrepo/cloudinary_uploader.go`: Cloudinary signed upload implementation using Basic Auth and multipart form fields.
- Create `internal/contentrepo/cloudinary_uploader_test.go`: request-shape, response parsing, and error tests.
- Modify `internal/config/config.go`: add Cloudinary config fields and load env vars in competitor mode.
- Modify `internal/config/config_test.go`: assert Cloudinary overrides load.
- Modify `cmd/worker/content_pr.go`: route generated cover bytes through Cloudinary when configured, otherwise preserve current asset-base fallback.
- Modify `cmd/worker/content_pr_test.go`: test cover selection without live OpenRouter, Cloudinary, or GitHub calls.
- Modify `docs/competitor-oneshot-workflow.md`: document Cloudinary setup and private GitHub raw URL caveat.
- Modify `scripts/setup-env.sh`: prompt/write Cloudinary env vars.

---

### Task 1: Cloudinary Uploader

**Files:**
- Create: `internal/contentrepo/cover_uploader.go`
- Create: `internal/contentrepo/cloudinary_uploader.go`
- Test: `internal/contentrepo/cloudinary_uploader_test.go`

- [ ] **Step 1: Write the failing Cloudinary uploader tests**

Create `internal/contentrepo/cloudinary_uploader_test.go`:

```go
package contentrepo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudinaryCoverUploaderUploadsImageAndReturnsSecureURL(t *testing.T) {
	var capturedAuth string
	var capturedFields map[string]string
	var capturedFile []byte
	var capturedFileContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1_1/demo-cloud/image/upload", r.URL.Path)
		capturedAuth = r.Header.Get("Authorization")

		reader, err := r.MultipartReader()
		require.NoError(t, err)
		capturedFields = map[string]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			body, err := io.ReadAll(part)
			require.NoError(t, err)
			if part.FormName() == "file" {
				capturedFile = body
				capturedFileContentType = part.Header.Get("Content-Type")
				continue
			}
			capturedFields[part.FormName()] = string(body)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"secure_url": "https://res.cloudinary.com/demo-cloud/image/upload/v123/createos/blog-covers/test-post.png",
			"public_id":  "createos/blog-covers/test-post",
			"format":     "png",
			"bytes":      7,
		})
	}))
	defer server.Close()

	uploader := NewCloudinaryCoverUploader("demo-cloud", "api-key", "api-secret", "createos/blog-covers")
	uploader.apiBaseURL = server.URL
	uploader.httpClient = server.Client()

	result, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Path:        "covers/test-post.png",
		Content:     []byte("pngdata"),
		ContentType: "image/png",
		Slug:        "test-post",
	})

	require.NoError(t, err)
	require.Equal(t, "https://res.cloudinary.com/demo-cloud/image/upload/v123/createos/blog-covers/test-post.png", result.URL)
	require.Equal(t, "createos/blog-covers/test-post", result.PublicID)
	require.Equal(t, "png", result.Format)
	require.Equal(t, int64(7), result.Bytes)
	require.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("api-key:api-secret")), capturedAuth)
	require.Equal(t, []byte("pngdata"), capturedFile)
	require.Equal(t, "image/png", capturedFileContentType)
	require.Equal(t, "createos/blog-covers", capturedFields["folder"])
	require.Equal(t, "test-post", capturedFields["public_id"])
	require.Equal(t, "true", capturedFields["overwrite"])
	require.NotContains(t, capturedFields, "type")
}

func TestCloudinaryCoverUploaderRejectsMissingCredentials(t *testing.T) {
	uploader := NewCloudinaryCoverUploader("", "api-key", "api-secret", "createos/blog-covers")

	result, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Content: []byte("pngdata"),
		Slug:    "test-post",
	})

	require.Empty(t, result)
	require.EqualError(t, err, "cloudinary cloud name, api key, and api secret are required")
}

func TestCloudinaryCoverUploaderReturnsStatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad upload", http.StatusUnauthorized)
	}))
	defer server.Close()

	uploader := NewCloudinaryCoverUploader("demo-cloud", "api-key", "api-secret", "createos/blog-covers")
	uploader.apiBaseURL = server.URL
	uploader.httpClient = server.Client()

	result, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Content: []byte("pngdata"),
		Slug:    "test-post",
	})

	require.Empty(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cloudinary upload status=401")
	require.Contains(t, err.Error(), "bad upload")
}

func TestCloudinaryCoverUploaderRejectsEmptySecureURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"public_id":"createos/blog-covers/test-post"}`))
	}))
	defer server.Close()

	uploader := NewCloudinaryCoverUploader("demo-cloud", "api-key", "api-secret", "createos/blog-covers")
	uploader.apiBaseURL = server.URL
	uploader.httpClient = server.Client()

	result, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Content: []byte("pngdata"),
		Slug:    "test-post",
	})

	require.Empty(t, result)
	require.EqualError(t, err, "cloudinary upload response missing secure_url")
}

func TestCloudinaryCoverUploaderDefaultsFolderAndSlug(t *testing.T) {
	var fields map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields = readMultipartFields(t, r)
		_, _ = w.Write([]byte(`{"secure_url":"https://cdn.example.com/cover.jpg"}`))
	}))
	defer server.Close()

	uploader := NewCloudinaryCoverUploader("demo-cloud", "api-key", "api-secret", "")
	uploader.apiBaseURL = server.URL
	uploader.httpClient = server.Client()

	_, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Path:    "covers/Fancy Post.jpg",
		Content: []byte("jpgdata"),
		Slug:    "fancy-post",
	})

	require.NoError(t, err)
	require.Equal(t, "createos/blog-covers", fields["folder"])
	require.Equal(t, "fancy-post", fields["public_id"])
	require.Equal(t, "true", fields["overwrite"])
}

func readMultipartFields(t *testing.T, r *http.Request) map[string]string {
	t.Helper()
	require.True(t, strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"))
	reader, err := r.MultipartReader()
	require.NoError(t, err)
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		body, err := io.ReadAll(part)
		require.NoError(t, err)
		if part.FormName() != "file" {
			fields[part.FormName()] = string(body)
		}
	}
	return fields
}

var _ = multipart.ErrMessageTooLarge
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run:

```bash
go test ./internal/contentrepo -run Cloudinary -count=1
```

Expected: FAIL with undefined symbols such as `NewCloudinaryCoverUploader` and `CoverUploadAsset`.

- [ ] **Step 3: Add uploader types and Cloudinary implementation**

Create `internal/contentrepo/cover_uploader.go`:

```go
package contentrepo

import "context"

type CoverUploadAsset struct {
	Path        string
	Content     []byte
	ContentType string
	Slug        string
}

type CoverUploadResult struct {
	URL      string
	PublicID string
	Format   string
	Bytes    int64
}

type CoverUploader interface {
	UploadCover(ctx context.Context, asset CoverUploadAsset) (CoverUploadResult, error)
}
```

Create `internal/contentrepo/cloudinary_uploader.go`:

```go
package contentrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

const defaultCloudinaryAPIBaseURL = "https://api.cloudinary.com"
const defaultCloudinaryUploadFolder = "createos/blog-covers"

type CloudinaryCoverUploader struct {
	httpClient *http.Client
	apiBaseURL string
	cloudName  string
	apiKey     string
	apiSecret  string
	folder     string
}

type cloudinaryUploadResponse struct {
	SecureURL string `json:"secure_url"`
	PublicID  string `json:"public_id"`
	Format    string `json:"format"`
	Bytes     int64  `json:"bytes"`
}

func NewCloudinaryCoverUploader(cloudName string, apiKey string, apiSecret string, folder string) *CloudinaryCoverUploader {
	folder = strings.Trim(strings.TrimSpace(folder), "/")
	if folder == "" {
		folder = defaultCloudinaryUploadFolder
	}
	return &CloudinaryCoverUploader{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		apiBaseURL: defaultCloudinaryAPIBaseURL,
		cloudName:  strings.TrimSpace(cloudName),
		apiKey:     strings.TrimSpace(apiKey),
		apiSecret:  strings.TrimSpace(apiSecret),
		folder:     folder,
	}
}

func (uploader *CloudinaryCoverUploader) UploadCover(ctx context.Context, asset CoverUploadAsset) (CoverUploadResult, error) {
	if uploader.cloudName == "" || uploader.apiKey == "" || uploader.apiSecret == "" {
		return CoverUploadResult{}, fmt.Errorf("cloudinary cloud name, api key, and api secret are required")
	}
	if len(asset.Content) == 0 {
		return CoverUploadResult{}, fmt.Errorf("cover upload asset content is required")
	}
	slug := strings.TrimSpace(asset.Slug)
	if slug == "" {
		slug = slugFromCoverPath(asset.Path)
	}
	if slug == "" {
		return CoverUploadResult{}, fmt.Errorf("cover upload asset slug is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writeMultipartFile(writer, "file", asset.Path, asset.ContentType, asset.Content); err != nil {
		return CoverUploadResult{}, err
	}
	fields := map[string]string{
		"folder":    uploader.folder,
		"public_id": slug,
		"overwrite": "true",
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return CoverUploadResult{}, fmt.Errorf("write cloudinary multipart field %q: %w", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return CoverUploadResult{}, fmt.Errorf("close cloudinary multipart body: %w", err)
	}

	requestURL := strings.TrimRight(uploader.apiBaseURL, "/") + "/v1_1/" + uploader.cloudName + "/image/upload"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, &body)
	if err != nil {
		return CoverUploadResult{}, fmt.Errorf("build cloudinary upload request: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.SetBasicAuth(uploader.apiKey, uploader.apiSecret)

	client := uploader.httpClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return CoverUploadResult{}, fmt.Errorf("execute cloudinary upload request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return CoverUploadResult{}, fmt.Errorf("cloudinary upload status=%d body=%q", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var parsed cloudinaryUploadResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return CoverUploadResult{}, fmt.Errorf("decode cloudinary upload response: %w", err)
	}
	if strings.TrimSpace(parsed.SecureURL) == "" {
		return CoverUploadResult{}, fmt.Errorf("cloudinary upload response missing secure_url")
	}
	return CoverUploadResult{
		URL:      strings.TrimSpace(parsed.SecureURL),
		PublicID: strings.TrimSpace(parsed.PublicID),
		Format:   strings.TrimSpace(parsed.Format),
		Bytes:    parsed.Bytes,
	}, nil
}

func writeMultipartFile(writer *multipart.Writer, fieldName string, path string, contentType string, content []byte) error {
	fileName := strings.TrimSpace(path)
	if fileName == "" {
		fileName = "cover"
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, escapeMultipartQuote(fileName)))
	if strings.TrimSpace(contentType) != "" {
		header.Set("Content-Type", strings.TrimSpace(contentType))
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create cloudinary multipart file: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("write cloudinary multipart file: %w", err)
	}
	return nil
}

func escapeMultipartQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func slugFromCoverPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.TrimPrefix(path, "covers/")
	if dot := strings.LastIndex(path, "."); dot > 0 {
		path = path[:dot]
	}
	return slugify(path)
}
```

Cloudinary `type` is the delivery type, not the MIME type. Do not send a multipart form field named `type`; when `CoverUploadAsset.ContentType` is set, carry it only on the `file` part `Content-Type` header.

- [ ] **Step 4: Run Cloudinary tests to verify they pass**

Run:

```bash
go test ./internal/contentrepo -run Cloudinary -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

Run:

```bash
git add internal/contentrepo/cover_uploader.go internal/contentrepo/cloudinary_uploader.go internal/contentrepo/cloudinary_uploader_test.go
git commit -m "Add Cloudinary cover uploader"
```

---

### Task 2: Config Loading

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing config assertions**

In `internal/config/config_test.go`, update `TestLoadCompetitorModeSuccessDefaults` to include:

```go
require.Empty(t, cfg.CloudinaryCloudName)
require.Empty(t, cfg.CloudinaryAPIKey)
require.Empty(t, cfg.CloudinaryAPISecret)
require.Equal(t, "createos/blog-covers", cfg.CloudinaryUploadFolder)
```

In `TestLoadCompetitorModeAllowsOverrides`, add env setup before `cfg, err := Load()`:

```go
t.Setenv("CLOUDINARY_CLOUD_NAME", "demo-cloud")
t.Setenv("CLOUDINARY_API_KEY", "cloudinary-key")
t.Setenv("CLOUDINARY_API_SECRET", "cloudinary-secret")
t.Setenv("CLOUDINARY_UPLOAD_FOLDER", "custom/covers")
```

Then add assertions after the existing cover model assertion:

```go
require.Equal(t, "demo-cloud", cfg.CloudinaryCloudName)
require.Equal(t, "cloudinary-key", cfg.CloudinaryAPIKey)
require.Equal(t, "cloudinary-secret", cfg.CloudinaryAPISecret)
require.Equal(t, "custom/covers", cfg.CloudinaryUploadFolder)
```

- [ ] **Step 2: Run config tests to verify they fail**

Run:

```bash
go test ./internal/config -run 'CompetitorMode' -count=1
```

Expected: FAIL with undefined `Config` fields.

- [ ] **Step 3: Add config fields and env loading**

In `internal/config/config.go`, add this default next to the other defaults:

```go
defaultCloudinaryUploadFolder = "createos/blog-covers"
```

Add fields to `Config`:

```go
CloudinaryCloudName    string
CloudinaryAPIKey       string
CloudinaryAPISecret    string
CloudinaryUploadFolder string
```

Set the default in the `cfg := &Config{...}` literal:

```go
CloudinaryUploadFolder: defaultCloudinaryUploadFolder,
```

In the `oneshot-competitor` case, after `OPENROUTER_COVER_MODEL` loading, add:

```go
cfg.CloudinaryCloudName = strings.TrimSpace(os.Getenv("CLOUDINARY_CLOUD_NAME"))
cfg.CloudinaryAPIKey = strings.TrimSpace(os.Getenv("CLOUDINARY_API_KEY"))
cfg.CloudinaryAPISecret = strings.TrimSpace(os.Getenv("CLOUDINARY_API_SECRET"))
if folder := strings.Trim(strings.TrimSpace(os.Getenv("CLOUDINARY_UPLOAD_FOLDER")), "/"); folder != "" {
	cfg.CloudinaryUploadFolder = folder
}
```

- [ ] **Step 4: Run config tests to verify they pass**

Run:

```bash
go test ./internal/config -run 'CompetitorMode' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

Run:

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "Load Cloudinary cover upload config"
```

---

### Task 3: Worker Cover Selection

**Files:**
- Modify: `cmd/worker/content_pr.go`
- Test: `cmd/worker/content_pr_test.go`

- [ ] **Step 1: Write failing worker selection tests**

Append to `cmd/worker/content_pr_test.go`:

```go
func TestApplyGeneratedCoverUploadsToCloudinaryWhenConfigured(t *testing.T) {
	post := contentrepo.BlogPost{
		Slug:  "test-post",
		Cover: "https://example.com/default.png",
	}
	cover := contentrepo.GeneratedCover{
		URL: "https://private.example.com/covers/test-post.png",
		Asset: contentrepo.CoverAsset{
			Path:    "covers/test-post.png",
			Content: []byte("pngdata"),
		},
	}
	uploader := stubCoverUploader{
		result: contentrepo.CoverUploadResult{URL: "https://res.cloudinary.com/demo/image/upload/test-post.png"},
	}

	assets := applyGeneratedCover(context.Background(), &post, cover, uploader)

	require.Empty(t, assets)
	require.Equal(t, "https://res.cloudinary.com/demo/image/upload/test-post.png", post.Cover)
	require.Equal(t, "test-post", uploader.asset.Slug)
	require.Equal(t, "covers/test-post.png", uploader.asset.Path)
	require.Equal(t, []byte("pngdata"), uploader.asset.Content)
	require.Equal(t, "image/png", uploader.asset.ContentType)
}

func TestApplyGeneratedCoverFallsBackToRepoAssetURLWithoutUploader(t *testing.T) {
	post := contentrepo.BlogPost{
		Slug:  "test-post",
		Cover: "https://example.com/default.png",
	}
	cover := contentrepo.GeneratedCover{
		URL: "https://cdn.example.com/covers/test-post.png",
		Asset: contentrepo.CoverAsset{
			Path:    "covers/test-post.png",
			Content: []byte("pngdata"),
		},
	}

	assets := applyGeneratedCover(context.Background(), &post, cover, nil)

	require.Equal(t, "https://cdn.example.com/covers/test-post.png", post.Cover)
	require.Len(t, assets, 1)
	require.Equal(t, "covers/test-post.png", assets[0].Path)
}

func TestApplyGeneratedCoverKeepsDefaultWhenUploaderFails(t *testing.T) {
	post := contentrepo.BlogPost{
		Slug:  "test-post",
		Cover: "https://example.com/default.png",
	}
	cover := contentrepo.GeneratedCover{
		URL: "https://cdn.example.com/covers/test-post.png",
		Asset: contentrepo.CoverAsset{
			Path:    "covers/test-post.png",
			Content: []byte("pngdata"),
		},
	}
	uploader := stubCoverUploader{err: errors.New("upload failed")}

	assets := applyGeneratedCover(context.Background(), &post, cover, uploader)

	require.Empty(t, assets)
	require.Equal(t, "https://example.com/default.png", post.Cover)
}

func TestNewCoverUploaderFromConfigRequiresAllCloudinaryCredentials(t *testing.T) {
	cfg := &config.Config{
		CloudinaryCloudName:    "demo-cloud",
		CloudinaryAPIKey:       "cloudinary-key",
		CloudinaryAPISecret:    "",
		CloudinaryUploadFolder: "createos/blog-covers",
	}

	require.Nil(t, newCoverUploaderFromConfig(cfg))

	cfg.CloudinaryAPISecret = "cloudinary-secret"
	require.NotNil(t, newCoverUploaderFromConfig(cfg))
}

type stubCoverUploader struct {
	result contentrepo.CoverUploadResult
	err    error
	asset  contentrepo.CoverUploadAsset
}

func (uploader *stubCoverUploader) UploadCover(ctx context.Context, asset contentrepo.CoverUploadAsset) (contentrepo.CoverUploadResult, error) {
	uploader.asset = asset
	return uploader.result, uploader.err
}
```

Add imports to `cmd/worker/content_pr_test.go`:

```go
import (
	"context"
	"errors"
	"testing"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/nodeops/seo-workflow/internal/contentrepo"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Run worker tests to verify they fail**

Run:

```bash
go test ./cmd/worker -run 'ApplyGeneratedCover|NewCoverUploaderFromConfig|FirstDraftRecommendation' -count=1
```

Expected: FAIL with undefined `applyGeneratedCover` and `newCoverUploaderFromConfig`.

- [ ] **Step 3: Implement cover uploader selection and fallback**

In `cmd/worker/content_pr.go`, replace the current generated-cover handling block with:

```go
coverAssets := []contentrepo.CoverAsset{}
if strings.TrimSpace(cfg.OpenRouterAPIKey) != "" && strings.TrimSpace(cfg.OpenRouterCoverModel) != "" {
	cover, coverErr := contentrepo.GenerateOpenRouterCover(ctx, cfg.OpenRouterAPIKey, cfg.OpenRouterCoverModel, post, cfg.ContentCoverAssetBaseURL)
	if coverErr != nil {
		log.Printf("competitor cover image generation skipped: %v", coverErr)
	} else {
		coverAssets = applyGeneratedCover(ctx, &post, cover, newCoverUploaderFromConfig(cfg))
	}
}
```

Then add these helper functions below `firstDraftRecommendation`:

```go
func newCoverUploaderFromConfig(cfg *config.Config) contentrepo.CoverUploader {
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.CloudinaryCloudName) == "" || strings.TrimSpace(cfg.CloudinaryAPIKey) == "" || strings.TrimSpace(cfg.CloudinaryAPISecret) == "" {
		return nil
	}
	return contentrepo.NewCloudinaryCoverUploader(cfg.CloudinaryCloudName, cfg.CloudinaryAPIKey, cfg.CloudinaryAPISecret, cfg.CloudinaryUploadFolder)
}

func applyGeneratedCover(ctx context.Context, post *contentrepo.BlogPost, cover contentrepo.GeneratedCover, uploader contentrepo.CoverUploader) []contentrepo.CoverAsset {
	if post == nil {
		return nil
	}
	if uploader != nil && len(cover.Asset.Content) > 0 {
		result, err := uploader.UploadCover(ctx, contentrepo.CoverUploadAsset{
			Path:        cover.Asset.Path,
			Content:     cover.Asset.Content,
			ContentType: contentTypeFromAssetPath(cover.Asset.Path),
			Slug:        post.Slug,
		})
		if err != nil {
			log.Printf("competitor cover image upload skipped: %v", err)
			return nil
		}
		if strings.TrimSpace(result.URL) != "" {
			post.Cover = strings.TrimSpace(result.URL)
			log.Printf("competitor cover image uploaded: public_id=%q url=%q", result.PublicID, result.URL)
			return nil
		}
	}
	if strings.TrimSpace(cover.URL) != "" {
		post.Cover = cover.URL
		log.Printf("competitor cover image generated: path=%q url=%q", cover.Asset.Path, cover.URL)
	}
	if strings.TrimSpace(cover.Asset.Path) == "" || len(cover.Asset.Content) == 0 {
		return nil
	}
	return []contentrepo.CoverAsset{cover.Asset}
}

func contentTypeFromAssetPath(path string) string {
	switch strings.ToLower(strings.TrimSpace(path)) {
	case "":
		return ""
	default:
		if strings.HasSuffix(strings.ToLower(path), ".jpg") || strings.HasSuffix(strings.ToLower(path), ".jpeg") {
			return "image/jpeg"
		}
		if strings.HasSuffix(strings.ToLower(path), ".webp") {
			return "image/webp"
		}
		if strings.HasSuffix(strings.ToLower(path), ".png") {
			return "image/png"
		}
		return ""
	}
}
```

- [ ] **Step 4: Run worker tests to verify they pass**

Run:

```bash
go test ./cmd/worker -run 'ApplyGeneratedCover|NewCoverUploaderFromConfig|FirstDraftRecommendation' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run contentrepo tests because worker now depends on Cloudinary uploader types**

Run:

```bash
go test ./internal/contentrepo -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 3**

Run:

```bash
git add cmd/worker/content_pr.go cmd/worker/content_pr_test.go
git commit -m "Upload generated covers before publishing content PRs"
```

---

### Task 4: Docs and Setup Script

**Files:**
- Modify: `docs/competitor-oneshot-workflow.md`
- Modify: `scripts/setup-env.sh`

- [ ] **Step 1: Update workflow docs**

In `docs/competitor-oneshot-workflow.md`, replace the generated cover env section with:

```markdown
# optional generated cover image model
export OPENROUTER_COVER_MODEL='google/gemini-2.5-flash-image'

# recommended public image hosting for generated covers
# Cloudinary has a free plan; create a cloud and API key, then set:
export CLOUDINARY_CLOUD_NAME='your-cloud-name'
export CLOUDINARY_API_KEY='123456789'
export CLOUDINARY_API_SECRET='cloudinary-secret'
export CLOUDINARY_UPLOAD_FOLDER='createos/blog-covers'

# legacy fallback only: use when files committed under covers/ in the content
# repo are served by a public CDN. Do not point this at private GitHub raw URLs.
export CONTENT_COVER_ASSET_BASE_URL='https://public-cdn.example.com'
```

In the output section, replace:

```markdown
- Generated cover assets are committed under `covers/` when cover generation succeeds.
```

with:

```markdown
- Generated cover images are uploaded to Cloudinary when Cloudinary env vars are configured.
- Without Cloudinary, generated cover assets are committed under `covers/` only when `CONTENT_COVER_ASSET_BASE_URL` points at a public CDN.
```

- [ ] **Step 2: Update setup script prompts and env output**

In `scripts/setup-env.sh`, after the `CONTENT_COVER_ASSET_BASE_URL` prompt, add:

```bash
  prompt_default CLOUDINARY_CLOUD_NAME "Cloudinary cloud name (optional)" "${CLOUDINARY_CLOUD_NAME:-}"
  prompt_default CLOUDINARY_API_KEY "Cloudinary API key (optional)" "${CLOUDINARY_API_KEY:-}"
  prompt_secret CLOUDINARY_API_SECRET "Cloudinary API secret (optional; press Enter to leave empty)"
  prompt_default CLOUDINARY_UPLOAD_FOLDER "Cloudinary upload folder" "createos/blog-covers"
```

In the generated competitor `.env` block, after `CONTENT_COVER_ASSET_BASE_URL=${CONTENT_COVER_ASSET_BASE_URL:-}`, add:

```bash
CLOUDINARY_CLOUD_NAME=${CLOUDINARY_CLOUD_NAME:-}
CLOUDINARY_API_KEY=${CLOUDINARY_API_KEY:-}
CLOUDINARY_API_SECRET=${CLOUDINARY_API_SECRET:-}
CLOUDINARY_UPLOAD_FOLDER=${CLOUDINARY_UPLOAD_FOLDER:-createos/blog-covers}
```

- [ ] **Step 3: Run formatting or shell syntax checks**

Run:

```bash
bash -n scripts/setup-env.sh
```

Expected: no output and exit code 0.

- [ ] **Step 4: Commit Task 4**

Run:

```bash
git add docs/competitor-oneshot-workflow.md scripts/setup-env.sh
git commit -m "Document Cloudinary cover upload setup"
```

---

### Task 5: Full Verification

**Files:**
- Verify all modified Go files and docs.

- [ ] **Step 1: Run Go formatting**

Run:

```bash
gofmt -w internal/contentrepo/cover_uploader.go internal/contentrepo/cloudinary_uploader.go internal/contentrepo/cloudinary_uploader_test.go internal/config/config.go internal/config/config_test.go cmd/worker/content_pr.go cmd/worker/content_pr_test.go
```

Expected: files are formatted with no command output.

- [ ] **Step 2: Run focused test packages**

Run:

```bash
go test ./internal/contentrepo ./internal/config ./cmd/worker -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full test suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Inspect final diff**

Run:

```bash
git status --short
git diff --stat
```

Expected: only intentional Cloudinary implementation, docs, and setup script changes remain uncommitted after the task commits. Existing unrelated untracked files may still appear and should not be modified.

- [ ] **Step 5: Final implementation commit if Task 5 produced formatting-only changes**

If `gofmt` or verification produced changes after Task 4, commit them:

```bash
git add internal/contentrepo/cover_uploader.go internal/contentrepo/cloudinary_uploader.go internal/contentrepo/cloudinary_uploader_test.go internal/config/config.go internal/config/config_test.go cmd/worker/content_pr.go cmd/worker/content_pr_test.go docs/competitor-oneshot-workflow.md scripts/setup-env.sh
git commit -m "Verify Cloudinary cover upload workflow"
```

If `git status --short` shows no tracked modified files from this feature, do not create an empty commit.

---

## Self-Review

- Spec coverage: Cloudinary upload, public `secure_url`, fallback rules, config, tests, and docs are each covered by Tasks 1-5.
- Placeholder scan: The plan intentionally contains no unresolved placeholders or undefined future work.
- Type consistency: `CoverUploadAsset`, `CoverUploadResult`, `CoverUploader`, `NewCloudinaryCoverUploader`, `applyGeneratedCover`, and config field names are consistent across tasks.

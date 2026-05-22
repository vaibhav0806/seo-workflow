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
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	defaultCloudinaryAPIBaseURL   = "https://api.cloudinary.com"
	defaultCloudinaryUploadFolder = "createos/blog-covers"
)

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

func NewCloudinaryCoverUploader(cloudName, apiKey, apiSecret, folder string) *CloudinaryCoverUploader {
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
	if strings.TrimSpace(uploader.cloudName) == "" || strings.TrimSpace(uploader.apiKey) == "" || strings.TrimSpace(uploader.apiSecret) == "" {
		return CoverUploadResult{}, fmt.Errorf("cloudinary cloud name, api key, and api secret are required")
	}
	if len(asset.Content) == 0 {
		return CoverUploadResult{}, fmt.Errorf("cover upload content is required")
	}

	publicID := publicIDForCover(asset)
	if publicID == "" {
		return CoverUploadResult{}, fmt.Errorf("cover upload public_id is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("folder", uploader.folder); err != nil {
		return CoverUploadResult{}, fmt.Errorf("build cloudinary upload form: %w", err)
	}
	if err := writer.WriteField("public_id", publicID); err != nil {
		return CoverUploadResult{}, fmt.Errorf("build cloudinary upload form: %w", err)
	}
	if err := writer.WriteField("overwrite", "true"); err != nil {
		return CoverUploadResult{}, fmt.Errorf("build cloudinary upload form: %w", err)
	}
	if err := writeCoverFilePart(writer, asset, publicID); err != nil {
		return CoverUploadResult{}, fmt.Errorf("build cloudinary upload form: %w", err)
	}
	if err := writer.Close(); err != nil {
		return CoverUploadResult{}, fmt.Errorf("build cloudinary upload form: %w", err)
	}

	endpoint := strings.TrimRight(uploader.apiBaseURL, "/") + "/v1_1/" + url.PathEscape(uploader.cloudName) + "/image/upload"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return CoverUploadResult{}, fmt.Errorf("build cloudinary upload request: %w", err)
	}
	req.SetBasicAuth(uploader.apiKey, uploader.apiSecret)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := uploader.httpClient.Do(req)
	if err != nil {
		return CoverUploadResult{}, fmt.Errorf("execute cloudinary upload request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return CoverUploadResult{}, fmt.Errorf("cloudinary upload status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var parsed cloudinaryUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return CoverUploadResult{}, fmt.Errorf("decode cloudinary upload response: %w", err)
	}
	parsed.SecureURL = strings.TrimSpace(parsed.SecureURL)
	if parsed.SecureURL == "" {
		return CoverUploadResult{}, fmt.Errorf("cloudinary upload response missing secure_url")
	}
	return CoverUploadResult{
		URL:      parsed.SecureURL,
		PublicID: strings.TrimSpace(parsed.PublicID),
		Format:   strings.TrimSpace(parsed.Format),
		Bytes:    parsed.Bytes,
	}, nil
}

func writeCoverFilePart(writer *multipart.Writer, asset CoverUploadAsset, publicID string) error {
	filename := path.Base(strings.ReplaceAll(strings.TrimSpace(asset.Path), "\\", "/"))
	if filename == "." || filename == "/" || filename == "" {
		filename = publicID
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeMultipartQuote(filename)))
	if contentType := strings.TrimSpace(asset.ContentType); contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = part.Write(asset.Content)
	return err
}

func publicIDForCover(asset CoverUploadAsset) string {
	if slug := slugify(asset.Slug); slug != "" {
		return slug
	}
	assetPath := strings.ReplaceAll(strings.TrimSpace(asset.Path), "\\", "/")
	name := path.Base(assetPath)
	if name == "." || name == "/" {
		return ""
	}
	extension := path.Ext(name)
	return slugify(strings.TrimSuffix(name, extension))
}

func escapeMultipartQuote(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(value)
}

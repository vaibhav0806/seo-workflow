package contentrepo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudinaryCoverUploaderUploadsCoverAsset(t *testing.T) {
	var captured cloudinaryUploadRequest
	var handlerErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, handlerErr = captureCloudinaryUploadRequest(r)
		if handlerErr != nil {
			http.Error(w, handlerErr.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"secure_url":"https://res.cloudinary.com/demo-cloud/image/upload/v1/createos/blog-covers/test-post.png","public_id":"createos/blog-covers/test-post","format":"png","bytes":11}`))
	}))
	defer server.Close()

	uploader := NewCloudinaryCoverUploader("demo-cloud", "api-key", "api-secret", "createos/blog-covers")
	uploader.apiBaseURL = server.URL

	result, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Path:        "covers/test-post.png",
		Content:     []byte("cover-bytes"),
		ContentType: "image/png",
		Slug:        "test-post",
	})

	require.NoError(t, err)
	require.NoError(t, handlerErr)
	require.True(t, captured.Seen)
	require.Equal(t, http.MethodPost, captured.Method)
	require.Equal(t, "/v1_1/demo-cloud/image/upload", captured.Path)
	require.Equal(t, "api-key", captured.BasicAuthUsername)
	require.Equal(t, "api-secret", captured.BasicAuthPassword)
	require.Equal(t, []byte("cover-bytes"), captured.FileBytes)
	require.Equal(t, "image/png", captured.FileContentType)
	require.NotContains(t, captured.Fields, "folder")
	require.Equal(t, "createos/blog-covers", captured.Fields["asset_folder"])
	require.Equal(t, "createos/blog-covers", captured.Fields["public_id_prefix"])
	require.Equal(t, "test-post", captured.Fields["public_id"])
	require.Equal(t, "true", captured.Fields["overwrite"])
	require.NotContains(t, captured.Fields, "type")
	require.Equal(t, CoverUploadResult{
		URL:      "https://res.cloudinary.com/demo-cloud/image/upload/v1/createos/blog-covers/test-post.png",
		PublicID: "createos/blog-covers/test-post",
		Format:   "png",
		Bytes:    11,
	}, result)
}

func TestCloudinaryCoverUploaderRequiresCredentials(t *testing.T) {
	uploader := NewCloudinaryCoverUploader("", "api-key", "api-secret", "createos/blog-covers")

	_, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Path:    "covers/test-post.png",
		Content: []byte("cover-bytes"),
		Slug:    "test-post",
	})

	require.EqualError(t, err, "cloudinary cloud name, api key, and api secret are required")
}

func TestCloudinaryCoverUploaderReturnsStatusAndBodyForFailedUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
	}))
	defer server.Close()

	uploader := NewCloudinaryCoverUploader("demo-cloud", "api-key", "api-secret", "createos/blog-covers")
	uploader.apiBaseURL = server.URL

	_, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Path:    "covers/test-post.png",
		Content: []byte("cover-bytes"),
		Slug:    "test-post",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "cloudinary upload status=401")
	require.Contains(t, err.Error(), "invalid signature")
}

func TestCloudinaryCoverUploaderRequiresSecureURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"public_id":"createos/blog-covers/test-post","format":"png","bytes":11}`))
	}))
	defer server.Close()

	uploader := NewCloudinaryCoverUploader("demo-cloud", "api-key", "api-secret", "createos/blog-covers")
	uploader.apiBaseURL = server.URL

	_, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Path:    "covers/test-post.png",
		Content: []byte("cover-bytes"),
		Slug:    "test-post",
	})

	require.EqualError(t, err, "cloudinary upload response missing secure_url")
}

func TestCloudinaryCoverUploaderDefaultsFolderAndUsesPathSlug(t *testing.T) {
	var captured cloudinaryUploadRequest
	var handlerErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, handlerErr = captureCloudinaryUploadRequest(r)
		if handlerErr != nil {
			http.Error(w, handlerErr.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"secure_url":"https://res.cloudinary.com/demo-cloud/image/upload/v1/createos/blog-covers/my-test-post.png","public_id":"createos/blog-covers/my-test-post","format":"png","bytes":11}`))
	}))
	defer server.Close()

	uploader := NewCloudinaryCoverUploader("demo-cloud", "api-key", "api-secret", "")
	uploader.apiBaseURL = server.URL

	result, err := uploader.UploadCover(context.Background(), CoverUploadAsset{
		Path:    "covers/My Test Post.png",
		Content: []byte("cover-bytes"),
	})

	require.NoError(t, err)
	require.NoError(t, handlerErr)
	require.NotContains(t, captured.Fields, "folder")
	require.Equal(t, defaultCloudinaryUploadFolder, captured.Fields["asset_folder"])
	require.Equal(t, defaultCloudinaryUploadFolder, captured.Fields["public_id_prefix"])
	require.Equal(t, "my-test-post", captured.Fields["public_id"])
	require.Equal(t, "true", captured.Fields["overwrite"])
	require.Equal(t, "https://res.cloudinary.com/demo-cloud/image/upload/v1/createos/blog-covers/my-test-post.png", result.URL)
	require.Equal(t, "createos/blog-covers/my-test-post", result.PublicID)
}

type cloudinaryUploadRequest struct {
	Seen              bool
	Method            string
	Path              string
	BasicAuthUsername string
	BasicAuthPassword string
	Fields            map[string]string
	FileBytes         []byte
	FileContentType   string
}

func captureCloudinaryUploadRequest(r *http.Request) (cloudinaryUploadRequest, error) {
	captured := cloudinaryUploadRequest{
		Seen:   true,
		Method: r.Method,
		Path:   r.URL.Path,
		Fields: map[string]string{},
	}
	username, password, ok := r.BasicAuth()
	if ok {
		captured.BasicAuthUsername = username
		captured.BasicAuthPassword = password
	}
	if err := r.ParseMultipartForm(1024); err != nil {
		return captured, fmt.Errorf("parse multipart form: %w", err)
	}
	for key, values := range r.MultipartForm.Value {
		if len(values) > 0 {
			captured.Fields[key] = values[0]
		}
	}
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		return captured, fmt.Errorf("read file part: %w", err)
	}
	defer file.Close()

	captured.FileContentType = fileHeader.Header.Get("Content-Type")
	captured.FileBytes, err = io.ReadAll(file)
	if err != nil {
		return captured, fmt.Errorf("read file bytes: %w", err)
	}
	return captured, nil
}

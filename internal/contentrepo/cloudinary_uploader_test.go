package contentrepo

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudinaryCoverUploaderUploadsCoverAsset(t *testing.T) {
	var sawUpload bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawUpload = true
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1_1/demo-cloud/image/upload", r.URL.Path)

		username, password, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "api-key", username)
		require.Equal(t, "api-secret", password)

		require.NoError(t, r.ParseMultipartForm(1024))
		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, []byte("cover-bytes"), fileBytes)
		require.Equal(t, "createos/blog-covers", r.MultipartForm.Value["folder"][0])
		require.Equal(t, "test-post", r.MultipartForm.Value["public_id"][0])
		require.Equal(t, "true", r.MultipartForm.Value["overwrite"][0])
		require.Equal(t, "image/png", r.MultipartForm.Value["type"][0])

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
	require.True(t, sawUpload)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1024))
		require.Equal(t, defaultCloudinaryUploadFolder, r.MultipartForm.Value["folder"][0])
		require.Equal(t, "my-test-post", r.MultipartForm.Value["public_id"][0])

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
	require.Equal(t, "https://res.cloudinary.com/demo-cloud/image/upload/v1/createos/blog-covers/my-test-post.png", result.URL)
	require.Equal(t, "createos/blog-covers/my-test-post", result.PublicID)
}

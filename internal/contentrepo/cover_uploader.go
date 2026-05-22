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

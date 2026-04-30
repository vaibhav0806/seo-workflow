package worker

import (
	"context"

	"github.com/nodeops/seo-workflow/internal/classifier"
)

// Inspector fetches normalized inspection signals for a URL under a property.
type Inspector interface {
	InspectURL(ctx context.Context, property string, url string) (classifier.InspectionSignal, error)
}

// PropertyWaiter enforces per-property throttling before inspections.
type PropertyWaiter interface {
	Wait(ctx context.Context, property string) error
}

// Finding is a single inspection classification result.
type Finding struct {
	URL    string
	Bucket classifier.Bucket
}

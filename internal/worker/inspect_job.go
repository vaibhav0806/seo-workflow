package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/nodeops/seo-workflow/internal/classifier"
)

// RunInspectionJob runs deterministic URL inspections for one property.
func RunInspectionJob(
	ctx context.Context,
	inspector Inspector,
	limiter PropertyWaiter,
	property string,
	urls []string,
) ([]Finding, error) {
	if inspector == nil {
		return nil, errors.New("inspection job precondition failed: inspector is nil")
	}

	if limiter == nil {
		return nil, errors.New("inspection job precondition failed: limiter is nil")
	}

	findings := make([]Finding, 0, len(urls))

	for _, url := range urls {
		if err := limiter.Wait(ctx, property); err != nil {
			return nil, fmt.Errorf(
				"inspection job limiter wait failed for property %q url %q: %w",
				property,
				url,
				err,
			)
		}

		signal, err := inspector.InspectURL(ctx, property, url)
		if err != nil {
			return nil, fmt.Errorf(
				"inspection job inspect failed for property %q url %q: %w",
				property,
				url,
				err,
			)
		}

		findings = append(findings, Finding{
			URL:    url,
			Bucket: classifier.Classify(signal),
		})
	}

	return findings, nil
}

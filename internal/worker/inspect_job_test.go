package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/nodeops/seo-workflow/internal/classifier"
	"github.com/stretchr/testify/require"
)

type fakeInspector struct {
	signal    classifier.InspectionSignal
	err       error
	calls     int
	seenProps []string
	seenURLs  []string
}

func (f *fakeInspector) InspectURL(_ context.Context, property string, url string) (classifier.InspectionSignal, error) {
	f.calls++
	f.seenProps = append(f.seenProps, property)
	f.seenURLs = append(f.seenURLs, url)
	if f.err != nil {
		return classifier.InspectionSignal{}, f.err
	}

	return f.signal, nil
}

type fakeLimiter struct {
	err       error
	calls     int
	seenProps []string
}

func (f *fakeLimiter) Wait(_ context.Context, property string) error {
	f.calls++
	f.seenProps = append(f.seenProps, property)
	if f.err != nil {
		return f.err
	}

	return nil
}

func TestRunInspectionJob(t *testing.T) {
	t.Parallel()

	t.Run("returns findings for all urls", func(t *testing.T) {
		t.Parallel()

		inspector := &fakeInspector{
			signal: classifier.InspectionSignal{
				CoverageState: "Not found (404)",
				InSitemap:     true,
			},
		}
		limiter := &fakeLimiter{}

		findings, err := RunInspectionJob(
			context.Background(),
			inspector,
			limiter,
			"sc-domain:example.com",
			[]string{"a", "b"},
		)

		require.NoError(t, err)
		require.Len(t, findings, 2)
		require.Equal(t, classifier.Sitemap404, findings[0].Bucket)
		require.Equal(t, "a", findings[0].URL)
		require.Equal(t, "b", findings[1].URL)
		require.Equal(t, 2, limiter.calls)
		require.Equal(t, 2, inspector.calls)
		require.Equal(t, []string{"sc-domain:example.com", "sc-domain:example.com"}, limiter.seenProps)
		require.Equal(t, []string{"sc-domain:example.com", "sc-domain:example.com"}, inspector.seenProps)
		require.Equal(t, []string{"a", "b"}, inspector.seenURLs)
	})

	t.Run("returns empty findings for empty url list", func(t *testing.T) {
		t.Parallel()

		inspector := &fakeInspector{}
		limiter := &fakeLimiter{}

		findings, err := RunInspectionJob(
			context.Background(),
			inspector,
			limiter,
			"sc-domain:example.com",
			[]string{},
		)

		require.NoError(t, err)
		require.Empty(t, findings)
		require.Equal(t, 0, limiter.calls)
		require.Equal(t, 0, inspector.calls)
		require.Empty(t, limiter.seenProps)
		require.Empty(t, inspector.seenProps)
		require.Empty(t, inspector.seenURLs)
	})

	t.Run("returns error on limiter failure", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("rate limited")
		inspector := &fakeInspector{}
		limiter := &fakeLimiter{err: expectedErr}

		findings, err := RunInspectionJob(
			context.Background(),
			inspector,
			limiter,
			"sc-domain:example.com",
			[]string{"a", "b"},
		)

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "limiter wait failed")
		require.ErrorContains(t, err, "sc-domain:example.com")
		require.ErrorContains(t, err, "a")
		require.Nil(t, findings)
		require.Equal(t, 1, limiter.calls)
		require.Equal(t, 0, inspector.calls)
		require.Equal(t, []string{"sc-domain:example.com"}, limiter.seenProps)
		require.Empty(t, inspector.seenProps)
		require.Empty(t, inspector.seenURLs)
	})

	t.Run("returns error on inspect failure", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("inspect failed")
		inspector := &fakeInspector{err: expectedErr}
		limiter := &fakeLimiter{}

		findings, err := RunInspectionJob(
			context.Background(),
			inspector,
			limiter,
			"sc-domain:example.com",
			[]string{"a", "b"},
		)

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "inspect failed")
		require.ErrorContains(t, err, "sc-domain:example.com")
		require.ErrorContains(t, err, "a")
		require.Nil(t, findings)
		require.Equal(t, 1, limiter.calls)
		require.Equal(t, 1, inspector.calls)
		require.Equal(t, []string{"sc-domain:example.com"}, limiter.seenProps)
		require.Equal(t, []string{"sc-domain:example.com"}, inspector.seenProps)
		require.Equal(t, []string{"a"}, inspector.seenURLs)
	})

	t.Run("returns error for nil inspector", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeLimiter{}

		findings, err := RunInspectionJob(
			context.Background(),
			nil,
			limiter,
			"sc-domain:example.com",
			[]string{"a"},
		)

		require.Error(t, err)
		require.ErrorContains(t, err, "inspector is nil")
		require.Nil(t, findings)
		require.Equal(t, 0, limiter.calls)
		require.Empty(t, limiter.seenProps)
	})

	t.Run("returns error for nil limiter", func(t *testing.T) {
		t.Parallel()

		inspector := &fakeInspector{}

		findings, err := RunInspectionJob(
			context.Background(),
			inspector,
			nil,
			"sc-domain:example.com",
			[]string{"a"},
		)

		require.Error(t, err)
		require.ErrorContains(t, err, "limiter is nil")
		require.Nil(t, findings)
		require.Equal(t, 0, inspector.calls)
		require.Empty(t, inspector.seenProps)
		require.Empty(t, inspector.seenURLs)
	})
}

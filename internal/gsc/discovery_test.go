package gsc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeDiscoveries(t *testing.T) {
	t.Parallel()

	t.Run("merges sitemap and analytics and sorts by impressions", func(t *testing.T) {
		t.Parallel()

		sitemap := []string{"a", "b"}
		analytics := []URLMetric{
			{URL: "b", Impressions: 50},
			{URL: "c", Impressions: 100},
		}

		got := MergeDiscoveries(sitemap, analytics)

		require.Equal(t, []string{"c", "b", "a"}, got)
	})

	t.Run("dedupes urls ignores empties uses max impressions and tie-breaks lexicographically", func(t *testing.T) {
		t.Parallel()

		sitemap := []string{"", "b", "a", "a"}
		analytics := []URLMetric{
			{URL: "", Impressions: 100},
			{URL: "a", Impressions: 10},
			{URL: "a", Impressions: 50},
			{URL: "c", Impressions: 50},
			{URL: "b", Impressions: 50},
		}

		got := MergeDiscoveries(sitemap, analytics)

		require.Equal(t, []string{"a", "b", "c"}, got)
	})
}

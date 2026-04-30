package oneshot

import (
	"context"
	"testing"

	"github.com/nodeops/seo-workflow/internal/pr"
	"github.com/stretchr/testify/require"
)

func TestSplitRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantOrg  string
		wantRepo string
		wantErr  bool
	}{
		{name: "valid", input: "nodeops/seo-workflow", wantOrg: "nodeops", wantRepo: "seo-workflow"},
		{name: "missing slash", input: "nodeops", wantErr: true},
		{name: "empty repo", input: "nodeops/", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			org, repo, err := splitRepo(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantOrg, org)
			require.Equal(t, tc.wantRepo, repo)
		})
	}
}

func TestDryRunOpen(t *testing.T) {
	t.Parallel()

	opener := NewGitHubOpener("", "nodeops/seo-workflow", "", "public/sitemap.xml", true)
	plan := pr.PullRequestPlan{Branch: "seo/fix-sitemap-404", Files: []pr.FileEdit{{Path: "public/sitemap.xml", Content: "x"}}}

	prURL, err := opener.Open(context.Background(), "sc-domain:example.com", plan)
	require.NoError(t, err)
	require.Contains(t, prURL, "dry-run://")
	require.Contains(t, prURL, "nodeops/seo-workflow")
}

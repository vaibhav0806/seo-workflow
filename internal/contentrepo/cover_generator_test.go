package contentrepo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoverPromptUsesCreateOSPastelNatureTechStyle(t *testing.T) {
	post := BlogPost{
		Title:        "Enterprise Security Governance",
		Description:  "How teams govern AI app development without fragmenting execution.",
		Slug:         "enterprise-security-governance",
		Tags:         []string{"security", "enterprise"},
		Author:       "CreateOS",
		Cover:        "https://example.com/cover.png",
		PublishedAt:  time.Date(2026, 5, 19, 8, 30, 0, 0, time.UTC),
		Destination:  DestinationCreateOS,
		BodyMarkdown: "# Enterprise Security Governance\n\nBody.",
	}

	prompt := coverPrompt(post)

	require.Contains(t, prompt, "bright surreal 3D landscape")
	require.Contains(t, prompt, "soft pastel terrain")
	require.Contains(t, prompt, "moss, vines, flowers")
	require.Contains(t, prompt, "glassy futuristic technology objects")
	require.Contains(t, prompt, "policy gates")
	require.Contains(t, prompt, "protected workflow layers")
	require.Contains(t, prompt, "no readable text")
	require.NotContains(t, prompt, "dark graphite")
}

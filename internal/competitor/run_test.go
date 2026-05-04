package competitor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadCreateOSContextReturnsTrimmedContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "createos-context.md")
	require.NoError(t, os.WriteFile(path, []byte("\nCreateOS is the workspace where ideas become applications.\n"), 0o644))

	content, err := readCreateOSContext(path)

	require.NoError(t, err)
	require.Equal(t, "CreateOS is the workspace where ideas become applications.", content)
}

func TestReadCreateOSContextAllowsMissingFile(t *testing.T) {
	content, err := readCreateOSContext(filepath.Join(t.TempDir(), "missing.md"))

	require.NoError(t, err)
	require.Empty(t, content)
}

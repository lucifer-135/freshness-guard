package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteResultNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

	first, err := writeResult(dir, "curated", day, map[string]int{"a": 1}, "# one\n")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "curated-2026-11-08.json"), first.JSON)
	assert.Equal(t, filepath.Join(dir, "curated-2026-11-08.md"), first.Markdown)

	second, err := writeResult(dir, "curated", day, map[string]int{"a": 2}, "# two\n")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "curated-2026-11-08-2.json"), second.JSON)

	third, err := writeResult(dir, "curated", day, map[string]int{"a": 3}, "# three\n")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "curated-2026-11-08-3.md"), third.Markdown)

	kept, err := os.ReadFile(first.Markdown)
	require.NoError(t, err)
	assert.Equal(t, "# one\n", string(kept), "the first result is untouched")
}

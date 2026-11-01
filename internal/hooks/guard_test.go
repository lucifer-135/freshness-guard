package hooks_test

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/hooks"
)

func TestGuardReturnsFalseOnPanicAndLogsStack(t *testing.T) {
	var buf bytes.Buffer
	ok := hooks.Guard(slog.New(slog.NewTextHandler(&buf, nil)), "TestHook", func() error { panic("boom") })
	assert.False(t, ok)
	assert.Contains(t, buf.String(), "TestHook")
	assert.Contains(t, buf.String(), "boom")
	assert.Contains(t, buf.String(), "goroutine")
}

func TestGuardReturnsFalseOnError(t *testing.T) {
	var buf bytes.Buffer
	ok := hooks.Guard(slog.New(slog.NewTextHandler(&buf, nil)), "TestHook", func() error { return errors.New("failed") })
	assert.False(t, ok)
	assert.Contains(t, buf.String(), "failed")
}

func TestGuardReturnsTrueOnSuccess(t *testing.T) {
	called := false
	ok := hooks.Guard(slog.Default(), "TestHook", func() error { called = true; return nil })
	assert.True(t, ok)
	assert.True(t, called)
}

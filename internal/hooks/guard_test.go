package hooks_test

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/hooks"
)

func TestGuardRecoversPanic(t *testing.T) {
	var buf bytes.Buffer
	err := hooks.Guard(slog.New(slog.NewTextHandler(&buf, nil)), "TestHook", func() error { panic("boom") })
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "TestHook")
	assert.Contains(t, buf.String(), "boom")
}

func TestGuardSwallowsError(t *testing.T) {
	var buf bytes.Buffer
	err := hooks.Guard(slog.New(slog.NewTextHandler(&buf, nil)), "TestHook", func() error { return errors.New("failed") })
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "failed")
}

func TestGuardRunsFunction(t *testing.T) {
	called := false
	_ = hooks.Guard(slog.Default(), "TestHook", func() error { called = true; return nil })
	assert.True(t, called)
}

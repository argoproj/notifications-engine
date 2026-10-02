package http

import (
	"io"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryableHTTPLogger_HonoursLogLevel(t *testing.T) {
	// exercises the constructor, so this covers the global logger the webhook service gets
	hook := logtest.NewGlobal()
	level := log.GetLevel()
	log.SetOutput(io.Discard)
	log.SetLevel(log.InfoLevel)
	t.Cleanup(func() {
		log.SetLevel(level)
		log.SetOutput(os.Stderr)
	})

	l := NewRetryableHTTPLogger("webhook")

	// this is the call retryablehttp makes for every request
	l.Debug("performing request", "method", "POST", "url", "https://example.com/invoke?sig=secret")
	assert.Empty(t, hook.AllEntries(), "debug output must not be emitted at info level")

	log.SetLevel(log.DebugLevel)
	l.Debug("performing request", "method", "POST")
	require.Len(t, hook.AllEntries(), 1)
	assert.Equal(t, "webhook", hook.LastEntry().Data["service"])
	assert.Equal(t, "POST", hook.LastEntry().Data["method"])
}

func TestRetryableHTTPLogger_MapsLevelsAndFields(t *testing.T) {
	logger, hook := logtest.NewNullLogger()
	logger.SetLevel(log.DebugLevel)
	l := &retryableHTTPLogger{entry: logger.WithField("service", "webhook")}

	l.Debug("performing request", "method", "POST")
	l.Info("retrying request", "remaining", 2)
	l.Warn("unexpected HTTP status", "status", "502")
	l.Error("request failed", "error", "connection refused")

	entries := hook.AllEntries()
	require.Len(t, entries, 4)

	assert.Equal(t, log.DebugLevel, entries[0].Level)
	assert.Equal(t, "performing request", entries[0].Message)
	assert.Equal(t, "webhook", entries[0].Data["service"])
	assert.Equal(t, "POST", entries[0].Data["method"])

	assert.Equal(t, log.InfoLevel, entries[1].Level)
	assert.Equal(t, 2, entries[1].Data["remaining"])

	assert.Equal(t, log.WarnLevel, entries[2].Level)
	assert.Equal(t, "502", entries[2].Data["status"])

	assert.Equal(t, log.ErrorLevel, entries[3].Level)
	assert.Equal(t, "connection refused", entries[3].Data["error"])
}

func TestRetryableHTTPLogger_ToleratesOddVarargs(t *testing.T) {
	logger, hook := logtest.NewNullLogger()
	logger.SetLevel(log.DebugLevel)
	l := &retryableHTTPLogger{entry: logger.WithField("service", "webhook")}

	l.Debug("no pairs at all")
	l.Debug("dangling key", "method")
	l.Debug("pair plus dangling key", "method", "POST", "url")

	entries := hook.AllEntries()
	require.Len(t, entries, 3)
	assert.Equal(t, log.Fields{"service": "webhook"}, entries[0].Data)
	assert.Equal(t, log.Fields{"service": "webhook"}, entries[1].Data)
	assert.Equal(t, log.Fields{"service": "webhook", "method": "POST"}, entries[2].Data)
}

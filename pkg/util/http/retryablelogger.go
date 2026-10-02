package http

import (
	"fmt"

	"github.com/hashicorp/go-retryablehttp"
	log "github.com/sirupsen/logrus"
)

// NewRetryableHTTPLogger adapts logrus to retryablehttp.LeveledLogger.
//
// retryablehttp.NewClient defaults to its own stdlib logger, which prints
// "[DEBUG] <method> <url>" to stderr for every request. That bypasses the configured logrus
// level entirely, so the line cannot be turned off, and it puts the full request URL - including
// any credential carried in a query parameter - into the controller log. Requests are already
// logged at debug level by NewLoggingRoundTripper, so this keeps retryablehttp's own output on
// the same logger and under the same level.
func NewRetryableHTTPLogger(serviceName string) retryablehttp.LeveledLogger {
	return &retryableHTTPLogger{entry: log.WithField("service", serviceName)}
}

type retryableHTTPLogger struct {
	entry *log.Entry
}

func (l *retryableHTTPLogger) Error(msg string, keysAndValues ...any) {
	l.withFields(keysAndValues).Error(msg)
}

func (l *retryableHTTPLogger) Warn(msg string, keysAndValues ...any) {
	l.withFields(keysAndValues).Warn(msg)
}

func (l *retryableHTTPLogger) Info(msg string, keysAndValues ...any) {
	l.withFields(keysAndValues).Info(msg)
}

func (l *retryableHTTPLogger) Debug(msg string, keysAndValues ...any) {
	l.withFields(keysAndValues).Debug(msg)
}

// withFields turns retryablehttp's alternating key/value varargs into logrus fields. A trailing
// key without a value is dropped.
func (l *retryableHTTPLogger) withFields(keysAndValues []any) *log.Entry {
	if len(keysAndValues) < 2 {
		return l.entry
	}
	fields := make(log.Fields, len(keysAndValues)/2)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		fields[fmt.Sprint(keysAndValues[i])] = keysAndValues[i+1]
	}
	return l.entry.WithFields(fields)
}

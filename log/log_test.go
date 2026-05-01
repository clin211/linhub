package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureLogger 创建一个把日志写入 buffer 的 logger，用于断言输出。
func captureLogger(t *testing.T, format string, level string) (*slogLogger, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}

	opts := &slog.HandlerOptions{
		AddSource: false,
		Level:     mustLevel(level),
	}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(buf, opts)
	default:
		handler = slog.NewTextHandler(buf, opts)
	}

	o := NewOptions()
	o.Format = format
	o.Level = level

	logger := &slogLogger{
		logger:            slog.New(handler),
		opts:              o,
		contextExtractors: map[string]func(context.Context) string{},
	}
	return logger, buf
}

func mustLevel(s string) slog.Level {
	l, _ := parseLevel(s)
	return l
}

func TestSugarFormatStyle(t *testing.T) {
	logger, buf := captureLogger(t, "text", "debug")

	logger.Infof("hello %s, age=%d", "alice", 30)
	out := buf.String()
	assert.Contains(t, out, "hello alice, age=30")
}

func TestSugarKeyvalsStyle(t *testing.T) {
	logger, buf := captureLogger(t, "json", "debug")

	logger.Infow("user_login", "user_id", "u-001", "ip", "1.2.3.4")

	var record map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &record); err != nil {
		t.Fatalf("expected json output: %v\nraw=%s", err, buf.String())
	}
	assert.Equal(t, "user_login", record["msg"])
	assert.Equal(t, "u-001", record["user_id"])
	assert.Equal(t, "1.2.3.4", record["ip"])
}

func TestErrorwAddsErrField(t *testing.T) {
	logger, buf := captureLogger(t, "json", "debug")

	logger.Errorw(errors.New("boom"), "operation_failed", "op", "delete")

	var record map[string]any
	_ = json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &record)
	assert.Equal(t, "operation_failed", record["msg"])
	assert.Equal(t, "delete", record["op"])
	assert.Equal(t, "boom", record["err"])
	assert.Equal(t, "ERROR", record["level"])
}

func TestLevelFiltering(t *testing.T) {
	logger, buf := captureLogger(t, "text", "warn")

	logger.Debugw("debug message", "key", "v")
	logger.Infow("info message", "key", "v")
	logger.Warnw("warn message", "key", "v")
	logger.Errorw(nil, "error message", "key", "v")

	out := buf.String()
	assert.NotContains(t, out, "debug message")
	assert.NotContains(t, out, "info message")
	assert.Contains(t, out, "warn message")
	assert.Contains(t, out, "error message")
}

func TestWithContextExtractors(t *testing.T) {
	logger, buf := captureLogger(t, "json", "debug")
	logger.contextExtractors = map[string]func(context.Context) string{
		"trace_id": func(ctx context.Context) string {
			if v, ok := ctx.Value("trace").(string); ok {
				return v
			}
			return ""
		},
	}

	ctx := context.WithValue(context.Background(), "trace", "tr-abc")
	logger.W(ctx).Infow("with-context", "k", "v")

	var record map[string]any
	_ = json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &record)
	assert.Equal(t, "tr-abc", record["trace_id"])
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"":        slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"panic":   slog.LevelError,
		"fatal":   slog.LevelError,
	}
	for input, want := range tests {
		got, err := parseLevel(input)
		assert.NoError(t, err, "input=%s", input)
		assert.Equal(t, want, got, "input=%s", input)
	}

	_, err := parseLevel("not-a-level")
	assert.Error(t, err)
}

func TestNewOptionsDefaults(t *testing.T) {
	o := NewOptions()
	assert.Equal(t, "INFO", o.Level)
	assert.Equal(t, "text", o.Format)
	assert.False(t, o.EnableOTel)
	assert.False(t, o.AddSource)
}

func TestInitDoesNotCrashOnInvalidPath(t *testing.T) {
	// 即便 outputPaths 中指定了不可写文件，Init 也不应 panic
	opts := NewOptions()
	opts.OutputPaths = []string{"/path/that/should/not/exist/log.txt"}
	assert.NotPanics(t, func() {
		Init(opts)
	})
}

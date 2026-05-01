package otelslog_test

import (
	"go.opentelemetry.io/otel/log/noop"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

func Example() {
	// 实际使用时请改用一个可用的 LoggerProvider 实现，例如使用 go.opentelemetry.io/otel/sdk/log.
	provider := noop.NewLoggerProvider()

	// 创建一个 *slog.Logger 并在你的应用中使用它.
	otelslog.NewLogger("my/pkg/name", otelslog.WithLoggerProvider(provider))
}

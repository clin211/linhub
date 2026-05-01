package log

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// 慢 SQL 阈值。超过此阈值的 SQL 会以 warn 级别输出。
const slowSQLThreshold = 200 * time.Millisecond

// gormLevelMap 把 gorm 日志级别映射为 slog 级别。
var gormLevelMap = map[string]gormlogger.LogLevel{
	"silent":  gormlogger.Silent,
	"panic":   gormlogger.Silent,
	"error":   gormlogger.Error,
	"warn":    gormlogger.Warn,
	"warning": gormlogger.Warn,
	"info":    gormlogger.Info,
	"debug":   gormlogger.Info,
}

// gormLevel 返回当前 logger 配置对应的 gorm 日志级别。
func (l *slogLogger) gormLevel() gormlogger.LogLevel {
	if l.opts == nil {
		return gormlogger.Warn
	}
	if lvl, ok := gormLevelMap[l.opts.Level]; ok {
		return lvl
	}
	return gormlogger.Warn
}

// LogMode 实现 gormlogger.Interface。
// 这里返回新的 logger 实例（仅修改副本中的级别）。
func (l *slogLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	cp := *l
	optsCopy := *l.opts
	switch {
	case level <= gormlogger.Silent:
		optsCopy.Level = "error"
	case level == gormlogger.Error:
		optsCopy.Level = "error"
	case level == gormlogger.Warn:
		optsCopy.Level = "warn"
	default:
		optsCopy.Level = "info"
	}
	cp.opts = &optsCopy
	return &cp
}

// Info 实现 gormlogger.Interface。
func (l *slogLogger) Info(ctx context.Context, msg string, keyvals ...any) {
	if l.gormLevel() < gormlogger.Info {
		return
	}
	l.logger.Log(ctx, slog.LevelInfo, msg, keyvals...)
}

// Warn 实现 gormlogger.Interface。
func (l *slogLogger) Warn(ctx context.Context, msg string, keyvals ...any) {
	if l.gormLevel() < gormlogger.Warn {
		return
	}
	l.logger.Log(ctx, slog.LevelWarn, msg, keyvals...)
}

// Error 实现 gormlogger.Interface。
func (l *slogLogger) Error(ctx context.Context, msg string, keyvals ...any) {
	if l.gormLevel() < gormlogger.Error {
		return
	}
	l.logger.Log(ctx, slog.LevelError, msg, keyvals...)
}

// Trace 实现 gormlogger.Interface：记录每条 SQL 的执行情况。
//
// - 出错时（且非 RecordNotFound）以 error 级别记录
// - 超过 slowSQLThreshold 时以 warn 级别记录
// - 否则以 debug 级别记录（避免在生产环境产生过多日志）
func (l *slogLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	level := l.gormLevel()
	if level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && level >= gormlogger.Error && !errors.Is(err, gorm.ErrRecordNotFound):
		sql, rows := fc()
		l.logger.Log(ctx, slog.LevelError, "gorm: query failed",
			"err", err,
			"elapsed_ms", float64(elapsed.Microseconds())/1000,
			"rows", rows,
			"sql", sql,
		)
	case elapsed > slowSQLThreshold && level >= gormlogger.Warn:
		sql, rows := fc()
		l.logger.Log(ctx, slog.LevelWarn, "gorm: slow sql",
			"threshold_ms", slowSQLThreshold.Milliseconds(),
			"elapsed_ms", float64(elapsed.Microseconds())/1000,
			"rows", rows,
			"sql", sql,
		)
	case level >= gormlogger.Info:
		sql, rows := fc()
		l.logger.Log(ctx, slog.LevelDebug, "gorm: query",
			"elapsed_ms", float64(elapsed.Microseconds())/1000,
			"rows", rows,
			"sql", sql,
		)
	}
}

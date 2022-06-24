package pg

import (
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/log/zerologadapter"
	"github.com/rs/zerolog"
)

var _ pgx.Logger = (*zerologAdapter)(nil)

// zerologAdapter is zerolog adapter for pgx.Logger with custom log level option
type zerologAdapter struct {
	externalAdapter *zerologadapter.Logger
	level           pgx.LogLevel
}

func newZerologAdapter(log zerolog.Logger) *zerologAdapter {
	var lvl pgx.LogLevel
	switch log.GetLevel() {
	case zerolog.NoLevel, zerolog.Disabled:
		lvl = pgx.LogLevelNone
	case zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel:
		lvl = pgx.LogLevelError
	case zerolog.WarnLevel:
		lvl = pgx.LogLevelWarn
	case zerolog.InfoLevel:
		lvl = pgx.LogLevelInfo
	case zerolog.DebugLevel:
		lvl = pgx.LogLevelDebug
	case zerolog.TraceLevel:
		lvl = pgx.LogLevelTrace
	default:
		lvl = pgx.LogLevelDebug
	}
	return &zerologAdapter{level: lvl, externalAdapter: zerologadapter.NewLogger(log)}
}

func (z *zerologAdapter) Log(ctx context.Context, _ pgx.LogLevel, msg string, data map[string]interface{}) {
	z.externalAdapter.Log(ctx, z.level, msg, data)
}

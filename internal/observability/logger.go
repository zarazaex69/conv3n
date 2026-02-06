package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

type Logger struct {
	*slog.Logger
}

func NewLogger(level slog.Level, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format(time.RFC3339Nano))
			}
			return a
		},
	})

	return &Logger{
		Logger: slog.New(handler),
	}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	span := SpanFromContext(ctx)
	if span == nil {
		return l
	}

	return &Logger{
		Logger: l.With(
			slog.String("trace_id", string(span.ctx.TraceID)),
			slog.String("span_id", string(span.ctx.SpanID)),
		),
	}
}

func (l *Logger) WithWorkflow(workflowID, executionID string) *Logger {
	return &Logger{
		Logger: l.With(
			slog.String("workflow_id", workflowID),
			slog.String("execution_id", executionID),
		),
	}
}

func (l *Logger) WithNode(nodeID string) *Logger {
	return &Logger{
		Logger: l.With(slog.String("node_id", nodeID)),
	}
}

var globalLogger = NewLogger(slog.LevelInfo, nil)

func GetLogger() *Logger {
	return globalLogger
}

func SetLogger(logger *Logger) {
	globalLogger = logger
}

package logging

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/kanengo/akasar/runtime/protos"
)

type ignoreWriter struct {
}

func (i ignoreWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func TestSLogger(t *testing.T) {
	pp := &JsonPrinter{}
	logger := slog.New(&LogHandler{
		opts: Options{},
		Write: func(entry *protos.LogEntry) {
			pp.Format(entry)
			_, _ = fmt.Fprintln(os.Stdout, pp.Format(entry))
		},
	})

	logger.Info("hello", "name", "leeka")
	//logger.WithGroup("test").Info("group")
	logger.With(slog.Int64("id", 15521170853)).Info("with id")
}

func loggerBenchmark(b *testing.B, logger *slog.Logger) {
	logger = logger.With("id", 1234567)
	logger = logger.With("name", "leeka")
	logger = logger.With("phone", "15521170853")

	logger.Info("hello logger", slog.String("age", "30"))

}

func BenchmarkCompare(b *testing.B) {
	b.Run("custom", BenchmarkLogHandler)

	b.Run("json", BenchmarkJsonLogHandler)

	b.Run("json-2", BenchmarkJsonLogHandler2)
}

func BenchmarkLogHandler(b *testing.B) {
	pp := &JsonPrinter{}
	logger := slog.New(&LogHandler{
		opts: Options{},
		Write: func(entry *protos.LogEntry) {
			pp.Format(entry)
			//_, _ = os.Stdout.Write([]byte(entry.String()))
		},
	})

	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loggerBenchmark(b, logger)
	}

	b.StopTimer()

}

func BenchmarkJsonLogHandler(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(&ignoreWriter{}, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 {
				if a.Key == slog.TimeKey {
					a.Value = slog.StringValue(a.Value.Time().Format(time.DateTime))
					return a
				}
			}

			return a
		},
	}))

	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loggerBenchmark(b, logger)
	}

	b.StopTimer()

}

func BenchmarkJsonLogHandler2(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(&ignoreWriter{}, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 {
				if a.Key == slog.TimeKey {
					a.Value = slog.StringValue(a.Value.Time().Format(time.DateTime))
					return a
				} else if a.Key == slog.SourceKey {
					s, ok := a.Value.Any().(*slog.Source)
					if ok {
						a.Value = slog.StringValue(fmt.Sprintf("%s:%d", s.File, s.Line))
					}
				}
			}

			return a
		},
	}))

	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loggerBenchmark(b, logger)
	}

	b.StopTimer()

}

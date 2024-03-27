package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"time"
)

type Options struct {
	App        string
	Deployment string
	Component  string

	//Attrs []string
	Attrs []slog.Attr
}

type LogHandler struct {
	opts Options
	*slog.JSONHandler
}

// NewLogHandler
// w after w.Write(b), b will be put back to a sync.Pool, so do not continue hold a reference to b.
func NewLogHandler(w io.Writer, opts Options, level slog.Leveler) *LogHandler {
	h := &LogHandler{
		opts: opts,
		JSONHandler: slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if len(groups) == 0 {
					if a.Key == slog.TimeKey {
						a.Value = slog.StringValue(a.Value.Time().Format(time.DateTime))
						return a
					}
				}

				return a
			},
		}),
	}

	if opts.App != "" {
		h.opts.Attrs = append(h.opts.Attrs, slog.String("app", opts.App))
	}

	if opts.Component != "" {
		h.opts.Attrs = append(h.opts.Attrs, slog.String("component", opts.Component))
	}

	if opts.Deployment != "" {
		h.opts.Attrs = append(h.opts.Attrs, slog.String("deployment", opts.Deployment))
	}

	return h
}

var _ slog.Handler = (*LogHandler)(nil)

func (h *LogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.JSONHandler.Enabled(ctx, level)
}

func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.opts.Attrs = append(c.opts.Attrs, attrs...)
	return &c
}

func (h *LogHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *LogHandler) Handle(ctx context.Context, r slog.Record) error {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	if fs != nil {
		f, _ := fs.Next()
		h.opts.Attrs = append(h.opts.Attrs, slog.String(slog.SourceKey, fmt.Sprintf("%s:%d", f.File, f.Line)))
	}
	if len(h.opts.Attrs) > 0 {
		r.AddAttrs(h.opts.Attrs...)
	}
	return h.JSONHandler.Handle(ctx, r)
}

type LogHandler2 struct {
	opts Options
	*slog.JSONHandler
}

var _ slog.Handler = (*LogHandler2)(nil)

func (h *LogHandler2) Enabled(ctx context.Context, level slog.Level) bool {
	return h.JSONHandler.Enabled(ctx, level)
}

func (h *LogHandler2) WithGroup(name string) slog.Handler {
	return h
}

func (h *LogHandler2) Handle(ctx context.Context, r slog.Record) error {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	if fs != nil {
		f, _ := fs.Next()
		h.opts.Attrs = append(h.opts.Attrs, slog.String(slog.SourceKey, fmt.Sprintf("%s:%d", f.File, f.Line)))
	}
	if len(h.opts.Attrs) > 0 {
		r.AddAttrs(h.opts.Attrs...)
	}
	return h.JSONHandler.Handle(ctx, r)
}

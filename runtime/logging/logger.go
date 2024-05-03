package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"slices"

	"github.com/kanengo/akasar/runtime/protos"
)

const SystemAttribute = "serviceakasar/system"

type Options struct {
	App        string
	Deployment string
	Component  string
	Akasalet   string

	//Attrs []string
	Attrs []slog.Attr

	Level int32
}

type LogHandler struct {
	opts   Options
	Write  func(entry *protos.LogEntry)
	level  slog.Leveler
	groups []string
	attrs  []string
}

// NewLogHandler
// w after w.Write(b), b will be put back to a sync.Pool, so do not continue hold a reference to b.
func NewLogHandler(w func(entry *protos.LogEntry), opts Options) *LogHandler {
	h := &LogHandler{
		opts:  opts,
		Write: w,
	}

	if opts.Level != 0 {
		h.level = slog.Level(opts.Level)
	}

	return h
}

func appendAttrs(prefix []string, attrs ...slog.Attr) []string {
	if len(attrs) == 0 {
		return prefix
	}

	// 复制prefix 防止多个goroutine并发操作问题
	dst := make([]string, 0, len(prefix)+len(attrs)*2)
	dst = append(dst, prefix...)

	for _, attr := range attrs {
		dst = append(dst, attr.Key, attr.Value.String())
	}

	return dst
}

var _ slog.Handler = (*LogHandler)(nil)

func (h *LogHandler) Enabled(_ context.Context, l slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.level != nil {
		minLevel = h.level.Level()
	}

	return l >= minLevel
}

func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.attrs = appendAttrs(c.attrs, attrs...)
	return &c
}

func (h *LogHandler) WithGroup(name string) slog.Handler {
	h2 := &LogHandler{
		opts:   h.opts,
		Write:  h.Write,
		groups: slices.Clip(h.groups),
	}
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *LogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.Write(h.makeEntry(r))
	return nil
}

func (h *LogHandler) makeEntry(record slog.Record) *protos.LogEntry {
	attrsCount := 0
	record.Attrs(func(attr slog.Attr) bool {
		attrsCount += 1
		return true
	})
	attrs := make([]slog.Attr, 0, attrsCount)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	entry := protos.LogEntry{
		App:        h.opts.App,
		Version:    h.opts.Deployment,
		Component:  h.opts.Component,
		Node:       h.opts.Akasalet,
		TimeMicros: record.Time.UnixMicro(),
		Level:      record.Level.String(),
		Msg:        record.Message,
		Attrs:      appendAttrs(h.attrs, attrs...),
		File:       "",
		Line:       -1,
	}

	fs := runtime.CallersFrames([]uintptr{record.PC})
	if fs != nil {
		f, _ := fs.Next()
		entry.File = f.File
		entry.Line = int32(f.Line)
	}

	return &entry
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

func StderrLogger(opts Options) *slog.Logger {
	pp := NewPrettyPrinter()
	writer := func(entry *protos.LogEntry) {
		_, _ = fmt.Fprintln(os.Stderr, pp.Format(entry))
	}

	return slog.New(&LogHandler{
		opts:  opts,
		Write: writer,
	})
}

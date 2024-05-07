package akasar

import (
	"context"
	"fmt"
	"io"

	"github.com/kanengo/akasar/runtime/protos"
)

type remoteLogger struct {
	c        chan *protos.LogEntry
	fallback io.Writer
}

const logBufferCount = 1000

func newRemoteLogger(fallback io.Writer) *remoteLogger {
	rl := &remoteLogger{
		c:        make(chan *protos.LogEntry, logBufferCount),
		fallback: fallback,
	}

	return rl
}

func (rl *remoteLogger) log(entry *protos.LogEntry) {
	rl.c <- entry
}

func (rl *remoteLogger) run(ctx context.Context, logBatch func(context.Context, *protos.LogEntryBatch) error) {
	batch := &protos.LogEntryBatch{
		Entries: make([]*protos.LogEntry, 0, logBufferCount),
	}
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-rl.c:
			batch.Entries = append(batch.Entries, entry)
		readLoop:
			//若接收到新日志，则继续尝试接收，直到没有新日志再执行批量日志操作
			for {
				select {
				case <-ctx.Done():
					return
				case entry := <-rl.c:
					batch.Entries = append(batch.Entries, entry)
				default:
					break readLoop
				}
			}
			if err := logBatch(ctx, batch); err != nil {
				attr := err.Error()
				for _, e := range batch.Entries {
					e.Attrs = append(e.Attrs, "serviceakasar/logerror", attr)
					_, _ = fmt.Fprintln(rl.fallback)
				}
			}
			batch.Entries = batch.Entries[:0]
		}
	}
}

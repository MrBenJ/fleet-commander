package hangar

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

type channelLogHandler struct {
	ch    chan string
	mu    sync.Mutex
	attrs []slog.Attr
}

func newChannelLogger(ch chan string) *slog.Logger {
	return slog.New(&channelLogHandler{ch: ch})
}

func (h *channelLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *channelLogHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	attrs := append([]slog.Attr(nil), h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	h.mu.Unlock()

	var b strings.Builder
	b.WriteString(record.Level.String())
	b.WriteString(" ")
	b.WriteString(record.Message)
	for _, attr := range attrs {
		b.WriteString(" ")
		b.WriteString(attr.Key)
		b.WriteString("=")
		b.WriteString(attr.Value.String())
	}

	select {
	case h.ch <- b.String():
	default:
	}
	return nil
}

func (h *channelLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	next := &channelLogHandler{ch: h.ch}
	next.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return next
}

func (h *channelLogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.WithAttrs([]slog.Attr{slog.String("group", name)})
}

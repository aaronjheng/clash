package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/square/exit"
	"go.uber.org/multierr"

	"github.com/clash-dev/clash/internal/common/observable"
)

var level = &slog.LevelVar{}

func Infoln(format string, v ...any) {
	slog.Info(fmt.Sprintf(format, v...))
}

func Warnln(format string, v ...any) {
	slog.Warn(fmt.Sprintf(format, v...))
}

func Errorln(format string, v ...any) {
	slog.Error(fmt.Sprintf(format, v...))
}

func Debugln(format string, v ...any) {
	slog.Debug(fmt.Sprintf(format, v...))
}

func Fatalln(format string, v ...any) {
	slog.Error(fmt.Sprintf(format, v...))
	os.Exit(exit.NotOK)
}

type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{
		handlers: handlers,
	}
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var err error
	for _, handler := range h.handlers {
		multierr.AppendInto(&err, handler.Handle(ctx, r.Clone()))
	}

	return err
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}

	return false
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}

	return newMultiHandler(handlers...)
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}

	return newMultiHandler(handlers...)
}

type chanWriter struct {
	ch chan any
}

func newChanWriter(ch chan any) *chanWriter {
	return &chanWriter{ch: ch}
}

func (c *chanWriter) Write(p []byte) (n int, err error) {
	c.ch <- p

	return len(p), nil
}

type LoggerProvider struct {
	levelVar   *slog.LevelVar
	logger     *slog.Logger
	observable *observable.Observable
}

func (l *LoggerProvider) Observable() *observable.Observable {
	return l.observable
}

func (l *LoggerProvider) Logger() *slog.Logger {
	return l.logger
}

func (l *LoggerProvider) LevelVar() *slog.LevelVar {
	return l.levelVar
}

func Install() *LoggerProvider {
	opts := &slog.HandlerOptions{Level: level}

	logCh := make(chan any)
	observable := observable.NewObservable(logCh)

	consoleHandler := slog.NewTextHandler(os.Stderr, opts)
	channelHandler := slog.NewJSONHandler(newChanWriter(logCh), opts)

	logger := slog.New(newMultiHandler(consoleHandler, channelHandler))

	slog.SetDefault(logger)

	return &LoggerProvider{
		logger:     logger,
		levelVar:   level,
		observable: observable,
	}
}

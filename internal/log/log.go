package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/square/exit"

	"github.com/clash-dev/clash/internal/common/observable"
)

var (
	logCh  = make(chan any)
	source = observable.NewObservable(logCh)
	level  = &slog.LevelVar{}
)

type Event struct {
	LogLevel slog.Level
	Payload  string
}

func (e *Event) Type() string {
	return e.LogLevel.String()
}

func Infoln(format string, v ...any) {
	event := newLog(slog.LevelInfo, format, v...)
	logCh <- event
	print(event)
}

func Warnln(format string, v ...any) {
	event := newLog(slog.LevelWarn, format, v...)
	logCh <- event
	print(event)
}

func Errorln(format string, v ...any) {
	event := newLog(slog.LevelError, format, v...)
	logCh <- event
	print(event)
}

func Debugln(format string, v ...any) {
	event := newLog(slog.LevelDebug, format, v...)
	logCh <- event
	print(event)
}

func Fatalln(format string, v ...any) {
	slog.Error(fmt.Sprintf(format, v...))
	os.Exit(exit.NotOK)
}

func Subscribe() observable.Subscription {
	sub, _ := source.Subscribe()
	return sub
}

func UnSubscribe(sub observable.Subscription) {
	source.UnSubscribe(sub)
}

func Level() slog.Level {
	return level.Level()
}

func SetLevel(lvl slog.Level) {
	level.Set(lvl)
}

func print(data Event) {
	slog.Log(context.Background(), data.LogLevel, data.Payload)
}

func newLog(logLevel slog.Level, format string, v ...any) Event {
	return Event{
		LogLevel: logLevel,
		Payload:  fmt.Sprintf(format, v...),
	}
}

func Setup() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))

	slog.SetDefault(logger)
}

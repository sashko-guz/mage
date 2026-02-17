package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync/atomic"
)

type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var currentLevel atomic.Int32

func init() {
	currentLevel.Store(int32(LevelInfo))
}

func SetOutput(w io.Writer) {
	log.SetOutput(w)
}

func SetFlags(flags int) {
	log.SetFlags(flags)
}

func InitFromEnv() {
	SetLevelFromString(os.Getenv("LOG_LEVEL"))
}

func SetLevelFromString(level string) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		currentLevel.Store(int32(LevelDebug))
	case "warn", "warning":
		currentLevel.Store(int32(LevelWarn))
	case "error":
		currentLevel.Store(int32(LevelError))
	default:
		currentLevel.Store(int32(LevelInfo))
	}
}

func EnabledDebug() bool {
	return enabled(LevelDebug)
}

func CurrentLevelString() string {
	switch Level(currentLevel.Load()) {
	case LevelDebug:
		return "debug"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

func Debugf(format string, args ...any) {
	if enabled(LevelDebug) {
		outputf("DEBUG", format, args...)
	}
}

func Infof(format string, args ...any) {
	if enabled(LevelInfo) {
		outputf("INFO", format, args...)
	}
}

func Warnf(format string, args ...any) {
	if enabled(LevelWarn) {
		outputf("WARN", format, args...)
	}
}

func Errorf(format string, args ...any) {
	if enabled(LevelError) {
		outputf("ERROR", format, args...)
	}
}

func Fatalf(format string, args ...any) {
	outputf("FATAL", format, args...)
	os.Exit(1)
}

func enabled(level Level) bool {
	return level >= Level(currentLevel.Load())
}

func outputf(level string, format string, args ...any) {
	message := fmt.Sprintf("[%s] %s", level, fmt.Sprintf(format, args...))
	_ = log.Output(3, message)
}

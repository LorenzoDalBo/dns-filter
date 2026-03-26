package logger

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Level represents log severity.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

var levelFromString = map[string]Level{
	"debug": DEBUG,
	"info":  INFO,
	"warn":  WARN,
	"error": ERROR,
}

// Logger provides structured logging with levels (RNF04.4).
type Logger struct {
	mu    sync.Mutex
	level Level
}

var defaultLogger = &Logger{level: INFO}

// Init sets the global log level from config.
func Init(level string) {
	if l, ok := levelFromString[level]; ok {
		defaultLogger.level = l
	}
	defaultLogger.log(INFO, "Logger inicializado (nível: %s)", level)
}

// Debug logs a debug message (only shown when level=debug).
func Debug(format string, args ...interface{}) {
	defaultLogger.log(DEBUG, format, args...)
}

// Info logs an informational message.
func Info(format string, args ...interface{}) {
	defaultLogger.log(INFO, format, args...)
}

// Warn logs a warning message.
func Warn(format string, args ...interface{}) {
	defaultLogger.log(WARN, format, args...)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	defaultLogger.log(ERROR, format, args...)
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "%s [%s] %s\n", timestamp, levelNames[level], msg)
}
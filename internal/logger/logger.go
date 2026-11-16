// Package logger 提供应用运行期与启动期内存错误日志收集。
//
// 自动拦截标准库 log 输出，同时提供显式的 Error/Warn/Info 方法。
// 供状态栏日志按钮展示与弹框排查。
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Entry 是单条日志记录。
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
}

// RingLogger 是并发安全的环形内存日志缓冲区。
type RingLogger struct {
	mu      sync.RWMutex
	entries []Entry
	max     int
}

// NewRingLogger 创建一个保留最近 max 条记录的收集器。
func NewRingLogger(max int) *RingLogger {
	if max <= 0 {
		max = 500
	}
	return &RingLogger{max: max}
}

// Default 是全局默认日志收集器。
var Default = NewRingLogger(500)

func init() {
	// 挂载到标准库 log，同时输出到终端和内存环形缓冲
	log.SetOutput(io.MultiWriter(os.Stderr, &logWriter{l: Default}))
}

type logWriter struct {
	l *RingLogger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}
	lvl := LevelInfo
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "err") || strings.Contains(lower, "fail") || strings.Contains(lower, "panic") {
		lvl = LevelError
	} else if strings.Contains(lower, "warn") {
		lvl = LevelWarn
	}
	w.l.Add(lvl, msg)
	return len(p), nil
}

// Add 追加一条日志。
func (l *RingLogger) Add(level Level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := Entry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
	}
	if len(l.entries) >= l.max {
		l.entries = append(l.entries[1:], entry)
	} else {
		l.entries = append(l.entries, entry)
	}
}

// Entries 返回全部日志切片副本。
func (l *RingLogger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// ErrorCount 返回当前错误级别日志数。
func (l *RingLogger) ErrorCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	count := 0
	for _, e := range l.entries {
		if e.Level == LevelError {
			count++
		}
	}
	return count
}

// Clear 清空日志。
func (l *RingLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}

// FormatAll 把所有日志格式化为纯文本，供复制到剪贴板。
func (l *RingLogger) FormatAll() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.entries) == 0 {
		return "（暂无日志）"
	}
	var sb strings.Builder
	for _, e := range l.entries {
		sb.WriteString(fmt.Sprintf("[%s] [%s] %s\n",
			e.Time.Format("2006-01-02 15:04:05.000"),
			e.Level,
			e.Message,
		))
	}
	return sb.String()
}

// Error 记录一条错误日志。
func Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	Default.Add(LevelError, msg)
	fmt.Fprintf(os.Stderr, "[ERROR] %s\n", msg)
}

// Warn 记录一条告警日志。
func Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	Default.Add(LevelWarn, msg)
	fmt.Fprintf(os.Stderr, "[WARN] %s\n", msg)
}

// Info 记录一条常规日志。
func Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	Default.Add(LevelInfo, msg)
	fmt.Fprintf(os.Stderr, "[INFO] %s\n", msg)
}

// Entries 返回当前所有内存日志。
func Entries() []Entry {
	return Default.Entries()
}

// ErrorCount 返回当前错误条数。
func ErrorCount() int {
	return Default.ErrorCount()
}

// Clear 清空当前日志。
func Clear() {
	Default.Clear()
}

// FormatAll 返回全部格式化日志文本。
func FormatAll() string {
	return Default.FormatAll()
}

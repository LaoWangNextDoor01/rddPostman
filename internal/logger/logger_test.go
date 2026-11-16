package logger

import (
	"strings"
	"testing"
)

func TestLoggerRingAndLevels(t *testing.T) {
	l := NewRingLogger(3)
	l.Add(LevelInfo, "info msg")
	l.Add(LevelWarn, "warn msg")
	l.Add(LevelError, "err 1")
	l.Add(LevelError, "err 2")

	entries := l.Entries()
	if len(entries) != 3 {
		t.Fatalf("最多保留 3 条，得到 %d", len(entries))
	}
	// "info msg" 应该已被移出环形队列
	if entries[0].Message != "warn msg" {
		t.Errorf("首条应为 warn msg，得到 %q", entries[0].Message)
	}
	if l.ErrorCount() != 2 {
		t.Errorf("错误数应为 2，得到 %d", l.ErrorCount())
	}

	formatted := l.FormatAll()
	if !strings.Contains(formatted, "[ERROR] err 2") {
		t.Errorf("格式化文本缺少内容: %s", formatted)
	}

	l.Clear()
	if len(l.Entries()) != 0 || l.ErrorCount() != 0 {
		t.Errorf("清空后应为空: %v", l.Entries())
	}
}

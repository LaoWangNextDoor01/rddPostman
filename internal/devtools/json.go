// Package devtools 提供面向 API 调试的纯算法开发者小工具（JSON、编解码、哈希/HMAC、JWT、时间戳、UUID）。
// 本包不依赖任何 UI 组件，便于独立单元测试与复用。
package devtools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// JSONErrorInfo 表示解析 JSON 失败时的定位信息，供 UI 高亮错误行与内联诊断卡使用。
type JSONErrorInfo struct {
	Line    int
	Col     int
	Summary string
	Detail  string
}

// Error 实现 error 接口。
func (e *JSONErrorInfo) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("第 %d 行第 %d 列语法错误: %s", e.Line, e.Col, e.Detail)
	}
	return e.Detail
}

// FormatJSON 格式化 JSON 字符串，支持自定义缩进空格数与按键名递归排序。
func FormatJSON(src string, indent int, sortKeys bool) (string, error) {
	if indent <= 0 {
		indent = 2
	}
	return FormatJSONWithPad(src, strings.Repeat(" ", indent), sortKeys)
}

// FormatJSONWithPad 格式化 JSON 字符串，支持自定义缩进串（如 "  "、"    "、"\t"）与按键名递归排序。
func FormatJSONWithPad(src string, pad string, sortKeys bool) (string, error) {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return "", nil
	}
	if pad == "" {
		pad = "  "
	}

	if sortKeys {
		dec := json.NewDecoder(strings.NewReader(trimmed))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return "", describeJSONError(trimmed, err)
		}
		var buf bytes.Buffer
		if err := writeSortedJSON(&buf, v, 0, pad); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(trimmed), "", pad); err != nil {
		return "", describeJSONError(trimmed, err)
	}
	return buf.String(), nil
}

// MinifyJSON 压缩 JSON，移除多余空白与换行。
func MinifyJSON(src string) (string, error) {
	return MinifyJSONSorted(src, false)
}

// MinifyJSONSorted 压缩 JSON，支持可选按键名递归排序。
func MinifyJSONSorted(src string, sortKeys bool) (string, error) {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return "", nil
	}
	if sortKeys {
		formatted, err := FormatJSONWithPad(trimmed, "  ", true)
		if err != nil {
			return "", err
		}
		trimmed = formatted
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(trimmed)); err != nil {
		return "", describeJSONError(trimmed, err)
	}
	return buf.String(), nil
}

// EscapeJSON 将原文转义为可嵌入 JSON 字符串字面量内部的文本（不含外层双引号）。
func EscapeJSON(src string) string {
	if src == "" {
		return ""
	}
	b, _ := json.Marshal(src)
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return string(b[1 : len(b)-1])
	}
	return string(b)
}

// UnescapeJSON 将 JSON 转义字符串还原为原文，兼容带外层双引号与不带外层双引号两种输入。
func UnescapeJSON(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	trimmed := strings.TrimSpace(src)
	var out string
	if strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) && len(trimmed) >= 2 {
		if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
			return out, nil
		}
	}
	if err := json.Unmarshal([]byte(`"`+src+`"`), &out); err != nil {
		return "", &JSONErrorInfo{
			Summary: "JSON 反转义失败",
			Detail:  err.Error(),
		}
	}
	return out, nil
}

// ExtractJSONError 从错误中提取结构化行列定位信息。
func ExtractJSONError(err error) *JSONErrorInfo {
	if err == nil {
		return nil
	}
	var info *JSONErrorInfo
	if errors.As(err, &info) {
		return info
	}
	return &JSONErrorInfo{
		Summary: "SyntaxError",
		Detail:  err.Error(),
	}
}

func describeJSONError(src string, err error) error {
	var syn *json.SyntaxError
	if errors.As(err, &syn) && syn.Offset > 0 {
		line, col := offsetToLineCol(src, int(syn.Offset))
		return &JSONErrorInfo{
			Line:    line,
			Col:     col,
			Summary: fmt.Sprintf("SyntaxError · 第 %d 行第 %d 列", line, col),
			Detail:  syn.Error(),
		}
	}
	return &JSONErrorInfo{
		Summary: "SyntaxError",
		Detail:  err.Error(),
	}
}

func offsetToLineCol(s string, offset int) (int, int) {
	if offset > len(s) {
		offset = len(s)
	}
	line, col := 1, 1
	for i := 0; i < offset; i++ {
		if s[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

func writeSortedJSON(buf *bytes.Buffer, v any, depth int, pad string) error {
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			buf.WriteString("{}")
			return nil
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteString("{\n")
		for i, k := range keys {
			buf.WriteString(strings.Repeat(pad, depth+1))
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteString(": ")
			if err := writeSortedJSON(buf, val[k], depth+1, pad); err != nil {
				return err
			}
			if i < len(keys)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat(pad, depth))
		buf.WriteByte('}')
		return nil
	case []any:
		if len(val) == 0 {
			buf.WriteString("[]")
			return nil
		}
		buf.WriteString("[\n")
		for i, item := range val {
			buf.WriteString(strings.Repeat(pad, depth+1))
			if err := writeSortedJSON(buf, item, depth+1, pad); err != nil {
				return err
			}
			if i < len(val)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat(pad, depth))
		buf.WriteByte(']')
		return nil
	case json.Number:
		buf.WriteString(val.String())
		return nil
	case string:
		buf.WriteString(strconv.Quote(val))
		return nil
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}
}

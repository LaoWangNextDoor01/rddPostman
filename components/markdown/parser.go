package markdown

import (
	"regexp"
	"strconv"
	"strings"
)

// BlockType 表示 Markdown 块级元素类型。
type BlockType int

const (
	BlockHeading BlockType = iota
	BlockParagraph
	BlockCode
	BlockQuote
	BlockList
	BlockTable
	BlockThematicBreak
)

// Block 是解析出的块级元素。
type Block struct {
	Type    BlockType
	Level   int        // 标题级别 1-6
	Text    string     // 标题或段落文本
	Lang    string     // 代码块语言
	Code    string     // 代码块内容
	Quote   []string   // 引用内容行
	Ordered bool       // 是否有序列表
	Items   []ListItem // 列表项
	Headers []string   // 表头
	Aligns  []string   // 列对齐方式 ("left", "center", "right")
	Rows    [][]string // 表格数据行
}

// ListItem 列表项。
type ListItem struct {
	Text    string
	IsTask  bool
	Checked bool
}

// InlineSegment 表示行内富文本片段。
type InlineSegment struct {
	Text   string
	Bold   bool
	Italic bool
	Strike bool
	Code   bool
	Link   string // 为空表示非链接，非空表示 URL
}

// ParseBlocks 将 markdown 字符串按行扫描解析为块级切片。
func ParseBlocks(src string) []Block {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var blocks []Block
	i := 0
	n := len(lines)

	for i < n {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// 1. 空行跳过
		if trimmed == "" {
			i++
			continue
		}

		// 2. 代码块 ``` 或 ~~~
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence := trimmed[:3]
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, fence))
			i++
			var codeLines []string
			for i < n {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), fence) {
					i++
					break
				}
				codeLines = append(codeLines, lines[i])
				i++
			}
			blocks = append(blocks, Block{
				Type: BlockCode,
				Lang: lang,
				Code: strings.Join(codeLines, "\n"),
			})
			continue
		}

		// 3. 标题 # ... ######
		if headingLevel, headingText := parseHeading(trimmed); headingLevel > 0 {
			blocks = append(blocks, Block{
				Type:  BlockHeading,
				Level: headingLevel,
				Text:  headingText,
			})
			i++
			continue
		}

		// 4. 水平分割线 ---, ***, ___
		if isThematicBreak(trimmed) {
			blocks = append(blocks, Block{Type: BlockThematicBreak})
			i++
			continue
		}

		// 5. 引用块 >
		if strings.HasPrefix(trimmed, ">") {
			var quoteLines []string
			for i < n {
				cur := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(cur, ">") {
					break
				}
				qText := strings.TrimPrefix(cur, ">")
				if strings.HasPrefix(qText, " ") {
					qText = qText[1:]
				}
				quoteLines = append(quoteLines, qText)
				i++
			}
			blocks = append(blocks, Block{
				Type:  BlockQuote,
				Quote: quoteLines,
			})
			continue
		}

		// 6. 表格 (首行含 | 且次行是分隔线 |---|---|)
		if strings.Contains(trimmed, "|") && i+1 < n && isTableDelimiter(strings.TrimSpace(lines[i+1])) {
			headers := parseTableRow(trimmed)
			aligns := parseTableAligns(strings.TrimSpace(lines[i+1]))
			i += 2
			var rows [][]string
			for i < n {
				rowTrimmed := strings.TrimSpace(lines[i])
				if !strings.Contains(rowTrimmed, "|") || rowTrimmed == "" {
					break
				}
				rows = append(rows, parseTableRow(rowTrimmed))
				i++
			}
			blocks = append(blocks, Block{
				Type:    BlockTable,
				Headers: headers,
				Aligns:  aligns,
				Rows:    rows,
			})
			continue
		}

		// 7. 列表项 (- , * , + , 1. , 2. )
		if isListStart(trimmed) {
			ordered := isOrderedListStart(trimmed)
			var items []ListItem
			for i < n {
				cur := strings.TrimSpace(lines[i])
				if cur == "" {
					break
				}
				if ordered && isOrderedListStart(cur) {
					dot := strings.Index(cur, ".")
					itemText := strings.TrimSpace(cur[dot+1:])
					item, task, checked := checkTask(itemText)
					items = append(items, ListItem{Text: item, IsTask: task, Checked: checked})
					i++
				} else if !ordered && isUnorderedListStart(cur) {
					itemText := strings.TrimSpace(cur[1:])
					item, task, checked := checkTask(itemText)
					items = append(items, ListItem{Text: item, IsTask: task, Checked: checked})
					i++
				} else {
					// 连续行作为上一个列表项的延续
					if len(items) > 0 {
						items[len(items)-1].Text += " " + cur
						i++
					} else {
						break
					}
				}
			}
			blocks = append(blocks, Block{
				Type:    BlockList,
				Ordered: ordered,
				Items:   items,
			})
			continue
		}

		// 8. 普通段落：收集直到遇到空行或其他块起始
		var paraLines []string
		for i < n {
			cur := strings.TrimSpace(lines[i])
			if cur == "" {
				break
			}
			if strings.HasPrefix(cur, "```") || strings.HasPrefix(cur, "~~~") ||
				isHeading(cur) || isThematicBreak(cur) || strings.HasPrefix(cur, ">") ||
				isListStart(cur) || (strings.Contains(cur, "|") && i+1 < n && isTableDelimiter(strings.TrimSpace(lines[i+1]))) {
				break
			}
			paraLines = append(paraLines, cur)
			i++
		}
		if len(paraLines) > 0 {
			blocks = append(blocks, Block{
				Type: BlockParagraph,
				Text: strings.Join(paraLines, "\n"),
			})
		}
	}

	return blocks
}

func parseHeading(s string) (int, string) {
	for level := 1; level <= 6; level++ {
		prefix := strings.Repeat("#", level) + " "
		if strings.HasPrefix(s, prefix) {
			return level, strings.TrimSpace(s[len(prefix):])
		}
	}
	return 0, ""
}

func isHeading(s string) bool {
	l, _ := parseHeading(s)
	return l > 0
}

func isThematicBreak(s string) bool {
	clean := strings.ReplaceAll(s, " ", "")
	if len(clean) < 3 {
		return false
	}
	ch := clean[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	for i := 0; i < len(clean); i++ {
		if clean[i] != ch {
			return false
		}
	}
	return true
}

func isUnorderedListStart(s string) bool {
	return (strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "+ "))
}

func isOrderedListStart(s string) bool {
	dot := strings.Index(s, ".")
	if dot <= 0 || dot > 5 || dot+1 >= len(s) || s[dot+1] != ' ' {
		return false
	}
	_, err := strconv.Atoi(s[:dot])
	return err == nil
}

func isListStart(s string) bool {
	return isUnorderedListStart(s) || isOrderedListStart(s)
}

func checkTask(s string) (text string, isTask bool, checked bool) {
	if strings.HasPrefix(s, "[ ] ") {
		return s[4:], true, false
	}
	if strings.HasPrefix(s, "[x] ") || strings.HasPrefix(s, "[X] ") {
		return s[4:], true, true
	}
	return s, false, false
}

func isTableDelimiter(s string) bool {
	if !strings.Contains(s, "|") {
		return false
	}
	cells := parseTableRow(s)
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		trimmed := strings.Trim(c, ":- ")
		if trimmed != "" {
			return false
		}
		if !strings.Contains(c, "-") {
			return false
		}
	}
	return true
}

func parseTableRow(s string) []string {
	s = strings.Trim(s, "|")
	parts := strings.Split(s, "|")
	res := make([]string, len(parts))
	for i, p := range parts {
		res[i] = strings.TrimSpace(p)
	}
	return res
}

func parseTableAligns(s string) []string {
	cells := parseTableRow(s)
	aligns := make([]string, len(cells))
	for i, c := range cells {
		c = strings.TrimSpace(c)
		left := strings.HasPrefix(c, ":")
		right := strings.HasSuffix(c, ":")
		switch {
		case left && right:
			aligns[i] = "center"
		case right:
			aligns[i] = "right"
		default:
			aligns[i] = "left"
		}
	}
	return aligns
}

// ParseInline 解析行内格式，输出富文本片段切片。
var inlineTokenRe = regexp.MustCompile("(?s)(`[^`]+`|\\[[^\\]]+\\]\\([^\\)]+\\)|\\*\\*\\*[^*]+\\*\\*\\*|\\*\\*[^*]+\\*\\*|\\*[^*]+\\*|~~[^~]+~~)")

func ParseInline(s string) []InlineSegment {
	var segs []InlineSegment
	last := 0
	matches := inlineTokenRe.FindAllStringIndex(s, -1)

	for _, loc := range matches {
		start, end := loc[0], loc[1]
		if start > last {
			segs = append(segs, InlineSegment{Text: s[last:start]})
		}
		token := s[start:end]
		switch {
		// 行内代码 `code`
		case strings.HasPrefix(token, "`") && strings.HasSuffix(token, "`") && len(token) >= 2:
			segs = append(segs, InlineSegment{
				Text: token[1 : len(token)-1],
				Code: true,
			})
		// 链接 [text](url)
		case strings.HasPrefix(token, "[") && strings.HasSuffix(token, ")"):
			mid := strings.Index(token, "](")
			if mid > 0 {
				linkText := token[1:mid]
				linkURL := token[mid+2 : len(token)-1]
				segs = append(segs, InlineSegment{
					Text: linkText,
					Link: linkURL,
				})
			} else {
				segs = append(segs, InlineSegment{Text: token})
			}
		// 粗斜体 ***text***
		case strings.HasPrefix(token, "***") && strings.HasSuffix(token, "***") && len(token) >= 6:
			segs = append(segs, InlineSegment{
				Text:   token[3 : len(token)-3],
				Bold:   true,
				Italic: true,
			})
		// 粗体 **text**
		case strings.HasPrefix(token, "**") && strings.HasSuffix(token, "**") && len(token) >= 4:
			segs = append(segs, InlineSegment{
				Text: token[2 : len(token)-2],
				Bold: true,
			})
		// 斜体 *text*
		case strings.HasPrefix(token, "*") && strings.HasSuffix(token, "*") && len(token) >= 2:
			segs = append(segs, InlineSegment{
				Text:   token[1 : len(token)-1],
				Italic: true,
			})
		// 删除线 ~~text~~
		case strings.HasPrefix(token, "~~") && strings.HasSuffix(token, "~~") && len(token) >= 4:
			segs = append(segs, InlineSegment{
				Text:   token[2 : len(token)-2],
				Strike: true,
			})
		default:
			segs = append(segs, InlineSegment{Text: token})
		}
		last = end
	}

	if last < len(s) {
		segs = append(segs, InlineSegment{Text: s[last:]})
	}

	return segs
}

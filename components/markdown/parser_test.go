package markdown

import (
	"reflect"
	"testing"
)

func TestParseBlocks(t *testing.T) {
	doc := `# 集合标题

这是第一段介绍，包含了 **粗体** 和 ` + "`行内代码`" + `。

## 接口说明
- [ ] 待实现任务
- [x] 已完成任务
- 普通列表项 1
- 普通列表项 2

> 这是引用说明内容
> 支持多行引用

` + "```json\n{\n  \"status\": \"ok\"\n}\n```" + `

---

| 参数 | 类型 | 必填 | 说明 |
| :--- | :---: | ---: | --- |
| id | string | 是 | 用户唯一标识 |
| age | int | 否 | 用户年龄 |
`

	blocks := ParseBlocks(doc)
	if len(blocks) == 0 {
		t.Fatal("解析块为空")
	}

	// 1. Heading 1
	if blocks[0].Type != BlockHeading || blocks[0].Level != 1 || blocks[0].Text != "集合标题" {
		t.Errorf("Block 0 mismatch: %+v", blocks[0])
	}

	// 2. Paragraph
	if blocks[1].Type != BlockParagraph {
		t.Errorf("Block 1 should be paragraph: %+v", blocks[1])
	}

	// 3. Heading 2
	if blocks[2].Type != BlockHeading || blocks[2].Level != 2 || blocks[2].Text != "接口说明" {
		t.Errorf("Block 2 mismatch: %+v", blocks[2])
	}

	// 4. List
	if blocks[3].Type != BlockList || len(blocks[3].Items) != 4 {
		t.Fatalf("Block 3 should have 4 list items: %+v", blocks[3])
	}
	if !blocks[3].Items[0].IsTask || blocks[3].Items[0].Checked {
		t.Errorf("Task 0 should be unchecked: %+v", blocks[3].Items[0])
	}
	if !blocks[3].Items[1].IsTask || !blocks[3].Items[1].Checked {
		t.Errorf("Task 1 should be checked: %+v", blocks[3].Items[1])
	}

	// 5. Quote
	if blocks[4].Type != BlockQuote || len(blocks[4].Quote) != 2 {
		t.Errorf("Block 4 should be quote with 2 lines: %+v", blocks[4])
	}

	// 6. Code
	if blocks[5].Type != BlockCode || blocks[5].Lang != "json" {
		t.Errorf("Block 5 should be json code block: %+v", blocks[5])
	}

	// 7. Thematic Break
	if blocks[6].Type != BlockThematicBreak {
		t.Errorf("Block 6 should be thematic break: %+v", blocks[6])
	}

	// 8. Table
	if blocks[7].Type != BlockTable {
		t.Fatalf("Block 7 should be table: %+v", blocks[7])
	}
	if len(blocks[7].Headers) != 4 || len(blocks[7].Rows) != 2 {
		t.Errorf("Table mismatch headers=%v rows=%v", blocks[7].Headers, blocks[7].Rows)
	}
	expectedAligns := []string{"left", "center", "right", "left"}
	if !reflect.DeepEqual(blocks[7].Aligns, expectedAligns) {
		t.Errorf("Table aligns expected %v, got %v", expectedAligns, blocks[7].Aligns)
	}
}

func TestParseInline(t *testing.T) {
	text := "这里有 **粗体** 以及 *斜体*，还有 `代码块` 与 [链接文字](https://example.com) 和 ~~删除线~~。"
	segs := ParseInline(text)

	var foundBold, foundItalic, foundCode, foundLink, foundStrike bool
	for _, s := range segs {
		if s.Bold && s.Text == "粗体" {
			foundBold = true
		}
		if s.Italic && s.Text == "斜体" {
			foundItalic = true
		}
		if s.Code && s.Text == "代码块" {
			foundCode = true
		}
		if s.Link == "https://example.com" && s.Text == "链接文字" {
			foundLink = true
		}
		if s.Strike && s.Text == "删除线" {
			foundStrike = true
		}
	}

	if !foundBold {
		t.Error("未找到粗体")
	}
	if !foundItalic {
		t.Error("未找到斜体")
	}
	if !foundCode {
		t.Error("未找到行内代码")
	}
	if !foundLink {
		t.Error("未找到链接")
	}
	if !foundStrike {
		t.Error("未找到删除线")
	}
}

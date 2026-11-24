package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultSolarVersion = "1.2.17"
	solarPackageBase    = "https://unpkg.com/@iconify-json/solar@%s"
	solarStampFile      = ".solar-version"
	legacyLucideStamp   = ".lucide-version"
)

// iconEntry 表示 scripts/icons.txt 中的一条本地名与 Solar 图标名映射。
type iconEntry struct {
	Local string
	Solar string
}

type iconifySet struct {
	Prefix  string                       `json:"prefix"`
	Width   int                          `json:"width"`
	Height  int                          `json:"height"`
	Icons   map[string]iconifyIcon       `json:"icons"`
	Aliases map[string]iconifyAliasEntry `json:"aliases"`
}

type iconifyIcon struct {
	Body   string `json:"body"`
	Left   int    `json:"left"`
	Top    int    `json:"top"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type iconifyAliasEntry struct {
	Parent string `json:"parent"`
	Left   int    `json:"left"`
	Top    int    `json:"top"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type iconifyInfo struct {
	Name   string `json:"name"`
	Author struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"author"`
	License struct {
		Title string `json:"title"`
		SPDX  string `json:"spdx"`
		URL   string `json:"url"`
	} `json:"license"`
}

// parseIconManifest 解析 scripts/icons.txt，支持 "local = solar" 与裸 "name" 两种写法。
func parseIconManifest(raw string) ([]iconEntry, error) {
	lines := strings.Split(raw, "\n")
	entries := make([]iconEntry, 0, len(lines))
	seen := map[string]int{}

	for idx, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var localName, solarName string
		if left, right, ok := strings.Cut(line, "="); ok {
			localName = strings.TrimSpace(left)
			solarName = strings.TrimSpace(right)
		} else {
			localName = line
			solarName = line
		}
		if localName == "" || solarName == "" {
			return nil, fmt.Errorf("第 %d 行格式无效: %q（应为 \"local = solar\" 或 \"name\"）", idx+1, line)
		}
		if prevLine, dup := seen[localName]; dup {
			return nil, fmt.Errorf("第 %d 行重复定义了本地图标名 %q（首次出现在第 %d 行）", idx+1, localName, prevLine)
		}
		seen[localName] = idx + 1
		entries = append(entries, iconEntry{Local: localName, Solar: solarName})
	}
	return entries, nil
}

// buildIconSVG 根据 Iconify 图标集中的条目构造标准 24x24 SVG。
func buildIconSVG(set *iconifySet, solarName string) (string, error) {
	defaultW := set.Width
	if defaultW <= 0 {
		defaultW = 24
	}
	defaultH := set.Height
	if defaultH <= 0 {
		defaultH = 24
	}

	cur := solarName
	var overrideW, overrideH, overrideLeft, overrideTop int
	for depth := 0; depth < 8; depth++ {
		if ic, ok := set.Icons[cur]; ok {
			w := firstPositive(overrideW, ic.Width, defaultW)
			h := firstPositive(overrideH, ic.Height, defaultH)
			left := overrideLeft
			if left == 0 {
				left = ic.Left
			}
			top := overrideTop
			if top == 0 {
				top = ic.Top
			}
			return fmt.Sprintf(
				`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="%d %d %d %d">%s</svg>`,
				w, h, left, top, w, h, strings.TrimSpace(ic.Body),
			), nil
		}
		alias, ok := set.Aliases[cur]
		if !ok || alias.Parent == "" {
			break
		}
		if overrideW == 0 {
			overrideW = alias.Width
		}
		if overrideH == 0 {
			overrideH = alias.Height
		}
		if overrideLeft == 0 {
			overrideLeft = alias.Left
		}
		if overrideTop == 0 {
			overrideTop = alias.Top
		}
		cur = alias.Parent
	}
	return "", fmt.Errorf("在 @iconify-json/solar 中未找到图标 %q", solarName)
}

func firstPositive(vals ...int) int {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 24
}

func syncIcons(version string, force bool) error {
	dir := filepath.Join("components", "v1", "icons", "assets")
	listPath := filepath.Join("scripts", "icons.txt")
	stampPath := filepath.Join(dir, solarStampFile)
	licensePath := filepath.Join(dir, "LICENSE")

	rawList, err := os.ReadFile(listPath)
	if err != nil {
		return fmt.Errorf("读取清单 %s: %w", listPath, err)
	}
	entries, err := parseIconManifest(string(rawList))
	if err != nil {
		return fmt.Errorf("解析清单 %s: %w", listPath, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("清单 %s 为空", listPath)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建资产目录 %s: %w", dir, err)
	}

	sameVersion := false
	if !force {
		if b, err := os.ReadFile(stampPath); err == nil && strings.TrimSpace(string(b)) == version {
			sameVersion = true
		}
	}

	needFetch := force || !sameVersion
	if !needFetch {
		if _, err := os.Stat(licensePath); err != nil {
			needFetch = true
		}
	}
	if !needFetch {
		for _, e := range entries {
			if _, err := os.Stat(filepath.Join(dir, e.Local+".svg")); err != nil {
				needFetch = true
				break
			}
		}
	}

	if needFetch {
		base := fmt.Sprintf(solarPackageBase, version)
		fmt.Printf("正在拉取 @iconify-json/solar@%s ...\n", version)

		iconsData, err := fetchURL(base + "/icons.json")
		if err != nil {
			return fmt.Errorf("下载 Solar icons.json: %w", err)
		}
		var set iconifySet
		if err := json.Unmarshal(iconsData, &set); err != nil {
			return fmt.Errorf("解析 Solar icons.json: %w", err)
		}

		updated := 0
		for _, e := range entries {
			svg, err := buildIconSVG(&set, e.Solar)
			if err != nil {
				return fmt.Errorf("图标 %q -> %q: %w", e.Local, e.Solar, err)
			}
			target := filepath.Join(dir, e.Local+".svg")
			if old, err := os.ReadFile(target); err == nil && strings.TrimSpace(string(old)) == svg {
				continue
			}
			if err := os.WriteFile(target, []byte(svg), 0o644); err != nil {
				return fmt.Errorf("写入 %s: %w", target, err)
			}
			updated++
			fmt.Printf("已同步 %s.svg <- solar:%s\n", e.Local, e.Solar)
		}
		if updated == 0 {
			fmt.Println("所有清单图标 SVG 内容均已最新，无需覆写")
		}

		infoData, err := fetchURL(base + "/info.json")
		if err != nil {
			return fmt.Errorf("下载 Solar info.json: %w", err)
		}
		var info iconifyInfo
		if err := json.Unmarshal(infoData, &info); err != nil {
			return fmt.Errorf("解析 Solar info.json: %w", err)
		}
		licText := formatSolarLicense(version, info)
		if old, err := os.ReadFile(licensePath); err != nil || string(old) != licText {
			if err := os.WriteFile(licensePath, []byte(licText), 0o644); err != nil {
				return fmt.Errorf("写入 %s: %w", licensePath, err)
			}
		}
	}

	// 清理清单外的 .svg，保证打包资产与 scripts/icons.txt 严格一致
	wanted := make(map[string]bool, len(entries))
	for _, e := range entries {
		wanted[e.Local] = true
	}
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("扫描资产目录: %w", err)
	}
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		if name, ok := strings.CutSuffix(de.Name(), ".svg"); ok && !wanted[name] {
			if err := os.Remove(filepath.Join(dir, de.Name())); err != nil {
				return fmt.Errorf("删除多余图标 %s: %w", de.Name(), err)
			}
			fmt.Printf("已删除 %s（不在清单中）\n", de.Name())
		}
	}

	// 移除旧 Lucide 版本戳并写入 Solar 版本戳
	_ = os.Remove(filepath.Join(dir, legacyLucideStamp))
	if err := os.WriteFile(stampPath, []byte(version+"\n"), 0o644); err != nil {
		return fmt.Errorf("写入 %s: %w", stampPath, err)
	}

	if err := run(); err != nil {
		return err
	}
	fmt.Printf("完成，图标目录与 %s 一致（Solar @iconify-json/solar %s，共 %d 个图标）。\n", listPath, version, len(entries))
	return nil
}

func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s 返回 HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func formatSolarLicense(version string, info iconifyInfo) string {
	author := info.Author.Name
	if author == "" {
		author = "480 Design"
	}
	authorURL := info.Author.URL
	if authorURL == "" {
		authorURL = "https://www.figma.com/community/file/1166831539721848736"
	}
	licTitle := info.License.Title
	if licTitle == "" {
		licTitle = "CC BY 4.0"
	}
	licSPDX := info.License.SPDX
	if licSPDX == "" {
		licSPDX = "CC-BY-4.0"
	}
	licURL := info.License.URL
	if licURL == "" {
		licURL = "https://creativecommons.org/licenses/by/4.0/"
	}
	return fmt.Sprintf(
		"Solar Icons (packaged via @iconify-json/solar@%s)\n\nAuthor: %s\nSource: %s\nLicense: %s (%s)\nLicense URL: %s\n",
		version, author, authorURL, licTitle, licSPDX, licURL,
	)
}

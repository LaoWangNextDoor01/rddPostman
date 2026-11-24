package collection

import (
	"sync"

	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo"
)

// WorkbenchCommandKind 描述外部（菜单、全局搜索、快捷键）发往工作台的一次性指令。
type WorkbenchCommandKind int

const (
	CmdNone WorkbenchCommandKind = iota
	CmdNewRequest
	CmdNewCollection
	CmdImportPostman
	CmdImportCurl
	CmdOpenTab
	CmdOpenEnv
	CmdOpenTool
	CmdCloseActiveTab
	CmdNextTab
	CmdPrevTab
	CmdFocusURL
)

// WorkbenchCommand 封装一次发往工作台的指令及目标标识。
type WorkbenchCommand struct {
	Kind     WorkbenchCommandKind
	TargetID string
	EnvID    domain.ID
	ToolID   string
}

// binaryFilePickResult 在后台 goroutine 与 UI 帧之间安全同步 Binary Body 的文件选择结果。
type binaryFilePickResult struct {
	mu      sync.Mutex
	pending bool
	picking bool
	path    string
	err     string
}

func (p *binaryFilePickResult) start() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.picking {
		return false
	}
	p.picking = true
	return true
}

func (p *binaryFilePickResult) finish(path, errMsg string) {
	p.mu.Lock()
	p.picking = false
	p.pending = true
	p.path = path
	p.err = errMsg
	p.mu.Unlock()
}

func (p *binaryFilePickResult) drain() (path, errMsg string, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.pending {
		return "", "", false
	}
	p.pending = false
	path, errMsg = p.path, p.err
	p.path, p.err = "", ""
	return path, errMsg, true
}

// openBinaryFilePicker 负责弹出系统文件选择框；测试中可替换以避免阻塞系统弹窗。
var openBinaryFilePicker = defaultOpenBinaryFilePicker

func defaultOpenBinaryFilePicker() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title: locales.BodySelectBinaryTitle(),
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

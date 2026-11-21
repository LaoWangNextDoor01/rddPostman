// Package storage 定义持久化后端的边界。
//
// 上层 store 只依赖这里的接口，不关心数据放在 JSON 文件、SQLite 还是远端。
// 当前实现见 storage/jsonfs：工作区对应一个真实目录，目录/请求对应真实文件，
// 这样可以直接用 Git、iCloud 或整目录拷贝来管理数据。
package storage

import (
	"context"
	"errors"
	"time"

	"rddpostman/internal/domain"
)

var (
	// ErrNotFound 表示工作区、集合、节点或环境不存在。
	ErrNotFound = errors.New("storage: 目标不存在")
	// ErrExists 表示目标位置已有同名项。
	ErrExists = errors.New("storage: 同名项已存在")
	// ErrDefaultWorkspace 表示试图移除系统默认工作区。
	ErrDefaultWorkspace = errors.New("storage: 默认工作区不可移除")
	// ErrNewerVersion 表示数据由更高版本的客户端写入。
	ErrNewerVersion = errors.New("storage: 数据由更高版本写入")
	// ErrReadOnly 表示工作区里有当前版本无法安全改写的数据。
	ErrReadOnly = errors.New("storage: 工作区处于只读状态")
)

// Ref 是注册表里的一条工作区记录。
// Path 可以是数据根目录下的相对路径，也可以是用户自选目录的绝对路径。
type Ref struct {
	ID      domain.ID
	Path    string
	Default bool
}

// Input 是新建集合的参数。
type Input struct {
	Name        string
	Description string
}

// HistoryQuery 是查询历史记录的参数。
type HistoryQuery struct {
	Limit  int       // 最多返回条数，0 表示不限
	Before time.Time // 只返回此时间之前的记录，零值表示不过滤
}

// Bundle 是一个工作区的完整内存视图，由后端一次性加载。
type Bundle struct {
	Ref          Ref
	Workspace    *domain.Workspace
	Collections  []*domain.Collection
	Environments []domain.Environment
	Local        domain.Local
	Secrets      map[domain.ID]string
	// Problems 记录加载时跳过的损坏或高版本文件，UI 可以提示用户。
	Problems []string
	// ReadOnly 表示存在当前版本无法安全改写的数据，所有写操作都应被拒绝。
	ReadOnly bool
}

// Backend 是存储实现的边界。所有方法都应保证原子写，失败时不能留下半个文件。
type Backend interface {
	// List 返回工作区注册表；首次运行时会创建默认工作区。
	// 旧版本数据在这里完成迁移。
	List(ctx context.Context) ([]Ref, error)

	// Load 读取一个工作区的全部文档。
	Load(ctx context.Context, ref Ref) (*Bundle, error)

	// CreateWorkspace 新建工作区；dir 为空时由后端在数据根目录下分配。
	CreateWorkspace(ctx context.Context, name, dir string) (*Bundle, error)
	UpdateWorkspace(ctx context.Context, ref Ref, ws *domain.Workspace) error
	// SaveWorkspaceIcon 将源图片复制到工作区目录（使用专属前缀避免与业务/项目文件冲突），
	// 返回存进 workspace.json 的相对文件名；srcPath 为空时清理工作区已有图标。
	SaveWorkspaceIcon(ctx context.Context, ref Ref, srcPath string) (string, error)
	// WorkspaceDir 返回工作区目录在磁盘上的绝对路径，用于解析工作区内相对资源路径。
	WorkspaceDir(ref Ref) string
	// RemoveWorkspace 只取消注册，磁盘上的目录保持原样，方便以后重新打开。
	RemoveWorkspace(ctx context.Context, ref Ref) error

	CreateCollection(ctx context.Context, ref Ref, in Input) (*domain.Collection, error)
	DeleteCollection(ctx context.Context, ref Ref, id domain.ID) error
	// SaveCollection 只写集合自身的元数据（名称、描述、变量、认证、顺序）。
	SaveCollection(ctx context.Context, ref Ref, c *domain.Collection) error

	// SaveNode 新建或更新一个目录/请求；parentID 只在新建时使用。
	SaveNode(ctx context.Context, ref Ref, collID, parentID domain.ID, n *domain.Node) error
	DeleteNode(ctx context.Context, ref Ref, collID, id domain.ID) error

	SaveEnvironment(ctx context.Context, ref Ref, env domain.Environment) error
	DeleteEnvironment(ctx context.Context, ref Ref, id domain.ID) error
	SaveLocal(ctx context.Context, ref Ref, local domain.Local) error
	SaveSecrets(ctx context.Context, ref Ref, values map[domain.ID]string) error

	LoadGlobals(ctx context.Context) (domain.Globals, error)
	SaveGlobals(ctx context.Context, g domain.Globals) error

	// AppendHistory 追加一条历史记录。
	AppendHistory(ctx context.Context, ref Ref, entry domain.HistoryEntry) error
	// LoadHistory 按时间倒序返回历史记录。
	LoadHistory(ctx context.Context, ref Ref, query HistoryQuery) ([]domain.HistoryEntry, error)
	// ClearHistory 清空工作区的所有历史记录。
	ClearHistory(ctx context.Context, ref Ref) error
}

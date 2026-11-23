// Package routes 定义应用的分组 Layout 路由模板与路径构造器，供外壳、侧栏和页面共用。
package routes

const (
	// 分组 Layout 路由模板（MyGo Router 原生嵌套 Layout，按前缀段保活外层壳）
	HubPattern       = "/hub/{section...}"
	WorkbenchPattern = "/w/{wsID}/{section...}"

	// (hub) 全局管理分组路由
	HubHome           = "/hub/home"
	HubWorkspaces     = "/hub/workspaces"
	HubWorkspaceNew   = "/hub/workspaces/new"
	HubInventory      = "/hub/inventory"
	HubAPICatalog     = "/hub/catalog"
	HubPrivateNetwork = "/hub/private-network"
	HubPublicNetwork  = "/hub/public-network"
	HubAgentContext   = "/hub/agent-context"
	HubIntegrations   = "/hub/integrations"
	HubReports        = "/hub/reports"

	// (hub) 相对子路由（供 HubLayout 内部 r.View 匹配）
	SubHome           = "/home"
	SubWorkspaces     = "/workspaces"
	SubWorkspaceNew   = "/workspaces/new"
	SubInventory      = "/inventory"
	SubAPICatalog     = "/catalog"
	SubPrivateNetwork = "/private-network"
	SubPublicNetwork  = "/public-network"
	SubAgentContext   = "/agent-context"
	SubIntegrations   = "/integrations"
	SubReports        = "/reports"

	// (workbench) 相对子路由（供 WorkbenchLayout 左侧面板 r.View 匹配）
	SectionCollections  = "/collections"
	SectionHistory      = "/history"
	SectionEnvironments = "/environments"
	SectionTools        = "/tools"
	SectionSpecs        = "/specs"
	SectionDocs         = "/docs"

	// 演示页
	Demo = "/demo"
)

// WorkbenchCollectionsOf 返回指定工作空间的集合工作台路径。
func WorkbenchCollectionsOf(wsID string) string { return "/w/" + wsID + SectionCollections }

// WorkbenchHistoryOf 返回指定工作空间的请求历史侧栏路径。
func WorkbenchHistoryOf(wsID string) string { return "/w/" + wsID + SectionHistory }

// WorkbenchEnvironmentsOf 返回指定工作空间的环境变量侧栏路径。
func WorkbenchEnvironmentsOf(wsID string) string { return "/w/" + wsID + SectionEnvironments }

// WorkbenchToolsOf 返回指定工作空间的开发者小工具侧栏路径。
func WorkbenchToolsOf(wsID string) string { return "/w/" + wsID + SectionTools }

// WorkbenchSpecsOf 返回指定工作空间的规范/文档侧栏路径。
func WorkbenchSpecsOf(wsID string) string { return "/w/" + wsID + SectionSpecs }

// WorkbenchDocsOf 返回指定工作空间的文档侧栏路径。
func WorkbenchDocsOf(wsID string) string { return "/w/" + wsID + SectionDocs }

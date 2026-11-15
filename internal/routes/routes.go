// Package routes 定义应用的页面路径，供外壳、侧栏和页面共用。
package routes

const (
	Home        = "/home"
	Workspace   = "/workspace"
	Collections = "/collections"
	// Collection 是集合内页面的路由模板。
	Collection   = "/collections/{id}"
	Environments = "/environments"
	History      = "/history"
	Docs         = "/docs"
	Mock         = "/mock"
	Network      = "/network"
	Demo         = "/demo"
	Settings     = "/settings"
)

// CollectionOf 返回一个集合的路由。
func CollectionOf(id string) string { return Collections + "/" + id }

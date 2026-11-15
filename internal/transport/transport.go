// Package transport 负责构建并发送 HTTP 请求、解析响应。
// 运行在后台 goroutine，结果交回 app 触发重绘，不阻塞主线程。
package transport

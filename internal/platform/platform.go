// Package platform 封装原生能力：应用数据目录、文件选择、剪贴板、通知。
package platform

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// DataDir 返回应用数据目录，不存在时创建。
// macOS 上是 ~/Library/Application Support/rddPostman；
// 测试可以用 RDDPOSTMAN_DATA_DIR 指到临时目录，避免碰真实数据。
func DataDir() (string, error) {
	if dir := os.Getenv("RDDPOSTMAN_DATA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "rddPostman")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// PickDirectory 打开系统目录选择器，返回用户选的目录；取消时返回空串。
// 它会阻塞到对话框关闭，必须在 goroutine 里调用，完成后再 Invalidate 重绘。
func PickDirectory(title, defaultPath string) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:             title,
		DefaultPath:       defaultPath,
		Directory:         true,
		CreateDirectories: true,
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

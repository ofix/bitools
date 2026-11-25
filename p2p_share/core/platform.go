package core

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetDefaultShareDirs 获取系统默认共享目录
func GetDefaultShareDirs() []string {
	var dirs []string
	home, err := os.UserHomeDir()
	if err != nil {
		return dirs
	}

	switch runtime.GOOS {
	case "windows":
		dirs = append(dirs, filepath.Join(home, "Desktop"))
		dirs = append(dirs, filepath.Join(home, "Documents"))
		dirs = append(dirs, filepath.Join(home, "Downloads"))
	case "darwin":
		dirs = append(dirs, filepath.Join(home, "Desktop"))
		dirs = append(dirs, filepath.Join(home, "Documents"))
		dirs = append(dirs, filepath.Join(home, "Downloads"))
	case "linux":
		dirs = append(dirs, filepath.Join(home, "Desktop"))
		dirs = append(dirs, filepath.Join(home, "Documents"))
		dirs = append(dirs, filepath.Join(home, "Downloads"))
	}
	return dirs
}

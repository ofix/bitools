package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type ShareManager struct {
	mu     sync.RWMutex
	shares map[string]string // virtualName -> realPath
}

func NewShareManager() *ShareManager {
	return &ShareManager{
		shares: make(map[string]string),
	}
}

func (sm *ShareManager) AddShare(name, path string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.shares[name] = path
}

func (sm *ShareManager) RemoveShare(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.shares, name)
}

func (sm *ShareManager) GetSharesList() map[string]string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range sm.shares {
		result[k] = v
	}
	return result
}

// ListDirectory 列出共享目录下的内容 (支持分页)
func (sm *ShareManager) ListDirectory(virtualPath string, offset, limit int) (*ListResponse, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 解析虚拟路径: /ShareName/SubDir
	parts := strings.SplitN(strings.TrimPrefix(virtualPath, "/"), "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		// 返回根目录 (所有共享)
		var entries []FileEntry
		for name := range sm.shares {
			entries = append(entries, FileEntry{
				Name:  name,
				Path:  "/" + name,
				IsDir: true,
			})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

		total := len(entries)
		if offset >= total {
			return &ListResponse{Status: "success", Path: virtualPath, Entries: []FileEntry{}, HasMore: false}, nil
		}
		end := offset + limit
		if end > total {
			end = total
		}
		return &ListResponse{
			Status:     "success",
			Path:       virtualPath,
			Entries:    entries[offset:end],
			HasMore:    end < total,
			NextOffset: end,
		}, nil
	}

	shareName := parts[0]
	realPath, ok := sm.shares[shareName]
	if !ok {
		return &ListResponse{Status: "error", Error: "share not found"}, nil
	}

	// 拼接真实路径
	targetPath := realPath
	if len(parts) > 1 && parts[1] != "" {
		targetPath = filepath.Join(realPath, parts[1])
	}

	// 安全检查：防止路径穿越
	if !strings.HasPrefix(targetPath, realPath) {
		return &ListResponse{Status: "error", Error: "access denied"}, nil
	}

	// 读取目录
	files, err := os.ReadDir(targetPath)
	if err != nil {
		return &ListResponse{Status: "error", Error: err.Error()}, nil
	}

	var entries []FileEntry
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		entries = append(entries, FileEntry{
			Name:    f.Name(),
			Path:    filepath.Join(virtualPath, f.Name()),
			IsDir:   f.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		})
	}

	// 排序：目录在前，文件在后
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	total := len(entries)
	if offset >= total {
		return &ListResponse{Status: "success", Path: virtualPath, Entries: []FileEntry{}, HasMore: false}, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return &ListResponse{
		Status:     "success",
		Path:       virtualPath,
		Entries:    entries[offset:end],
		HasMore:    end < total,
		NextOffset: end,
	}, nil
}

// GetRealPath 获取虚拟路径对应的真实路径
func (sm *ShareManager) GetRealPath(virtualPath string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	parts := strings.SplitN(strings.TrimPrefix(virtualPath, "/"), "/", 2)
	if len(parts) == 0 {
		return "", os.ErrInvalid
	}

	shareName := parts[0]
	realPath, ok := sm.shares[shareName]
	if !ok {
		return "", os.ErrNotExist
	}

	if len(parts) > 1 && parts[1] != "" {
		target := filepath.Join(realPath, parts[1])
		if !strings.HasPrefix(target, realPath) {
			return "", os.ErrPermission
		}
		return target, nil
	}
	return realPath, nil
}

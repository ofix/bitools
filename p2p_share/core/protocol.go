package core

// ListRequest 目录请求
type ListRequest struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// FileEntry 文件/目录条目
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

// ListResponse 目录响应
type ListResponse struct {
	Status     string      `json:"status"` // success, error
	Error      string      `json:"error,omitempty"`
	Path       string      `json:"path"`
	Entries    []FileEntry `json:"entries"`
	HasMore    bool        `json:"has_more"`
	NextOffset int         `json:"next_offset"`
}

// PeerInfo 节点信息
type PeerInfo struct {
	ID      string `json:"id"`
	Alias   string `json:"alias"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

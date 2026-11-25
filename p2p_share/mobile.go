//go:build mobile
// +build mobile

package main

import (
	"encoding/json"
	"os"
	"p2p_core/core"
)

var mobileNode *core.Node

// Init 初始化移动端节点
func Init(dataDir string) error {
	os.MkdirAll(dataDir, 0755)
	mobileNode = core.NewNode(dataDir)
	return mobileNode.Start(0) // 随机端口
}

// SetAlias 设置别名
func SetAlias(alias string) {
	if mobileNode != nil {
		mobileNode.SetAlias(alias)
	}
}

// AddShare 添加共享目录
func AddShare(name, path string) {
	if mobileNode != nil {
		mobileNode.ShareManager.AddShare(name, path)
	}
}

// GetPeers 获取节点列表 (JSON)
func GetPeers() string {
	if mobileNode == nil {
		return "[]"
	}
	peers := mobileNode.GetPeers()
	b, _ := json.Marshal(peers)
	return string(b)
}

// ListRemoteDir 获取远程目录 (JSON)
func ListRemoteDir(peerID, path string, offset, limit int) string {
	if mobileNode == nil {
		return `{"status":"error","error":"node not init"}`
	}
	result, err := mobileNode.ListRemoteDir(peerID, path, offset, limit)
	if err != nil {
		return `{"status":"error","error":"` + err.Error() + `"}`
	}
	return result
}

// DownloadFile 下载文件
func DownloadFile(peerID, remotePath, localPath string) error {
	if mobileNode == nil {
		return nil
	}
	return mobileNode.DownloadRemoteFile(peerID, remotePath, localPath)
}

func main() {
	// 移动端通常作为库被调用，main 函数仅用于测试
}

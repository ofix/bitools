//go:build !mobile
// +build !mobile

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"p2p_share/core"
	"strconv"
	"syscall"
)

//go:embed web/*
var webFS embed.FS

var node *core.Node

func main() {
	// 1. 初始化数据目录
	home, _ := os.UserHomeDir()
	dataDir := home + "/.p2pshare"
	os.MkdirAll(dataDir, 0755)

	// 2. 初始化 P2P 节点
	node = core.NewNode(dataDir)

	// 自动添加默认共享目录
	defaults := core.GetDefaultShareDirs()
	for i, path := range defaults {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			node.ShareManager.AddShare(fmt.Sprintf("Default_%d", i), path)
		}
	}

	// 3. 启动 P2P 网络
	if err := node.Start(8080); err != nil {
		log.Fatalf("Failed to start P2P node: %v", err)
	}
	log.Println("P2P Node started on port 8080")

	// 4. 启动本地 HTTP Server
	http.HandleFunc("/api/peers", handleGetPeers)
	http.HandleFunc("/api/alias", handleSetAlias)
	http.HandleFunc("/api/shares", handleGetShares)
	http.HandleFunc("/api/list", handleListRemoteDir)
	http.HandleFunc("/api/download", handleDownload)

	// 静态文件服务
	http.Handle("/", http.FileServer(http.FS(webFS)))

	log.Println("Local API Server started on http://127.0.0.1:3000")

	go func() {
		if err := http.ListenAndServe(":3000", nil); err != nil {
			log.Fatalf("HTTP Server error: %v", err)
		}
	}()

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	node.Stop()
	log.Println("Server stopped")
}

func handleGetPeers(w http.ResponseWriter, r *http.Request) {
	peers := node.GetPeers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peers)
}

func handleSetAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	node.SetAlias(req.Alias)
	w.WriteHeader(http.StatusOK)
}

func handleGetShares(w http.ResponseWriter, r *http.Request) {
	shares := node.ShareManager.GetSharesList()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shares)
}

func handleListRemoteDir(w http.ResponseWriter, r *http.Request) {
	peerID := r.URL.Query().Get("peer_id")
	path := r.URL.Query().Get("path")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if limit == 0 {
		limit = 50
	}

	result, err := node.ListRemoteDir(peerID, path, offset, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(result))
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	peerID := r.URL.Query().Get("peer_id")
	remotePath := r.URL.Query().Get("path")
	localPath := r.URL.Query().Get("local_path")

	if localPath == "" {
		// 如果未指定本地路径，则下载到默认下载目录
		home, _ := os.UserHomeDir()
		localPath = home + "/Downloads/" + remotePath
	}

	err := node.DownloadRemoteFile(peerID, remotePath, localPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Download completed"))
}

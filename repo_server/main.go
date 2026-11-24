package main

import (
	"archive/zip"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdp/qrterminal/v3"
	"github.com/quic-go/quic-go/http3"
)

//go:embed ui
var embeddedFiles embed.FS

// 默认忽略的目录
var ignoreDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"bin": true, "dist": true, "target": true, "__pycache__": true,
}

func getUIFileSystem() http.Handler {
	subFS, err := fs.Sub(embeddedFiles, "ui")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(subFS))
}

func getLocalIP() string {
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// API: 获取目录与 ZIP 文件列表
func handleDirList(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "."
	}

	rootDir, _ := os.Getwd()
	cleanPath := filepath.Clean(filepath.Join(rootDir, reqPath))
	
	// 安全校验
	if !strings.HasPrefix(cleanPath, filepath.Clean(rootDir)) {
		http.Error(w, "非法路径", http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type FileInfo struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"isDir"`
		IsZip bool   `json:"isZip"`
	}
	var files []FileInfo

	for _, entry := range entries {
		if ignoreDirs[entry.Name()] || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		isDir := entry.IsDir()
		isZip := !isDir && strings.HasSuffix(strings.ToLower(entry.Name()), ".zip")
		if isDir || isZip {
			relPath, _ := filepath.Rel(rootDir, filepath.Join(cleanPath, entry.Name()))
			files = append(files, FileInfo{
				Name:  entry.Name(),
				Path:  relPath,
				IsDir: isDir,
				IsZip: isZip,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// API: 处理打包与下载
func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Paths) == 0 {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	rootDir, _ := os.Getwd()
	repoName := filepath.Base(rootDir)

		// 如果根目录是 "/" 或者获取失败，给一个默认名
		if repoName == "" || repoName == "/" || repoName == "." {
			repoName = "repository"
		}

	// 对文件名进行 URL 编码，防止中文或特殊字符乱码
	encodedFileName := url.QueryEscape(repoName + ".zip")

	w.Header().Set("Content-Type", "application/zip")
	// 使用 filename*=UTF-8'' 语法兼容所有现代浏览器
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+encodedFileName)
	w.Header().Set("Content-Transfer-Encoding", "binary")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, relPath := range req.Paths {
		absPath := filepath.Join(rootDir, relPath)
		if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(rootDir)) {
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}

		// 1. 如果是已有的 ZIP 文件，直接流式透传
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(absPath), ".zip") {
			file, err := os.Open(absPath)
			if err != nil {
				continue
			}
			writer, err := zipWriter.Create(filepath.Base(absPath))
			if err != nil {
				file.Close()
				continue
			}
			io.Copy(writer, file)
			file.Close()
			continue
		}

		// 2. 如果是目录，执行常规的递归流式压缩
		filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if ignoreDirs[info.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			fileRelPath, _ := filepath.Rel(rootDir, path)
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			writer, err := zipWriter.Create(fileRelPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(writer, file)
			return err
		})
	}
}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", getUIFileSystem())
	mux.HandleFunc("/api/dirlist", handleDirList)
	mux.HandleFunc("/api/download", handleDownload)

	ip := getLocalIP()
	url := "https://" + ip + ":8443"

	// 生成自签名证书（如果不存在）
	if _, err := os.Stat("cert.pem"); os.IsNotExist(err) {
		log.Println("🔑 正在生成自签名证书...")
		// 实际生产中建议使用 mkcert 或 openssl 生成，此处为演示简化
		// 请确保运行目录下有 cert.pem 和 key.pem
		log.Fatal("❌ 请先生成 cert.pem 和 key.pem 再运行程序！")
	}

	qrterminal.GenerateHalfBlock(url, qrterminal.L, os.Stdout)
	log.Printf("📱 请使用手机扫码或访问: %s\n", url)

	// 启动 QUIC 服务器
	err := http3.ListenAndServeQUIC(":10000", "cert.pem", "key.pem", mux)
	if err != nil {
		log.Fatal(err)
	}
}
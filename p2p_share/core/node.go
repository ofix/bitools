package core

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/quic-go/quic-go"
)

type Node struct {
	ID           string
	Alias        string
	Port         int
	DataDir      string
	ShareManager *ShareManager
	Cache        *DirCache

	mdnsServer   *zeroconf.Server
	quicListener *quic.Listener
	peers        map[string]*PeerInfo
	peersMu      sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// generateSecureNodeID 使用 crypto/rand 生成加密安全的 16 字节（32 hex 字符）ID
func generateSecureNodeID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

func NewNode(dataDir string) *Node {
	id, err := generateSecureNodeID()
	if err != nil {
		panic(err)
	}
	hostname, _ := os.Hostname()

	return &Node{
		ID:           id,
		Alias:        hostname,
		DataDir:      dataDir,
		ShareManager: NewShareManager(),
		Cache:        NewDirCache(dataDir),
		peers:        make(map[string]*PeerInfo),
	}
}

func (n *Node) Start(port int) error {
	n.Port = port
	n.ctx, n.cancel = context.WithCancel(context.Background())

	// 1. 启动 QUIC 监听
	tlsConf := &tls.Config{
		InsecureSkipVerify: true, // 生产环境应使用自签名证书
		NextProtos:         []string{"p2p-share"},
	}
	listener, err := quic.ListenAddr(fmt.Sprintf(":%d", port), tlsConf, nil)
	if err != nil {
		return err
	}
	n.quicListener = listener

	// 2. 启动 mDNS（同步执行，失败直接把错误抛给调用方，避免静默隐身）
	if err := n.startMDNS(); err != nil {
		n.quicListener.Close()
		return fmt.Errorf("start mDNS: %w", err)
	}

	go n.acceptLoop()
	return nil
}

func (n *Node) Stop() {
	if n.cancel != nil {
		n.cancel()
	}
	if n.mdnsServer != nil {
		n.mdnsServer.Shutdown()
	}
	if n.quicListener != nil {
		n.quicListener.Close()
	}
}

// multicastInterfaces 返回可用于组播的物理网卡（值类型 []net.Interface）。
// 排除 Loopback 以及 docker / virbr / veth / utun / tun / tap / awdl 等虚拟网卡，
// 强制 mDNS 组播走真实物理网卡，解决“抓不到 5353 包”的根因。
func multicastInterfaces() ([]net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	skipPrefixes := []string{
		"docker", "br-", "virbr", "veth", "lo", "utun", "tun", "tap",
		"awdl", "llw", "anpi", "zt", "wg",
	}
	var out []net.Interface
	for _, iface := range all {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		skip := false
		for _, p := range skipPrefixes {
			if strings.HasPrefix(name, p) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, iface)
	}
	return out, nil
}

func (n *Node) startMDNS() error {

	// 注册服务（Register 的最后一个参数同样是 []net.Interface）
	txt := []string{"id=" + n.ID, "alias=" + n.Alias}
	services, err := zeroconf.Register(n.Alias, "_p2p-share._udp", "local.", n.Port, txt, nil)
	if err != nil {
		return fmt.Errorf("mDNS Register error: %w", err)
	}
	n.mdnsServer = services

	// 浏览服务：同样绑定到同一批网卡
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		n.mdnsServer.Shutdown()
		return fmt.Errorf("mDNS Resolver error: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			n.handleMDNSEntry(entry)
		}
	}()

	if err := resolver.Browse(n.ctx, "_p2p-share._udp", "local.", entries); err != nil {
		return fmt.Errorf("mDNS Browse error: %w", err)
	}
	return nil
}

func (n *Node) handleMDNSEntry(entry *zeroconf.ServiceEntry) {
	peerID := ""
	alias := entry.Instance
	for _, txt := range entry.Text {
		if strings.HasPrefix(txt, "id=") {
			peerID = strings.TrimPrefix(txt, "id=")
		}
		if strings.HasPrefix(txt, "alias=") {
			alias = strings.TrimPrefix(txt, "alias=")
		}
	}

	if peerID == "" || peerID == n.ID {
		return
	}

	addr := ""
	if len(entry.AddrIPv4) > 0 {
		addr = entry.AddrIPv4[0].String()
	} else if len(entry.AddrIPv6) > 0 {
		addr = entry.AddrIPv6[0].String()
	}
	if addr == "" {
		return
	}

	n.peersMu.Lock()
	n.peers[peerID] = &PeerInfo{
		ID:      peerID,
		Alias:   alias,
		Address: addr,
		Port:    entry.Port,
	}
	log.Println("found peer ", addr, alias, peerID)
	n.peersMu.Unlock()
}

func (n *Node) acceptLoop() {
	for {
		conn, err := n.quicListener.Accept(n.ctx)
		if err != nil {
			if n.ctx.Err() == nil {
				log.Printf("QUIC Accept error: %v", err)
			}
			return
		}
		go n.handleConnection(conn)
	}
}

func (n *Node) handleConnection(conn quic.Connection) {
	defer conn.CloseWithError(0, "")

	stream, err := conn.AcceptStream(n.ctx)
	if err != nil {
		return
	}
	defer stream.Close()

	// 读取请求头 (简单协议: LIST|Path|Offset|Limit\n  或  DOWNLOAD|Path\n)
	buf := make([]byte, 4096)
	nBytes, err := stream.Read(buf)
	if err != nil && nBytes == 0 {
		return
	}
	reqStr := strings.TrimRight(string(buf[:nBytes]), "\n")
	parts := strings.Split(reqStr, "|")
	if len(parts) < 2 {
		return
	}

	cmd := parts[0]

	// 只有 LIST 才需要 offset/limit，先判命令再取参，避免 DOWNLOAD（仅 2 段）越界 panic
	switch cmd {
	case "LIST":
		// LIST 必须有 4 段，否则直接拒绝，避免 parts[2]/parts[3] 越界 panic
		if len(parts) < 4 {
			stream.Write([]byte("ERR:bad LIST args"))
			return
		}
		path := parts[1]
		offset, _ := strconv.Atoi(parts[2])
		limit, _ := strconv.Atoi(parts[3])
		resp, err := n.ShareManager.ListDirectory(path, offset, limit)
		if err != nil {
			stream.Write([]byte("ERR:" + err.Error()))
			return
		}
		respBytes, _ := json.Marshal(resp)
		stream.Write(respBytes)
	case "DOWNLOAD":
		// DOWNLOAD 只需 2 段
		path := parts[1]
		realPath, err := n.ShareManager.GetRealPath(path)
		if err != nil {
			stream.Write([]byte("ERR:" + err.Error()))
			return
		}
		f, err := os.Open(realPath)
		if err != nil {
			stream.Write([]byte("ERR:" + err.Error()))
			return
		}
		defer f.Close()
		io.Copy(stream, f)
	}
}

// ListRemoteDir 请求远程目录
func (n *Node) ListRemoteDir(peerID, path string, offset, limit int) (string, error) {
	n.peersMu.RLock()
	peer, ok := n.peers[peerID]
	n.peersMu.RUnlock()

	if !ok {
		return "", fmt.Errorf("peer not found")
	}

	// 检查缓存
	cacheKey := peerID + ":" + path
	if cached, ok := n.Cache.Get(cacheKey); ok {
		return string(cached), nil
	}

	// 建立 QUIC 连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := fmt.Sprintf("%s:%d", peer.Address, peer.Port)
	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"p2p-share"},
	}, nil)
	if err != nil {
		return "", err
	}
	defer conn.CloseWithError(0, "")

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	req := fmt.Sprintf("LIST|%s|%d|%d\n", path, offset, limit)
	stream.Write([]byte(req))

	// 读取响应
	respBuf := make([]byte, 32*1024)
	nBytes, err := stream.Read(respBuf)
	if err != nil && nBytes == 0 {
		return "", err
	}

	result := string(respBuf[:nBytes])
	n.Cache.Set(cacheKey, respBuf[:nBytes])
	return result, nil
}

// DownloadRemoteFile 下载远程文件
func (n *Node) DownloadRemoteFile(peerID, remotePath, localPath string) error {
	n.peersMu.RLock()
	peer, ok := n.peers[peerID]
	n.peersMu.RUnlock()

	if !ok {
		return fmt.Errorf("peer not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	addr := fmt.Sprintf("%s:%d", peer.Address, peer.Port)
	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"p2p-share"},
	}, nil)
	if err != nil {
		return err
	}
	defer conn.CloseWithError(0, "")

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	req := fmt.Sprintf("DOWNLOAD|%s\n", remotePath)
	stream.Write([]byte(req))

	// 创建本地文件
	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, stream)
	return err
}

func (n *Node) GetPeers() []PeerInfo {
	n.peersMu.RLock()
	defer n.peersMu.RUnlock()

	var result []PeerInfo
	for _, p := range n.peers {
		result = append(result, *p)
	}
	return result
}

func (n *Node) SetAlias(alias string) {
	n.Alias = alias
	// 重新注册 mDNS 以更新 TXT 记录
	if n.mdnsServer != nil {
		n.mdnsServer.Shutdown()
		n.mdnsServer = nil
	}
	// 重新注册失败时记录日志，不要让调用方以为更新成功
	if err := n.startMDNS(); err != nil {
		log.Printf("re-register mDNS after SetAlias error: %v", err)
	}
}

package core

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"
)

const (
	multicastAddr = "239.255.0.1:10086"
	xorKey        = []byte("proxyserver007") // 两端必须一致
	shiftBits     = 3                      // 循环移位位数
)

type TunnelAnnounce struct {
	HTTP2Addr string `json:"h2"`
	SSH2Addr  string `json:"ssh"`
	QUICAddr  string `json:"quic"`
	Version   string `json:"ver"`
}

// rotL 循环左移 n bit（单字节）
func rotL(b byte, n int) byte {
	n = n & 7
	return (b << n) | (b >> (8 - n))
}

// rotR 循环右移 n bit（单字节）
func rotR(b byte, n int) byte {
	n = n & 7
	return (b >> n) | (b << (8 - n))
}

// encodePayload：原始 -> 左循环移位 -> 循环异或
func encodePayload(raw []byte) []byte {
	out := make([]byte, len(raw))
	for i := 0; i < len(raw); i++ {
		v := rotL(raw[i], shiftBits)
		v ^= xorKey[i%len(xorKey)]
		out[i] = v
	}
	return out
}

// decodePayload：收到的数据 -> 先异或 -> 右循环移位还原
func decodePayload(scr []byte) []byte {
	out := make([]byte, len(scr))
	for i := 0; i < len(scr); i++ {
		v := scr[i] ^ xorKey[i%len(xorKey)]
		v = rotR(v, shiftBits)
		out[i] = v
	}
	return out
}

// simplePack json -> 两层混淆 -> base64
// 原始JSON字节 → rotL(3) → 循环异或 → base64 → 发UDP组播
func simplePack(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	scrambled := encodePayload(raw)
	return base64.StdEncoding.EncodeToString(scrambled), nil
}

// simpleUnpack base64 -> 两层反混淆 -> json
// base64解码 → 循环异或 → rotR(3) → 原始JSON字节
func simpleUnpack(b64Str string, out any) error {
	scrambled, err := base64.StdEncoding.DecodeString(b64Str)
	if err != nil {
		return err
	}
	plain := decodePayload(scrambled)
	return json.Unmarshal(plain, out)
}

type Announcer struct {
	mu      sync.Mutex
	info    TunnelAnnounce
	running bool
	conn    *net.UDPConn
}

func NewAnnouncer(h2, ssh, quic string) *Announcer {
	return &Announcer{
		info: TunnelAnnounce{
			HTTP2Addr: h2,
			SSH2Addr:  ssh,
			QUICAddr:  quic,
			Version:   "v1",
		},
	}
}

func (a *Announcer) Start() error {
	addr, err := net.ResolveUDPAddr("udp", multicastAddr)
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	a.conn = conn
	a.running = true
	go a.loop()
	return nil
}

func (a *Announcer) loop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for a.running {
		<-ticker.C
		payload, err := simplePack(a.info)
		if err != nil {
			continue
		}
		_, _ = a.conn.Write([]byte(payload))
	}
}

func (a *Announcer) Stop() {
	a.running = false
	if a.conn != nil {
		_ = a.conn.Close()
	}
}

type Finder struct {
	conn *net.UDPConn
}

func NewFinder() (*Finder, error) {
	addr, err := net.ResolveUDPAddr("udp", multicastAddr)
	if err != nil {
		return nil, err
	}
	l, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: addr.Port})
	if err != nil {
		return nil, err
	}
	if err := l.SetReadBuffer(2048); err != nil {
		return nil, err
	}
	if err := l.JoinGroup(net.Interface{}, addr); err != nil {
		return nil, err
	}
	return &Finder{conn: l}, nil
}

func (f *Finder) WaitAnnounce() (*TunnelAnnounce, error) {
	buf := make([]byte, 1024)
	for {
		n, _, err := f.conn.ReadFromUDP(buf)
		if err != nil {
			return nil, err
		}
		var ann TunnelAnnounce
		err = simpleUnpack(string(buf[:n]), &ann)
		if err != nil {
			continue
		}
		return &ann, nil
	}
}

func (f *Finder) Close() {
	_ = f.conn.Close()
}

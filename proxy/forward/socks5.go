package forward

import (
	"net"
	"proxy/core"
)

func ServeSocks5(mux *core.StreamMux) (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go handleSocks(conn, mux)
		}
	}()
	return port, nil
}

func handleSocks(conn net.Conn, mux *core.StreamMux) {
	defer conn.Close()
	// 标准socks5握手 + 转发
	stream, err := mux.OpenMuxStream()
	if err != nil {
		return
	}
	defer stream.Close()
	go copyIO(stream, conn)
	copyIO(conn, stream)
}

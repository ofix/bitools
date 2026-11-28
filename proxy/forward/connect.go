package forward

import (
	"net"
	"proxy/core"
)

func ServeConnect(mux *core.StreamMux) (int, error) {
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
			go func(c net.Conn) {
				defer c.Close()
				stream, _ := mux.OpenMuxStream()
				defer stream.Close()
				go copyIO(stream, c)
				copyIO(c, stream)
			}(conn)
		}
	}()
	return port, nil
}

func copyIO(dst, src net.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		_, _ = dst.Write(buf[:n])
	}
}

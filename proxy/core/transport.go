package core

import "context"

// Transport 底层传输抽象：http2 / ssh2 / quic 都实现它
type Transport interface {
	OpenStream(ctx context.Context) (Stream, error)
	AcceptStream(ctx context.Context) (Stream, error)
	Close() error
}

// Stream 统一读写流
type Stream interface {
	Read(b []byte) (int, error)
	Write(b []byte) (int, error)
	Close() error
}

package core

import (
	"context"
	"crypto/tls"
	"github.com/quic-go/quic-go"
)

type QUICTransport struct {
	conn quic.Connection
}

func NewQUICTransport(addr string) (*QUICTransport, error) {
	tlsConf := &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"quic-tunnel"}}
	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, nil)
	if err != nil {
		return nil, err
	}
	return &QUICTransport{conn: conn}, nil
}

func (q *QUICTransport) OpenStream(ctx context.Context) (Stream, error) {
	s, err := q.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &quicStream{s: s}, nil
}
func (q *QUICTransport) AcceptStream(ctx context.Context) (Stream, error) { return nil, nil }
func (q *QUICTransport) Close() error { return q.conn.CloseWithError(0, "") }

type quicStream struct{ s quic.Stream }
func (q *quicStream) Read(b []byte) (int, error)  { return q.s.Read(b) }
func (q *quicStream) Write(b []byte) (int, error) { return q.s.Write(b) }
func (q *quicStream) Close() error                { return q.s.Close() }

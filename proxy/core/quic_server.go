package core

import (
	"context"
	"crypto/tls"
	"github.com/quic-go/quic-go"
)

func ServeQUIC(listen string, cert tls.Certificate) error {
	tlsConf := &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"quic-tunnel"}}
	l, err := quic.ListenAddr(listen, tlsConf, nil)
	if err != nil {
		return err
	}
	for {
		conn, err := l.Accept(context.Background())
		if err != nil {
			continue
		}
		go func(c quic.Connection) {
			for {
				s, err := c.AcceptStream(context.Background())
				if err != nil {
					return
				}
				stream := &quicStream{s: s}
				go func(st Stream) {
					for {
						f, err := DecodeFrame(st)
						if err != nil {
							return
						}
						_ = EncodeFrame(st, f)
					}
				}(stream)
			}
		}(conn)
	}
}

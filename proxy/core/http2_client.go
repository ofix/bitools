package core

import (
	"context"
	"crypto/tls"
	"net/http"

	"golang.org/x/net/http2"
)

type HTTP2Transport struct {
	client *http.Client
	addr   string
}

func NewHTTP2Transport(addr string) (*HTTP2Transport, error) {
	tlsConf := &tls.Config{InsecureSkipVerify: true}
	tr := &http.Transport{
		TLSClientConfig: tlsConf,
	}
	_ = http2.ConfigureTransport(tr)
	return &HTTP2Transport{
		client: &http.Client{Transport: tr},
		addr:   addr,
	}, nil
}

func (h *HTTP2Transport) OpenStream(ctx context.Context) (Stream, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", "https://"+h.addr+"/tunnel", nil)
	if err != nil {
		return nil, err
	}
	rc, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	return &http2Stream{rc: rc.Body, w: rc.Request.GetBody}, nil
}

func (h *HTTP2Transport) AcceptStream(ctx context.Context) (Stream, error) {
	return nil, nil // client侧不使用AcceptStream
}
func (h *HTTP2Transport) Close() error { return nil }

type http2Stream struct {
	rc, w io.ReadWriteCloser
}
func (s *http2Stream) Read(b []byte) (int, error)  { return s.rc.Read(b) }
func (s *http2Stream) Write(b []byte) (int, error) { return s.w.Write(b) }
func (s *http2Stream) Close() error                { return s.rc.Close() }

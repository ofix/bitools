package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type StreamMux struct {
	transport Transport
	ctx       context.Context
	cancel    context.CancelFunc
	streams   map[uint32]*muxStream
	mu        sync.Mutex
	nextID    uint32
}

type muxStream struct {
	id     uint32
	rxChan chan []byte
	closed bool
}

func NewStreamMux() *StreamMux {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamMux{
		ctx:    ctx,
		cancel: cancel,
		streams: make(map[uint32]*muxStream),
	}
}

func (m *StreamMux) BindTransport(t Transport) {
	m.transport = t
	go m.recvLoop()
}

func (m *StreamMux) OpenMuxStream() (Stream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sid := m.nextID
	m.nextID++
	ms := &muxStream{
		id:     sid,
		rxChan: make(chan []byte, 16),
	}
	m.streams[sid] = ms
	return &muxStreamWrapper{mux: m, inner: ms, id: sid}, nil
}

func (m *StreamMux) recvLoop() {
	for {
		stream, err := m.transport.AcceptStream(m.ctx)
		if err != nil {
			return
		}
		go m.handleStream(stream)
	}
}

func (m *StreamMux) handleStream(s Stream) {
	defer s.Close()
	for {
		f, err := DecodeFrame(s)
		if err != nil {
			return
		}
		if len(f.Payload) < 4 {
			continue
		}
		sid := binary.BigEndian.Uint32(f.Payload[:4])
		m.mu.Lock()
		ms, ok := m.streams[sid]
		m.mu.Unlock()
		if !ok {
			continue
		}
		select {
		case ms.rxChan <- f.Payload[4:]:
		default:
		}
	}
}

func (m *StreamMux) writeFrame(sid uint32, data []byte) error {
	stream, err := m.transport.OpenStream(m.ctx)
	if err != nil {
		return err
	}
	defer stream.Close()
	buf := make([]byte, 4, 4+len(data))
	binary.BigEndian.PutUint32(buf, sid)
	buf = append(buf, data...)
	return EncodeFrame(stream, &Frame{Payload: buf})
}

type muxStreamWrapper struct {
	mux   *StreamMux
	inner *muxStream
	id    uint32
}

func (w *muxStreamWrapper) Read(b []byte) (int, error) {
	select {
	case <-w.mux.ctx.Done():
		return 0, errors.New("mux closed")
	case data, ok := <-w.inner.rxChan:
		if !ok {
			return 0, errors.New("stream closed")
		}
		copy(b, data)
		return len(data), nil
	}
}

func (w *muxStreamWrapper) Write(b []byte) (int, error) {
	err := w.mux.writeFrame(w.id, b)
	return len(b), err
}

func (w *muxStreamWrapper) Close() error {
	w.mux.mu.Lock()
	defer w.mux.mu.Unlock()
	delete(w.mux.streams, w.id)
	close(w.inner.rxChan)
	return nil
}

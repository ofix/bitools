package core

import (
	"encoding/binary"
	"io"
)

const (
	FrameTypeSYN  = 0x01
	FrameTypeDATA = 0x02
	FrameTypeFIN  = 0x03
)

type Frame struct {
	Type uint8
	SID  uint32
	Data []byte
}

func EncodeFrame(f *Frame) []byte {
	buf := make([]byte, 1+4+4+len(f.Data))
	buf[0] = f.Type
	binary.BigEndian.PutUint32(buf[1:5], f.SID)
	binary.BigEndian.PutUint32(buf[5:9], uint32(len(f.Data)))
	copy(buf[9:], f.Data)
	return buf
}

func DecodeFrame(r io.Reader) (*Frame, error) {
	header := make([]byte, 9)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	typ := header[0]
	sid := binary.BigEndian.Uint32(header[1:5])
	dataLen := binary.BigEndian.Uint32(header[5:9])
	data := make([]byte, dataLen)
	if dataLen > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
	}
	return &Frame{
		Type: typ,
		SID:  sid,
		Data: data,
	}, nil
}

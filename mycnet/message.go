package mycnet

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	myc "myceliumweb.org/mycelium/mycmem"
)

type MessageType uint8

const (
	MT_INVALID MessageType = iota

	MT_BLOB_PULL
	MT_BLOB_PUSH

	MT_ANYVAL_TELL
	MT_ANYVAL_ASK
	MT_ANYVAL_REPLY
)

type Message struct {
	header uint32
	buf    [mycelium.MaxSizeBytes]byte
}

func (m *Message) Type() MessageType {
	return MessageType(m.header >> 24)
}

func (m *Message) Len() int {
	return int(m.header & low24)
}

func (m *Message) Body() []byte {
	return m.buf[:m.Len()]
}

// MaxBuf returns a slice pointing to the message's buffer.
func (m *Message) MaxBuf() []byte {
	return m.buf[:]
}

func (m *Message) ReadFrom(r io.Reader) (int64, error) {
	var headerBuf [4]byte
	if _, err := io.ReadFull(r, headerBuf[:]); err != nil {
		return 0, err
	}
	m.header = binary.BigEndian.Uint32(headerBuf[:])
	n, err := io.ReadFull(r, m.Body())
	return int64(n), err
}

func (m *Message) WriteTo(w io.Writer) (int64, error) {
	var headerBuf [4]byte
	binary.BigEndian.PutUint32(headerBuf[:], m.header)
	vec := net.Buffers{headerBuf[:], m.Body()}
	return vec.WriteTo(w)
}

func (m *Message) SetBlobPull(id cadata.ID) {
	m.setType(MT_BLOB_PULL)
	m.setBody(id[:])
}

func (m *Message) SetBlobPush(x []byte) {
	m.setType(MT_BLOB_PUSH)
	m.setBody(x)
}

func (m *Message) SetBlobNotFound(id cadata.ID) {
	m.setType(MT_BLOB_PUSH)
	m.setBody(id[:])
}

func (m *Message) SetAnyValTell(data []byte) {
	m.setType(MT_ANYVAL_TELL)
	m.setBody(data)
}

func (m *Message) SetAnyValAsk(data []byte) {
	m.setType(MT_ANYVAL_ASK)
	m.setBody(data)
}

func (m *Message) SetAnyValReply(data []byte) {
	m.setType(MT_ANYVAL_REPLY)
	m.setBody(data)
}

func (m *Message) AsAnyValue(ctx context.Context, src cadata.Getter) (*myc.AnyValue, error) {
	switch m.Type() {
	case MT_ANYVAL_TELL, MT_ANYVAL_ASK, MT_ANYVAL_REPLY:
		return myc.LoadRoot(ctx, src, m.Body())
	default:
		return nil, fmt.Errorf("%v message type does not contain an expression", m.Type())
	}
}

func (m *Message) AsID() (cadata.ID, error) {
	body := m.Body()
	if len(body) != cadata.IDSize {
		return cadata.ID{}, fmt.Errorf("message is wrong size to be ID, len=%d", len(body))
	}
	return cadata.IDFromBytes(body), nil
}

func (m *Message) String() string {
	data := m.Body()
	if len(data) > 10 {
		data = data[:10]
	}
	return fmt.Sprintf("Message{type=%v, n=%v, %q}", m.Type(), m.Len(), data)
}

func (m *Message) setType(x MessageType) {
	m.header &= low24
	m.header |= uint32(x) << 24
}

func (m *Message) setBody(x []byte) {
	n := copy(m.buf[:], x)
	m.SetLen(n)
}

func (m *Message) SetLen(x int) {
	m.header &= ^low24
	m.header |= low24 & uint32(x)
}

type sliceWriter struct {
	Bytes []byte
	N     int
}

func (w *sliceWriter) Write(p []byte) (int, error) {
	space := len(w.Bytes) - w.N
	if space <= 0 {
		return 0, errors.New("limit write out of space")
	}
	n := copy(w.Bytes[w.N:], p)
	w.N += n
	return n, nil
}

const low24 = uint32(0x00ff_ffff)

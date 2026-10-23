package protocol

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// MaxFrame bounds a single encoded message.
const MaxFrame = 16 << 20

const headerSize = 4

// ErrFrameTooLarge reports a message larger than MaxFrame.
var ErrFrameTooLarge = errors.New("protocol frame too large")

// Codec turns messages into bytes and back.
type Codec interface {
	Name() string
	Marshal(message Message) ([]byte, error)
	Unmarshal(data []byte) (Message, error)
}

// JSONCodec encodes messages as JSON, readable in a wire tap.
type JSONCodec struct{}

// Name identifies the codec.
func (JSONCodec) Name() string { return "json" }

// Marshal encodes a message.
func (JSONCodec) Marshal(message Message) ([]byte, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("json codec: %w", err)
	}

	return data, nil
}

// Unmarshal decodes a message.
func (JSONCodec) Unmarshal(data []byte) (Message, error) {
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, fmt.Errorf("json codec: %w", err)
	}

	return message, nil
}

// Conn sends and receives length-prefixed messages over a stream. Send is safe
// for concurrent use; Receive must be called from one goroutine.
type Conn struct {
	reader  *bufio.Reader
	writer  io.Writer
	closer  io.Closer
	codec   Codec
	writeMu sync.Mutex
}

// NewConn wraps a stream connection.
func NewConn(stream io.ReadWriteCloser, codec Codec) *Conn {
	return &Conn{
		reader:  bufio.NewReader(stream),
		writer:  stream,
		closer:  stream,
		codec:   codec,
		writeMu: sync.Mutex{},
	}
}

// Send writes one message.
func (c *Conn) Send(message Message) error {
	data, err := c.codec.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode %s: %w", message.Kind, err)
	}
	size := len(data)
	if size > MaxFrame {
		return fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, size)
	}
	frame := make([]byte, headerSize, headerSize+size)
	binary.BigEndian.PutUint32(frame, uint32(size))
	frame = append(frame, data...)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.writer.Write(frame); err != nil {
		return fmt.Errorf("send %s: %w", message.Kind, err)
	}

	return nil
}

// Receive reads one message. It returns io.EOF when the peer closed cleanly.
func (c *Conn) Receive() (Message, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(c.reader, header[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return Message{}, io.EOF
		}

		return Message{}, fmt.Errorf("receive header: %w", err)
	}
	size := binary.BigEndian.Uint32(header[:])
	if size > MaxFrame {
		return Message{}, fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(c.reader, data); err != nil {
		return Message{}, fmt.Errorf("receive body: %w", err)
	}

	message, err := c.codec.Unmarshal(data)
	if err != nil {
		return Message{}, fmt.Errorf("decode frame: %w", err)
	}

	return message, nil
}

// Close closes the underlying stream.
func (c *Conn) Close() error {
	if err := c.closer.Close(); err != nil {
		return fmt.Errorf("close connection: %w", err)
	}

	return nil
}

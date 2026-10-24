package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// ErrHandshake reports a connection refused during its handshake.
var ErrHandshake = errors.New("handshake refused")

func (o *Orchestrator) acceptLoop(ctx context.Context) {
	for {
		connection, err := o.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}
			o.cfg.Logger.Warn("accept failed", "error", err)

			continue
		}
		go o.serveConnection(ctx, connection)
	}
}

// serveConnection completes the handshake, then forwards every message to the
// core loop until the connection ends.
func (o *Orchestrator) serveConnection(ctx context.Context, connection net.Conn) {
	conn := protocol.NewConn(connection, protocol.JSONCodec{})
	joined, err := o.handshake(connection, conn)
	if err != nil {
		o.cfg.Logger.Warn("connection refused", "error", err)
		if closeErr := conn.Close(); closeErr != nil {
			o.cfg.Logger.Debug("close refused connection", "error", closeErr)
		}

		return
	}
	go writeLoop(joined, o.cfg.Logger.Debug)
	select {
	case o.joins <- joined:
	case <-ctx.Done():
		close(joined.out)

		return
	}
	for {
		message, err := conn.Receive()
		if err != nil {
			select {
			case o.leaves <- joined:
			case <-ctx.Done():
			}

			return
		}
		select {
		case o.inbound <- inbound{from: joined, message: message}:
		case <-ctx.Done():
			return
		}
	}
}

func (o *Orchestrator) handshake(connection net.Conn, conn *protocol.Conn) (*window, error) {
	if err := connection.SetReadDeadline(time.Now().Add(o.cfg.HandshakeTimeout)); err != nil {
		return nil, fmt.Errorf("handshake deadline: %w", err)
	}
	message, err := conn.Receive()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHandshake, err)
	}
	hello, err := protocol.DecodePayload[protocol.Hello](message)
	if err != nil || message.Kind != protocol.KindHello {
		return nil, fmt.Errorf("%w: expected hello", ErrHandshake)
	}
	if reason := o.refusal(hello); reason != "" {
		refusal := mustMessage(protocol.KindShutdown, message.Request, protocol.Shutdown{Reason: reason, Save: false})
		if err := conn.Send(refusal); err != nil {
			o.cfg.Logger.Debug("send refusal", "error", err)
		}

		return nil, fmt.Errorf("%w: %s", ErrHandshake, reason)
	}
	if err := connection.SetReadDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("clear handshake deadline: %w", err)
	}
	if hello.Window.IsZero() {
		hello.Window, err = o.ids.New()
		if err != nil {
			return nil, fmt.Errorf("window id: %w", err)
		}
	}
	group := hello.UndoGroup
	if group == "" {
		group = "window:" + hello.Window.String()
	}

	return &window{
		id:      hello.Window,
		hello:   hello,
		group:   group,
		conn:    conn,
		out:     make(chan protocol.Message, o.cfg.OutboundQueue),
		unsaved: protocol.Unsaved{Count: 0, Name: ""},
	}, nil
}

func (o *Orchestrator) refusal(hello protocol.Hello) string {
	if hello.Protocol != protocol.Version {
		return fmt.Sprintf("protocol version %d is not supported (expected %d)", hello.Protocol, protocol.Version)
	}
	if hello.Build != o.cfg.Build {
		return fmt.Sprintf("window build %q does not match editor build %q", hello.Build, o.cfg.Build)
	}

	return ""
}

// writeLoop sends a window's queued messages, then closes the connection when
// the queue is closed.
func writeLoop(target *window, debug func(string, ...any)) {
	for message := range target.out {
		if err := target.conn.Send(message); err != nil {
			debug("send failed", "window", target.id, "error", err)

			break
		}
	}
	for range target.out {
		continue
	}
	if err := target.conn.Close(); err != nil {
		debug("close window connection", "window", target.id, "error", err)
	}
}

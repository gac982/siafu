// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/vars"
)

const wsReadLimit = 1 << 20 // 1 MiB

// OpenWebSockets returns the number of live WebSocket connections.
func (c *Client) OpenWebSockets() int { return len(c.wsConns) }

// CloseWebSockets closes every connection owned by this virtual user. It is
// safe to call after an iteration, including after an error or cancellation.
// The shared handshake transport is left open; it belongs to the engine.
func (c *Client) CloseWebSockets() {
	for name, conn := range c.wsConns {
		_ = conn.CloseNow()
		delete(c.wsConns, name)
	}
}

func (c *Client) doWebSocket(ctx context.Context, step *PreparedWebSocket, scope vars.Scope) Result {
	switch step.Action {
	case config.WSActionConnect:
		return c.wsConnect(ctx, step, scope)
	case config.WSActionSend:
		return c.wsSend(ctx, step, scope)
	case config.WSActionReceive:
		return c.wsReceive(ctx, step, scope)
	case config.WSActionClose:
		return c.wsClose(ctx, step)
	default:
		return Result{
			Protocol: "WebSocket",
			ErrKind:  ErrWebSocketProtocol,
			Err:      fmt.Errorf("acción WebSocket desconocida %q", step.Action),
		}
	}
}

func (c *Client) wsConnect(ctx context.Context, step *PreparedWebSocket, scope vars.Scope) Result {
	start := time.Now()
	res := Result{Protocol: "WebSocket"}

	rawURL, err := step.URL.Render(scope)
	if err != nil {
		res.Duration = time.Since(start)
		res.ErrKind = ErrTemplate
		res.Err = c.sanitize(err)
		return res
	}
	resolved, err := c.resolveURL(rawURL)
	if err != nil {
		res.Duration = time.Since(start)
		res.ErrKind = ErrTemplate
		res.Err = c.sanitize(err)
		return res
	}
	headers, err := renderWSHeaders(step.Headers, scope)
	if err != nil {
		res.Duration = time.Since(start)
		res.ErrKind = ErrTemplate
		res.Err = c.sanitize(err)
		return res
	}

	ctx, cancel := c.wsContext(ctx, step)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, resolved, &websocket.DialOptions{
		HTTPClient:   c.wsHTTPClient,
		HTTPHeader:   headers,
		Subprotocols: step.Subprotocols,
	})
	res.Duration = time.Since(start)
	if resp != nil {
		res.Status = resp.StatusCode
	}
	if err != nil {
		res.ErrKind = ErrWebSocketDial
		res.Err = c.sanitize(err)
		return res
	}
	if existing := c.wsConns[step.Connection]; existing != nil {
		_ = conn.CloseNow()
		res.ErrKind = ErrWebSocketProtocol
		res.Err = fmt.Errorf("la conexión WebSocket %q ya existe", step.Connection)
		return res
	}
	conn.SetReadLimit(wsReadLimit)
	c.wsConns[step.Connection] = conn
	res.Status = 101
	res.Checks, res.CheckReason = evaluateWSConnect(step.Expect, res.Status, res.Duration)
	return res
}

func (c *Client) wsSend(ctx context.Context, step *PreparedWebSocket, scope vars.Scope) Result {
	start := time.Now()
	res := Result{Protocol: "WebSocket"}
	conn, ok := c.wsConns[step.Connection]
	if !ok {
		res.Duration = time.Since(start)
		res.ErrKind = ErrWebSocketProtocol
		res.Err = fmt.Errorf("la conexión WebSocket %q no existe", step.Connection)
		return res
	}

	typ := websocket.MessageText
	var data []byte
	if step.Binary != nil {
		typ = websocket.MessageBinary
		data = step.Binary
	} else {
		text, err := step.Text.Render(scope)
		if err != nil {
			res.Duration = time.Since(start)
			res.ErrKind = ErrTemplate
			res.Err = c.sanitize(err)
			return res
		}
		data = []byte(text)
	}

	ctx, cancel := c.wsContext(ctx, step)
	defer cancel()
	err := conn.Write(ctx, typ, data)
	res.Duration = time.Since(start)
	res.Bytes = int64(len(data))
	if err != nil {
		c.dropWSConn(step.Connection)
		res.ErrKind = classifyWSError(ctx, err, ErrWebSocketSend)
		res.Err = c.sanitize(err)
		return res
	}
	res.Checks, res.CheckReason = evaluateWSDuration(step.Expect, res.Duration)
	return res
}

func (c *Client) wsReceive(ctx context.Context, step *PreparedWebSocket, scope vars.Scope) Result {
	start := time.Now()
	res := Result{Protocol: "WebSocket"}
	conn, ok := c.wsConns[step.Connection]
	if !ok {
		res.Duration = time.Since(start)
		res.ErrKind = ErrWebSocketProtocol
		res.Err = fmt.Errorf("la conexión WebSocket %q no existe", step.Connection)
		return res
	}

	ctx, cancel := c.wsContext(ctx, step)
	defer cancel()
	typ, data, err := conn.Read(ctx)
	res.Duration = time.Since(start)
	res.Bytes = int64(len(data))
	if err != nil {
		c.dropWSConn(step.Connection)
		res.ErrKind = classifyWSError(ctx, err, ErrWebSocketReceive)
		res.Err = c.sanitize(err)
		return res
	}
	res.Checks, res.CheckReason = evaluateWSReceive(step.Expect, typ, data, res.Duration)
	return res
}

func (c *Client) wsClose(ctx context.Context, step *PreparedWebSocket) Result {
	start := time.Now()
	res := Result{Protocol: "WebSocket"}
	conn, ok := c.wsConns[step.Connection]
	if !ok {
		res.Duration = time.Since(start)
		res.ErrKind = ErrWebSocketProtocol
		res.Err = fmt.Errorf("la conexión WebSocket %q no existe", step.Connection)
		return res
	}
	ctx, cancel := c.wsContext(ctx, step)
	defer cancel()
	err := conn.Close(websocket.StatusCode(step.CloseCode), step.CloseReason)
	delete(c.wsConns, step.Connection)
	res.Duration = time.Since(start)
	if err != nil {
		_ = conn.CloseNow()
		res.ErrKind = ErrWebSocketClosed
		res.Err = c.sanitize(err)
	}
	return res
}

func (c *Client) dropWSConn(name string) {
	if conn, ok := c.wsConns[name]; ok {
		_ = conn.CloseNow()
		delete(c.wsConns, name)
	}
}

func (c *Client) wsContext(ctx context.Context, step *PreparedWebSocket) (context.Context, context.CancelFunc) {
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = c.wsTimeout
	}
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

func renderWSHeaders(headers map[string]vars.Template, scope vars.Scope) (http.Header, error) {
	rendered := make(http.Header, len(headers))
	for name, tpl := range headers {
		value, err := tpl.Render(scope)
		if err != nil {
			return nil, fmt.Errorf("header %s: %w", name, err)
		}
		rendered.Set(name, value)
	}
	return rendered, nil
}

func classifyWSError(ctx context.Context, err error, fallback ErrorKind) ErrorKind {
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
		return ErrWebSocketTimeout
	}
	if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
		return ErrCanceled
	}
	if websocket.CloseStatus(err) != -1 {
		return ErrWebSocketClosed
	}
	return fallback
}

func evaluateWSConnect(expect config.WebSocketExpect, status int, duration time.Duration) ([]Check, string) {
	checks := make([]Check, 0, 2)
	reason := ""
	statusPass := status == 101
	if len(expect.Status) > 0 {
		statusPass = false
		for _, code := range expect.Status {
			if status == code {
				statusPass = true
				break
			}
		}
	}
	checks = append(checks, Check{Name: "status", Passed: statusPass})
	if !statusPass {
		reason = "status"
	}
	if expect.MaxDuration.Value() > 0 {
		pass := duration <= expect.MaxDuration.Value()
		checks = append(checks, Check{Name: "duration", Passed: pass})
		if !pass && reason == "" {
			reason = "duration"
		}
	}
	return checks, reason
}

func evaluateWSDuration(expect config.WebSocketExpect, duration time.Duration) ([]Check, string) {
	if expect.MaxDuration.Value() <= 0 {
		return nil, ""
	}
	pass := duration <= expect.MaxDuration.Value()
	if !pass {
		return []Check{{Name: "duration", Passed: false}}, "duration"
	}
	return []Check{{Name: "duration", Passed: true}}, ""
}

func evaluateWSReceive(expect config.WebSocketExpect, typ websocket.MessageType, data []byte, duration time.Duration) ([]Check, string) {
	checks := make([]Check, 0, 2)
	reason := ""
	if expect.MessageContains != "" {
		pass := typ == websocket.MessageText && bytes.Contains(data, []byte(expect.MessageContains))
		checks = append(checks, Check{Name: "message", Passed: pass, Detail: typ.String()})
		if !pass {
			reason = "message"
		}
	}
	if expect.MaxDuration.Value() > 0 {
		pass := duration <= expect.MaxDuration.Value()
		checks = append(checks, Check{Name: "duration", Passed: pass})
		if !pass && reason == "" {
			reason = "duration"
		}
	}
	return checks, reason
}

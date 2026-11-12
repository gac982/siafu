// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package httpx prepares and executes HTTP requests, checks responses and
// extracts variables without leaking credentials.
package httpx

// ErrorKind classifies a failed request.
type ErrorKind string

const (
	// ErrDNS means the host could not be resolved.
	ErrDNS ErrorKind = "dns"
	// ErrDial means the TCP connection could not be established.
	ErrDial ErrorKind = "dial"
	// ErrTLS means the TLS handshake or certificate validation failed.
	ErrTLS ErrorKind = "tls"
	// ErrTimeout means the request exceeded its deadline.
	ErrTimeout ErrorKind = "timeout"
	// ErrRead means reading the response body failed.
	ErrRead ErrorKind = "read"
	// ErrRedirect means the redirect limit was exceeded.
	ErrRedirect ErrorKind = "redirect"
	// ErrBodyTooLarge means the response exceeded the read limit.
	ErrBodyTooLarge ErrorKind = "body_too_large"
	// ErrCanceled means the user stopped the test mid-request.
	ErrCanceled ErrorKind = "canceled"
	// ErrTemplate means a ${...} reference could not be resolved.
	ErrTemplate ErrorKind = "template"
	// ErrExtract means a variable could not be extracted from the response.
	ErrExtract ErrorKind = "extract"
	// ErrAnalysisBudget means the shared body-analysis budget was exhausted
	// and the step policy is fail.
	ErrAnalysisBudget ErrorKind = "analysis_budget"
	// ErrProtocol means the negotiated HTTP version does not satisfy the
	// requested target.http_version.
	ErrProtocol ErrorKind = "protocol"
	// ErrWebSocketDial means the WebSocket handshake could not be completed.
	ErrWebSocketDial ErrorKind = "ws_dial"
	// ErrWebSocketSend means writing a WebSocket message failed.
	ErrWebSocketSend ErrorKind = "ws_send"
	// ErrWebSocketReceive means reading a WebSocket message failed.
	ErrWebSocketReceive ErrorKind = "ws_receive"
	// ErrWebSocketTimeout means a WebSocket action exceeded its deadline.
	ErrWebSocketTimeout ErrorKind = "ws_timeout"
	// ErrWebSocketClosed means the peer closed the WebSocket unexpectedly.
	ErrWebSocketClosed ErrorKind = "ws_closed"
	// ErrWebSocketProtocol means the WebSocket action or connection is invalid.
	ErrWebSocketProtocol ErrorKind = "ws_protocol"
)

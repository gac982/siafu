// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/vars"
)

const (
	maxAnalysisBody = 1 << 20  // 1 MiB
	maxDiscardBody  = 10 << 20 // 10 MiB
)

// Result is the outcome of one step execution.
type Result struct {
	Status          int
	Protocol        string
	Duration        time.Duration
	Bytes           int64
	Checks          []Check
	CheckReason     string
	Extracted       map[string]string
	AnalysisSkipped bool
	ErrKind         ErrorKind
	Err             error
}

// Failed reports whether any check or transport stage failed.
func (r Result) Failed() bool {
	return r.ErrKind != "" || r.CheckReason != ""
}

// ChecksTotal returns how many checks were evaluated.
func (r Result) ChecksTotal() int { return len(r.Checks) }

// ChecksPassed returns how many checks passed.
func (r Result) ChecksPassed() int {
	passed := 0
	for _, check := range r.Checks {
		if check.Passed {
			passed++
		}
	}
	return passed
}

// Transport bundles the shared HTTP transport used for requests and the
// HTTP/1.1 transport used for WebSocket handshakes. HTTP is a *http.Transport
// for HTTP/1.1 and HTTP/2, and an *http3.Transport for HTTP/3.
type Transport struct {
	HTTP http.RoundTripper
	WS   *http.Transport
}

// CloseIdleConnections releases idle connections held by both transports.
func (t *Transport) CloseIdleConnections() {
	if closer, ok := t.HTTP.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
	if t.WS != nil {
		t.WS.CloseIdleConnections()
	}
}

// Close releases every resource held by the transports. For HTTP/3 it also
// closes the QUIC transport (UDP socket and read loop); HTTP/1.1 and HTTP/2
// only drop idle connections. It is safe to call more than once.
func (t *Transport) Close() error {
	var err error
	switch closer := t.HTTP.(type) {
	case io.Closer:
		err = closer.Close()
	case interface{ CloseIdleConnections() }:
		closer.CloseIdleConnections()
	}
	if t.WS != nil {
		t.WS.CloseIdleConnections()
	}
	return err
}

// Client is one virtual user's HTTP session.
type Client struct {
	httpClient     *http.Client
	base           *url.URL
	redactor       *redact.Redactor
	analysis       *AnalysisBudget
	analysisPolicy string
	requiredProto  int
	wsConns        map[string]*websocket.Conn
	wsTimeout      time.Duration
	wsHTTPClient   *http.Client
}

// NewTransport builds the shared transport.
func NewTransport(target config.Target, maxVUs int) (*Transport, error) {
	return NewTransportWithIdle(target, maxVUs, maxVUs)
}

// NewTransportWithIdle builds the shared transports with an explicit idle cap.
func NewTransportWithIdle(target config.Target, maxVUs, maxIdle int) (*Transport, error) {
	if maxVUs < 1 {
		maxVUs = 1
	}
	if maxIdle <= 0 {
		maxIdle = maxVUs
	}
	if maxIdle > maxVUs {
		maxIdle = maxVUs
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: target.TLS.InsecureSkipVerify} //nolint:gosec // explicit test option
	if target.TLS.CAFile != "" {
		data, err := os.ReadFile(target.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("leer ca_file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("ca_file %s no contiene certificados PEM válidos", target.TLS.CAFile)
		}
		tlsConfig.RootCAs = pool
	}

	version := target.HTTPVersion
	if version == "" {
		version = config.HTTPVersionAuto
	}
	if version == config.HTTPVersion3 {
		return newHTTP3Transport(tlsConfig, maxIdle), nil
	}

	transport := newHTTPTransport(tlsConfig, version, maxIdle)
	// WebSocket always upgrades over HTTP/1.1, even when requests use HTTP/2,
	// so it gets its own strict HTTP/1.1 transport.
	wsTransport := transport.Clone()
	wsTransport.ForceAttemptHTTP2 = false
	wsTransport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}

	return &Transport{HTTP: transport, WS: wsTransport}, nil
}

func newHTTPTransport(tlsConfig *tls.Config, version string, maxIdle int) *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     version != config.HTTPVersion11,
		MaxIdleConns:          maxIdle * 2,
		MaxIdleConnsPerHost:   maxIdle,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       tlsConfig,
	}
	if version == config.HTTPVersion11 {
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}
	return transport
}

func newHTTP3Transport(tlsConfig *tls.Config, maxIdle int) *Transport {
	// QUIC has no HTTP proxy support, so ProxyFromEnvironment is deliberately
	// not wired here: http_version 3 talks directly to the target.
	http3Transport := &http3.Transport{
		TLSClientConfig: tlsConfig.Clone(),
		QUICConfig: &quic.Config{
			HandshakeIdleTimeout: 10 * time.Second,
			MaxIdleTimeout:       90 * time.Second,
			KeepAlivePeriod:      15 * time.Second,
		},
	}
	// WebSocket keeps using HTTP/1.1 under http_version 3.
	wsTransport := newHTTPTransport(tlsConfig.Clone(), config.HTTPVersion11, maxIdle)
	return &Transport{HTTP: http3Transport, WS: wsTransport}
}

// requiredProtocol maps target.http_version to the minimum protocol the
// response must use, or 0 when any protocol is acceptable.
func requiredProtocol(version string) int {
	switch version {
	case config.HTTPVersion2:
		return 2
	case config.HTTPVersion3:
		return 3
	default:
		return 0
	}
}

// SetAnalysisBudget sets the shared byte budget for response analysis.
func (c *Client) SetAnalysisBudget(budget *AnalysisBudget) { c.analysis = budget }

// SetAnalysisPolicy configures what happens when the analysis budget is
// exhausted: "skip" skips body analysis, "fail" fails the step. An empty
// policy blocks until budget is available (closed-mode default).
func (c *Client) SetAnalysisPolicy(policy string) { c.analysisPolicy = policy }

// NewClient creates an isolated session (cookies included) sharing the
// transport with the rest of the virtual users.
func NewClient(target config.Target, transport *Transport, redactor *redact.Redactor) (*Client, error) {
	base, err := url.Parse(target.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("base_url inválida: %w", err)
	}
	httpClient := &http.Client{Transport: transport.HTTP, Jar: newLazyJar(), Timeout: target.Timeout.Value()}
	wsHTTPClient := &http.Client{Transport: transport.WS, Jar: httpClient.Jar, Timeout: target.Timeout.Value()}
	return &Client{
		httpClient:    httpClient,
		wsHTTPClient:  wsHTTPClient,
		base:          base,
		redactor:      redactor,
		requiredProto: requiredProtocol(target.HTTPVersion),
		wsConns:       make(map[string]*websocket.Conn),
		wsTimeout:     target.Timeout.Value(),
	}, nil
}

// Do executes one prepared step.
func (c *Client) Do(ctx context.Context, step PreparedStep, scope vars.Scope) Result {
	if step.WS != nil {
		return c.doWebSocket(ctx, step.WS, scope)
	}
	start := time.Now()

	rawURL, err := step.URL.Render(scope)
	if err != nil {
		return Result{Duration: time.Since(start), ErrKind: ErrTemplate, Err: c.sanitize(err)}
	}
	resolved, err := c.resolveURL(rawURL)
	if err != nil {
		return Result{Duration: time.Since(start), ErrKind: ErrTemplate, Err: c.sanitize(err)}
	}

	body, err := step.Body.Render(scope)
	if err != nil {
		return Result{Duration: time.Since(start), ErrKind: ErrTemplate, Err: c.sanitize(err)}
	}

	headers := make(http.Header, len(step.Headers))
	for name, tpl := range step.Headers {
		value, err := tpl.Render(scope)
		if err != nil {
			return Result{Duration: time.Since(start), ErrKind: ErrTemplate, Err: c.sanitize(err)}
		}
		headers.Set(name, value)
	}

	req, err := http.NewRequestWithContext(ctx, step.Method, resolved, strings.NewReader(body))
	if err != nil {
		return Result{Duration: time.Since(start), ErrKind: ErrTemplate, Err: c.sanitize(err)}
	}
	req.Header = headers

	analyze := step.Expect.BodyContains != "" || len(step.Extract) > 0

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{
			Duration: time.Since(start),
			ErrKind:  classifyError(ctx, err),
			Err:      c.sanitize(err),
		}
	}
	defer resp.Body.Close()

	if c.requiredProto != 0 && resp.ProtoMajor != c.requiredProto {
		return Result{
			Duration: time.Since(start),
			Status:   resp.StatusCode,
			Protocol: resp.Proto,
			ErrKind:  ErrProtocol,
			Err:      fmt.Errorf("HTTP/%d requerido; el servidor negoció %s", c.requiredProto, resp.Proto),
		}
	}

	var data []byte
	var bytesRead int64
	var readKind ErrorKind
	var readErr error
	var skipped bool
	release := func() {}
	if analyze {
		data, bytesRead, readKind, readErr, release, skipped = readAnalyzedBody(ctx, resp.Body, c.analysis, c.analysisPolicy)
	} else {
		data, bytesRead, readKind, readErr = readResponseBody(resp.Body, false)
	}
	defer release()
	duration := time.Since(start)

	result := Result{
		Status:          resp.StatusCode,
		Protocol:        resp.Proto,
		Duration:        duration,
		Bytes:           bytesRead,
		AnalysisSkipped: skipped,
	}
	if readKind == ErrBodyTooLarge {
		result.ErrKind = ErrBodyTooLarge
		result.Err = fmt.Errorf("el cuerpo supera el límite de lectura")
		return result
	}
	if readErr != nil {
		result.ErrKind = readKind
		if result.ErrKind == "" {
			result.ErrKind = classifyError(ctx, readErr)
		}
		result.Err = c.sanitize(readErr)
		return result
	}

	expect := step.Expect
	if skipped {
		expect.BodyContains = ""
	}
	checks, reason := evaluateChecks(expect, resp.StatusCode, duration, data)
	result.Checks = checks
	result.CheckReason = reason
	if reason != "" {
		return result
	}

	if len(step.Extract) > 0 && !skipped {
		extracted := make(map[string]string, len(step.Extract))
		for _, key := range sortedExtractKeys(step.Extract) {
			spec := step.Extract[key]
			var value string
			var err error
			switch spec.Source {
			case ExtractJSON:
				value, err = extractJSON(data, spec.Path)
			case ExtractHeader:
				value, err = extractHeader(resp.Header, spec.Path)
			default:
				err = fmt.Errorf("origen de extracción desconocido")
			}
			if err != nil {
				result.ErrKind = ErrExtract
				result.Err = c.sanitize(fmt.Errorf("extraer %s: %w", key, err))
				return result
			}
			extracted[key] = value
		}
		result.Extracted = extracted
	}

	return result
}

func (c *Client) sanitize(err error) error {
	if err == nil || c.redactor == nil {
		return err
	}
	return errors.New(c.redactor.Scrub(err.Error()))
}

func (c *Client) resolveURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("URL inválida %q: %w", raw, err)
	}
	if parsed.IsAbs() {
		return parsed.String(), nil
	}
	if strings.HasPrefix(raw, "/") {
		return c.base.Scheme + "://" + c.base.Host + raw, nil
	}
	return strings.TrimSuffix(c.base.String(), "/") + "/" + raw, nil
}

func readResponseBody(body io.Reader, analyze bool) ([]byte, int64, ErrorKind, error) {
	if analyze {
		data, err := io.ReadAll(io.LimitReader(body, maxAnalysisBody+1))
		if err != nil {
			return nil, int64(len(data)), "", err
		}
		if len(data) > maxAnalysisBody {
			return nil, maxAnalysisBody, ErrBodyTooLarge, nil
		}
		return data, int64(len(data)), "", nil
	}

	n, err := io.Copy(io.Discard, io.LimitReader(body, maxDiscardBody+1))
	if err != nil {
		return nil, n, "", err
	}
	if n > maxDiscardBody {
		return nil, maxDiscardBody, ErrBodyTooLarge, nil
	}
	return nil, n, "", nil
}

func classifyError(ctx context.Context, err error) ErrorKind {
	if err == nil {
		return ""
	}
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return ErrCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrTimeout
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrDNS
	}

	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return ErrTLS
	}
	var recordErr tls.RecordHeaderError
	if errors.As(err, &recordErr) {
		return ErrTLS
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return ErrTLS
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return ErrTLS
	}
	var certInvalid x509.CertificateInvalidError
	if errors.As(err, &certInvalid) {
		return ErrTLS
	}

	if strings.Contains(err.Error(), "stopped after 10 redirects") {
		return ErrRedirect
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return ErrDial
	}

	if strings.Contains(err.Error(), "tls:") || strings.Contains(err.Error(), "x509:") {
		return ErrTLS
	}
	return ErrRead
}

// readAnalyzedBody reads at most 1 MiB while reserving the bytes it retains
// in the shared analysis budget. It returns a release function for the
// successfully retained bytes.
func readAnalyzedBody(ctx context.Context, body io.Reader, budget *AnalysisBudget, policy string) ([]byte, int64, ErrorKind, error, func(), bool) {
	data := make([]byte, 0, 4096)
	chunk := make([]byte, 32*1024)
	var acquired int64
	noop := func() {}

	release := func() {
		if acquired > 0 {
			budget.Release(acquired)
			acquired = 0
		}
	}

	for {
		n, err := body.Read(chunk)
		if n > 0 {
			if len(data)+n > maxAnalysisBody {
				release()
				return nil, maxAnalysisBody, ErrBodyTooLarge, nil, noop, false
			}
			if budget != nil {
				if policy == "" {
					if err := budget.Acquire(ctx, int64(n)); err != nil {
						release()
						return nil, acquired, classifyError(ctx, err), err, noop, false
					}
				} else if !budget.TryAcquire(int64(n)) {
					release()
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, acquired, "", err, noop, false
					}
					_, drained, drainKind, drainErr := readResponseBody(body, false)
					drained += int64(n)
					if drainErr != nil {
						return nil, acquired + drained, drainKind, drainErr, noop, false
					}
					if policy == config.AnalysisFail {
						return nil, acquired + drained, ErrAnalysisBudget, fmt.Errorf("presupuesto de análisis agotado"), noop, false
					}
					return nil, acquired + drained, "", nil, noop, true
				}
			}
			acquired += int64(n)
			data = append(data, chunk[:n]...)
		}
		if errors.Is(err, io.EOF) {
			return data, acquired, "", nil, release, false
		}
		if err != nil {
			release()
			return nil, acquired, "", err, noop, false
		}
	}
}

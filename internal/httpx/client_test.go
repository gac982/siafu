// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"context"
	"encoding/pem"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/redact"
	"github.com/gac982/siafu/internal/vars"
)

func targetFor(base string) config.Target {
	return config.Target{
		BaseURL: base,
		Timeout: config.Duration{Duration: 5 * time.Second},
	}
}

func prepareOne(t *testing.T, target config.Target, step config.Step) PreparedStep {
	t.Helper()
	cfg := &config.Config{
		Target:    target,
		Scenarios: []config.Scenario{{Name: "s", Steps: []config.Step{step}}},
	}
	plan, err := Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	return plan.Scenarios[0].Steps[0]
}

func clientFor(t *testing.T, target config.Target) *Client {
	t.Helper()
	tr, err := NewTransport(target, 20)
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	t.Cleanup(tr.CloseIdleConnections)
	client, err := NewClient(target, tr, redact.New(nil))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestDoGETAndPOST(t *testing.T) {
	var gotMethod, gotBody, gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Test")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	client := clientFor(t, target)

	get := prepareOne(t, target, config.Step{
		Name: "get",
		Request: config.Request{
			Method:  "GET",
			URL:     "/x",
			Headers: map[string]string{"X-Test": "yes"},
		},
		Expect: config.Expect{Status: config.StatusCodes{201}},
	})
	res := client.Do(context.Background(), get, vars.Scope{})
	if res.Failed() || res.Status != 201 || gotMethod != "GET" || gotHeader != "yes" {
		t.Fatalf("GET result = %+v, method=%q header=%q", res, gotMethod, gotHeader)
	}

	post := prepareOne(t, target, config.Step{
		Name: "post",
		Request: config.Request{
			Method: "POST",
			URL:    "/x",
			Body:   `{"a":1}`,
		},
		Expect: config.Expect{Status: config.StatusCodes{201}},
	})
	res = client.Do(context.Background(), post, vars.Scope{})
	if res.Failed() || gotMethod != "POST" || gotBody != `{"a":1}` {
		t.Fatalf("POST result = %+v, method=%q body=%q", res, gotMethod, gotBody)
	}
}

func TestResolveRelativeURL(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	target := targetFor(srv.URL + "/api/")
	client := clientFor(t, target)
	for _, raw := range []string{"/x", "y"} {
		step := prepareOne(t, target, config.Step{Name: raw, Request: config.Request{Method: "GET", URL: raw}, Expect: config.Expect{Status: config.StatusCodes{200}}})
		res := client.Do(context.Background(), step, vars.Scope{})
		if res.Failed() {
			t.Fatalf("%s failed: %+v", raw, res)
		}
	}
	if len(paths) != 2 || paths[0] != "/x" || paths[1] != "/api/y" {
		t.Errorf("paths = %v, want [/x /api/y]", paths)
	}
}

func TestDefaultStatusRange(t *testing.T) {
	status := 299
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	client := clientFor(t, target)
	step := prepareOne(t, target, config.Step{Name: "default", Request: config.Request{Method: "GET", URL: "/"}})
	if res := client.Do(context.Background(), step, vars.Scope{}); res.Failed() {
		t.Errorf("299 should pass by default: %+v", res)
	}
	status = 400
	if res := client.Do(context.Background(), step, vars.Scope{}); !res.Failed() || res.CheckReason != "status" {
		t.Errorf("400 should fail by default: %+v", res)
	}
}

func TestChecksAllEvaluatedAndReasonPriority(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("bad"))
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	step := prepareOne(t, target, config.Step{
		Name:    "checks",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect: config.Expect{
			Status:       config.StatusCodes{200},
			BodyContains: "good",
			MaxDuration:  config.Duration{Duration: time.Nanosecond},
		},
	})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if len(res.Checks) != 3 {
		t.Fatalf("checks = %+v, want 3", res.Checks)
	}
	if res.ChecksPassed() != 0 || res.CheckReason != "status" || !res.Failed() {
		t.Errorf("result = %+v, want status failure", res)
	}
	if res.ErrKind != "" {
		t.Errorf("ErrKind = %q, want empty for check failure", res.ErrKind)
	}
}

func TestExtractJSONNestedAndArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Name", "abc")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":42}],"ok":true}}`))
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	step := prepareOne(t, target, config.Step{
		Name:    "extract",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect:  config.Expect{Status: config.StatusCodes{200}},
		Extract: map[string]string{
			"id":   "json:data.items.0.id",
			"ok":   "json:data.ok",
			"name": "header:X-Name",
		},
	})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if res.Failed() {
		t.Fatalf("result = %+v", res)
	}
	if res.Extracted["id"] != "42" || res.Extracted["ok"] != "true" || res.Extracted["name"] != "abc" {
		t.Errorf("extracted = %+v", res.Extracted)
	}
}

func TestExtractOnlyWhenChecksPass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"token":"secret"}`))
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	step := prepareOne(t, target, config.Step{
		Name:    "extract",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect:  config.Expect{Status: config.StatusCodes{200}},
		Extract: map[string]string{"token": "json:token"},
	})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if res.ErrKind != "" || res.CheckReason != "status" || len(res.Extracted) != 0 {
		t.Errorf("result = %+v, want status failure without extraction", res)
	}
}

func TestCookiesPersist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "1", Path: "/"})
			w.WriteHeader(200)
		case "/me":
			c, err := r.Cookie("session")
			if err != nil || c.Value != "1" {
				w.WriteHeader(401)
				return
			}
			w.WriteHeader(200)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	client := clientFor(t, target)
	steps := []config.Step{
		{Name: "login", Request: config.Request{Method: "GET", URL: "/login"}, Expect: config.Expect{Status: config.StatusCodes{200}}},
		{Name: "me", Request: config.Request{Method: "GET", URL: "/me"}, Expect: config.Expect{Status: config.StatusCodes{200}}},
	}
	cfg := &config.Config{Target: target, Scenarios: []config.Scenario{{Name: "s", Steps: steps}}}
	plan, err := Prepare(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range plan.Scenarios[0].Steps {
		res := client.Do(context.Background(), step, vars.Scope{})
		if res.Failed() {
			t.Fatalf("step %s failed: %+v", step.Name, res)
		}
	}
}

func TestTimeoutClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	target.Timeout = config.Duration{Duration: 20 * time.Millisecond}
	step := prepareOne(t, target, config.Step{Name: "slow", Request: config.Request{Method: "GET", URL: "/"}})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if res.ErrKind != ErrTimeout {
		t.Fatalf("ErrKind = %q, want %q (err: %v)", res.ErrKind, ErrTimeout, res.Err)
	}
}

func TestErrorRedactsSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	target.Timeout = config.Duration{Duration: 20 * time.Millisecond}
	tr, err := NewTransport(target, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tr.CloseIdleConnections)
	client, err := NewClient(target, tr, redact.New([]string{"supersecret"}))
	if err != nil {
		t.Fatal(err)
	}
	step := prepareOne(t, target, config.Step{Name: "secret", Request: config.Request{Method: "GET", URL: "/slow?token=supersecret"}})
	res := client.Do(context.Background(), step, vars.Scope{})
	if res.Err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(res.Err.Error(), "supersecret") {
		t.Errorf("error leaks secret: %q", res.Err)
	}
}

func TestErrorRedactsQueryToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	target.Timeout = config.Duration{Duration: 20 * time.Millisecond}
	client := clientFor(t, target)
	step := prepareOne(t, target, config.Step{Name: "query", Request: config.Request{Method: "GET", URL: "/slow?token=hardcodedsecret&ok=1"}})
	res := client.Do(context.Background(), step, vars.Scope{})
	if res.Err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(res.Err.Error(), "hardcodedsecret") {
		t.Errorf("error leaks query token: %q", res.Err)
	}
}

func TestConnectionRefusedClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	base := srv.URL
	srv.Close()

	target := targetFor(base)
	step := prepareOne(t, target, config.Step{Name: "refused", Request: config.Request{Method: "GET", URL: "/"}})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if res.ErrKind != ErrDial {
		t.Fatalf("ErrKind = %q, want %q (err: %v)", res.ErrKind, ErrDial, res.Err)
	}
}

func TestSelfSignedTLSAndCAFile(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	defer srv.Close()

	step := func(target config.Target) PreparedStep {
		return prepareOne(t, target, config.Step{Name: "tls", Request: config.Request{Method: "GET", URL: "/"}})
	}

	plain := targetFor(srv.URL)
	res := clientFor(t, plain).Do(context.Background(), step(plain), vars.Scope{})
	if res.ErrKind != ErrTLS {
		t.Fatalf("untrusted ErrKind = %q, want %q (err: %v)", res.ErrKind, ErrTLS, res.Err)
	}

	insecure := targetFor(srv.URL)
	insecure.TLS.InsecureSkipVerify = true
	res = clientFor(t, insecure).Do(context.Background(), step(insecure), vars.Scope{})
	if res.Failed() {
		t.Fatalf("insecure result = %+v", res)
	}

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caPath, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	trusted := targetFor(srv.URL)
	trusted.TLS.CAFile = caPath
	res = clientFor(t, trusted).Do(context.Background(), step(trusted), vars.Scope{})
	if res.Failed() {
		t.Fatalf("trusted CA result = %+v", res)
	}
}

func TestRedirectFollowedAndLoopClassified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			w.WriteHeader(200)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		}
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	client := clientFor(t, target)
	ok := prepareOne(t, target, config.Step{Name: "a", Request: config.Request{Method: "GET", URL: "/a"}, Expect: config.Expect{Status: config.StatusCodes{200}}})
	if res := client.Do(context.Background(), ok, vars.Scope{}); res.Failed() || res.Status != 200 {
		t.Fatalf("redirect result = %+v", res)
	}
	loop := prepareOne(t, target, config.Step{Name: "loop", Request: config.Request{Method: "GET", URL: "/loop"}})
	if res := client.Do(context.Background(), loop, vars.Scope{}); res.ErrKind != ErrRedirect {
		t.Fatalf("loop ErrKind = %q, want %q (err: %v)", res.ErrKind, ErrRedirect, res.Err)
	}
}

func TestBodyTooLargeAnalysisAndDiscard(t *testing.T) {
	big := strings.Repeat("x", 2<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, big)
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	client := clientFor(t, target)

	analyzed := prepareOne(t, target, config.Step{
		Name:    "analyze",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect:  config.Expect{Status: config.StatusCodes{200}, BodyContains: "x"},
	})
	if res := client.Do(context.Background(), analyzed, vars.Scope{}); res.ErrKind != ErrBodyTooLarge {
		t.Fatalf("analyzed ErrKind = %q, want %q", res.ErrKind, ErrBodyTooLarge)
	}

	huge := strings.Repeat("y", (10<<20)+1024)
	hugeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, huge)
	}))
	defer hugeSrv.Close()
	hugeTarget := targetFor(hugeSrv.URL)
	hugeStep := prepareOne(t, hugeTarget, config.Step{Name: "huge", Request: config.Request{Method: "GET", URL: "/"}, Expect: config.Expect{Status: config.StatusCodes{200}}})
	if res := clientFor(t, hugeTarget).Do(context.Background(), hugeStep, vars.Scope{}); res.ErrKind != ErrBodyTooLarge {
		t.Fatalf("discard ErrKind = %q, want %q", res.ErrKind, ErrBodyTooLarge)
	}
}

func TestTruncatedBodyIsReadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("abc"))
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	step := prepareOne(t, target, config.Step{Name: "truncated", Request: config.Request{Method: "GET", URL: "/"}, Expect: config.Expect{Status: config.StatusCodes{200}}})
	res := clientFor(t, target).Do(context.Background(), step, vars.Scope{})
	if res.ErrKind != ErrRead {
		t.Fatalf("ErrKind = %q, want %q (err: %v)", res.ErrKind, ErrRead, res.Err)
	}
}

func TestMissingEnvIsTemplateError(t *testing.T) {
	target := targetFor("http://127.0.0.1:1")
	step := prepareOne(t, target, config.Step{
		Name:    "env",
		Request: config.Request{Method: "GET", URL: "/${ENV:MISSING}"},
	})
	scope := vars.Scope{LookupEnv: func(string) (string, bool) { return "", false }}
	res := clientFor(t, target).Do(context.Background(), step, scope)
	if res.ErrKind != ErrTemplate {
		t.Fatalf("ErrKind = %q, want %q (err: %v)", res.ErrKind, ErrTemplate, res.Err)
	}
}

func TestPrepareMergesTargetAndStepHeaders(t *testing.T) {
	cfg := &config.Config{
		Target: config.Target{
			BaseURL: "http://x",
			Headers: map[string]string{"Accept": "application/json", "X-Target": "t"},
		},
		Scenarios: []config.Scenario{{
			Name: "s",
			Steps: []config.Step{{
				Name:    "step",
				Request: config.Request{Method: "GET", URL: "/", Headers: map[string]string{"accept": "text/html", "X-Step": "s"}},
			}},
		}},
	}
	plan, err := Prepare(cfg)
	if err != nil {
		t.Fatal(err)
	}
	headers := plan.Scenarios[0].Steps[0].Headers
	for key, want := range map[string]string{"Accept": "text/html", "X-Target": "t", "X-Step": "s"} {
		got, err := headers[key].Render(vars.Scope{})
		if err != nil || got != want {
			t.Errorf("header %s = %q (err %v), want %q", key, got, err, want)
		}
	}
}

func TestPrepareExtractSpecs(t *testing.T) {
	cfg := &config.Config{
		Target: config.Target{BaseURL: "http://x"},
		Scenarios: []config.Scenario{{Name: "s", Steps: []config.Step{{
			Name:    "step",
			Request: config.Request{Method: "GET", URL: "/"},
			Extract: map[string]string{"id": "json:data.id", "sid": "header:X-Session"},
		}}}},
	}
	plan, err := Prepare(cfg)
	if err != nil {
		t.Fatal(err)
	}
	specs := plan.Scenarios[0].Steps[0].Extract
	if specs["id"].Source != ExtractJSON || specs["id"].Path != "data.id" {
		t.Errorf("json spec = %+v", specs["id"])
	}
	if specs["sid"].Source != ExtractHeader || specs["sid"].Path != "X-Session" {
		t.Errorf("header spec = %+v", specs["sid"])
	}
}

func TestNewTransportCapsIdleConnections(t *testing.T) {
	tr, err := NewTransportWithIdle(config.Target{BaseURL: "http://localhost"}, 10000, 256)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.CloseIdleConnections()
	httpTransport, ok := tr.HTTP.(*http.Transport)
	if !ok {
		t.Fatalf("HTTP transport = %T, want *http.Transport", tr.HTTP)
	}
	if httpTransport.MaxIdleConnsPerHost != 256 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 256", httpTransport.MaxIdleConnsPerHost)
	}
	if httpTransport.MaxIdleConns > 512 {
		t.Errorf("MaxIdleConns = %d, want <= 512", httpTransport.MaxIdleConns)
	}
}

func TestNewTransportDefaultIdleFollowsMaxVUs(t *testing.T) {
	tr, err := NewTransportWithIdle(config.Target{BaseURL: "http://localhost"}, 500, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.CloseIdleConnections()
	httpTransport, ok := tr.HTTP.(*http.Transport)
	if !ok {
		t.Fatalf("HTTP transport = %T, want *http.Transport", tr.HTTP)
	}
	if httpTransport.MaxIdleConnsPerHost != 500 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 500", httpTransport.MaxIdleConnsPerHost)
	}
}

func TestTransportSeparatesWebSocketHTTP1(t *testing.T) {
	tr, err := NewTransportWithIdle(config.Target{BaseURL: "https://localhost"}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.CloseIdleConnections()

	httpTransport, ok := tr.HTTP.(*http.Transport)
	if !ok {
		t.Fatalf("HTTP transport = %T, want *http.Transport", tr.HTTP)
	}
	if !httpTransport.ForceAttemptHTTP2 {
		t.Error("HTTP transport should attempt HTTP/2 for auto")
	}
	if tr.WS == httpTransport {
		t.Error("WS transport must not be the shared HTTP transport")
	}
	if tr.WS.ForceAttemptHTTP2 {
		t.Error("WS transport must not attempt HTTP/2")
	}
	if len(tr.WS.TLSNextProto) != 0 {
		t.Errorf("WS TLSNextProto = %d, want 0", len(tr.WS.TLSNextProto))
	}
}

func TestRequiredProtocol(t *testing.T) {
	tests := map[string]int{
		"":     0,
		"auto": 0,
		"1.1":  0,
		"2":    2,
		"3":    3,
	}
	for version, want := range tests {
		if got := requiredProtocol(version); got != want {
			t.Errorf("requiredProtocol(%q) = %d, want %d", version, got, want)
		}
	}
}

func TestAnalysisSkipWhenBudgetExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello needle")
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	client := clientFor(t, target)
	budget := NewAnalysisBudget(1)
	if err := budget.Acquire(context.Background(), 1<<20); err != nil {
		t.Fatal(err)
	}
	defer budget.Release(1 << 20)
	client.SetAnalysisBudget(budget)
	client.SetAnalysisPolicy(config.AnalysisSkip)

	step := prepareOne(t, target, config.Step{
		Name:    "one",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect:  config.Expect{Status: config.StatusCodes{200}, BodyContains: "needle"},
		Extract: map[string]string{"token": "json:token"},
	})
	res := client.Do(context.Background(), step, vars.Scope{})
	if !res.AnalysisSkipped {
		t.Fatal("AnalysisSkipped = false")
	}
	if res.Failed() {
		t.Fatalf("skipped analysis failed the step: %+v", res)
	}
	if res.ChecksTotal() != 1 {
		t.Fatalf("checks = %d, want only the status check", res.ChecksTotal())
	}
	if len(res.Extracted) != 0 {
		t.Fatalf("extracted = %+v, want none when analysis is skipped", res.Extracted)
	}
}

func TestAnalysisFailWhenBudgetExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello needle")
	}))
	defer srv.Close()

	target := targetFor(srv.URL)
	client := clientFor(t, target)
	budget := NewAnalysisBudget(1)
	if err := budget.Acquire(context.Background(), 1<<20); err != nil {
		t.Fatal(err)
	}
	defer budget.Release(1 << 20)
	client.SetAnalysisBudget(budget)
	client.SetAnalysisPolicy(config.AnalysisFail)

	step := prepareOne(t, target, config.Step{
		Name:    "one",
		Request: config.Request{Method: "GET", URL: "/"},
		Expect:  config.Expect{Status: config.StatusCodes{200}, BodyContains: "needle"},
	})
	res := client.Do(context.Background(), step, vars.Scope{})
	if res.ErrKind != ErrAnalysisBudget {
		t.Fatalf("ErrKind = %q, want %q", res.ErrKind, ErrAnalysisBudget)
	}
	if !res.Failed() {
		t.Fatal("failed analysis did not mark the step as failed")
	}
}

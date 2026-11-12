// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/quic-go/quic-go/http3"
)

// Phase 0 spike: confirm that quic-go v0.65.0 exposes the API the HTTP/3
// transport will rely on, and that a local HTTP/3 server works on loopback.
var (
	_ http.RoundTripper                   = (*http3.Transport)(nil)
	_ io.Closer                           = (*http3.Transport)(nil)
	_ interface{ CloseIdleConnections() } = (*http3.Transport)(nil)
)

func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "siafu-http3-spike"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func startHTTP3Server(t *testing.T, handler http.Handler) (addr string, certPEM []byte, rootCAs *x509.CertPool) {
	t.Helper()
	cert := selfSignedCert(t)
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("UDP no disponible en loopback: %v", err)
	}
	srv := &http3.Server{
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		Handler:   handler,
	}
	go func() { _ = srv.Serve(pc) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = pc.Close()
	})
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	return pc.LocalAddr().String(), pemBytes, pool
}

func TestHTTP3SpikeLocalServer(t *testing.T) {
	addr, _, pool := startHTTP3Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Proto", r.Proto)
		_, _ = io.WriteString(w, "hola h3")
	}))

	tr := &http3.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}
	defer tr.Close()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+addr+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if resp.ProtoMajor != 3 {
		t.Errorf("ProtoMajor = %d; quiero 3", resp.ProtoMajor)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; quiero 200", resp.StatusCode)
	}
	if string(body) != "hola h3" {
		t.Errorf("body = %q; quiero %q", body, "hola h3")
	}
	if got := resp.Header.Get("X-Proto"); got != "HTTP/3.0" {
		t.Errorf("X-Proto = %q; quiero HTTP/3.0", got)
	}
}

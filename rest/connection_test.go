/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rest

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	http_client "github.com/go418/http-client"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
)

type tcpLB struct {
	t         *testing.T
	ln        net.Listener
	serverURL string
	dials     int32
}

func (lb *tcpLB) handleConnection(in net.Conn, ctx context.Context) {
	out, err := net.Dial("tcp", lb.serverURL)
	if err != nil {
		lb.t.Log(err)
		return
	}
	go io.Copy(out, in)
	go io.Copy(in, out)
	<-ctx.Done()
	if err := out.Close(); err != nil {
		lb.t.Fatalf("failed to close connection: %v", err)
	}
}

func (lb *tcpLB) serve(ctx context.Context) {
	conn, err := lb.ln.Accept()
	if err != nil {
		lb.t.Fatalf("failed to accept: %v", err)
	}
	atomic.AddInt32(&lb.dials, 1)
	lb.handleConnection(conn, ctx)
}

func newLB(t *testing.T, serverURL string) *tcpLB {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind: %v", err)
	}
	lb := tcpLB{
		serverURL: serverURL,
		ln:        ln,
		t:         t,
	}
	return &lb
}

func setEnv(key, value string) func() {
	originalValue := os.Getenv(key)
	os.Setenv(key, value)
	return func() {
		os.Setenv(key, originalValue)
	}
}

const (
	readIdleTimeout int = 1
	pingTimeout     int = 1
)

func TestReconnectBrokenTCP(t *testing.T) {
	defer setEnv("HTTP2_READ_IDLE_TIMEOUT_SECONDS", strconv.Itoa(readIdleTimeout))()
	defer setEnv("HTTP2_PING_TIMEOUT_SECONDS", strconv.Itoa(pingTimeout))()
	defer setEnv("DISABLE_HTTP2", "")()
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %s", r.Proto)
	}))
	ts.EnableHTTP2 = true
	ts.StartTLS()
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("failed to parse URL from %q: %v", ts.URL, err)
	}
	lb := newLB(t, u.Host)
	defer lb.ln.Close()
	ctx, cancel := context.WithCancel(context.TODO())
	go lb.serve(ctx)
	transport, ok := ts.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("failed to assert *http.Transport")
	}
	client, err := Config{
		Host: "https://" + lb.ln.Addr().String(),
		// These fields are required to create a REST client.
		Negotiator: newNegotiator(t),
	}.BuildManual(
		http_client.ManualCloneRequest(),
		http_client.Timeout(1*time.Second),
		http_client.ManualDefaultClient(),
		http_client.ManualTransport(utilnet.SetTransportDefaults(transport)),
	)
	if err != nil {
		t.Fatalf("failed to create REST client: %v", err)
	}
	data, err := client.Get().AbsPath("/").DoRaw(context.TODO())
	if err != nil {
		t.Fatalf("unexpected err: %s: %v", data, err)
	}
	if string(data) != "Hello, HTTP/2.0" {
		t.Fatalf("unexpected response: %s", data)
	}

	// Deliberately let the LB stop proxying traffic for the current
	// connection. This mimics a broken TCP connection that's not properly
	// closed.
	cancel()

	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()
	go lb.serve(ctx)
	// Sleep enough time for the HTTP/2 health check to detect and close
	// the broken TCP connection.
	time.Sleep(time.Duration(1+readIdleTimeout+pingTimeout) * time.Second)
	// If the HTTP/2 health check were disabled, the broken connection
	// would still be in the connection pool, the following request would
	// then reuse the broken connection instead of creating a new one, and
	// thus would fail.
	data, err = client.Get().AbsPath("/").DoRaw(context.TODO())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(data) != "Hello, HTTP/2.0" {
		t.Fatalf("unexpected response: %s", data)
	}
	dials := atomic.LoadInt32(&lb.dials)
	if dials != 2 {
		t.Fatalf("expected %d dials, got %d", 2, dials)
	}
}

// 1. connect to https server with http1.1 using a TCP proxy
// 2. the connection has keepalive enabled so it will be reused
// 3. break the TCP connection stopping the proxy
// 4. close the idle connection to force creating a new connection
// 5. count that there are 2 connection to the server (we didn't reuse the original connection)
func TestReconnectBrokenTCP_HTTP1(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %s", r.Proto)
	}))
	ts.EnableHTTP2 = false
	ts.StartTLS()
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("failed to parse URL from %q: %v", ts.URL, err)
	}
	lb := newLB(t, u.Host)
	defer lb.ln.Close()
	ctx, cancel := context.WithCancel(context.TODO())
	go lb.serve(ctx)
	transport, ok := ts.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatal("failed to assert *http.Transport")
	}
	client, err := Config{
		Host: "https://" + lb.ln.Addr().String(),
		// These fields are required to create a REST client.
		Negotiator: newNegotiator(t),
	}.BuildManual(
		http_client.ManualCloneRequest(),
		// large timeout, otherwise the broken connection will be cleaned by it
		http_client.Timeout(wait.ForeverTestTimeout),
		http_client.DisableHttp2(),
		http_client.ManualDefaultClient(),
		http_client.ManualTransport(utilnet.SetTransportDefaults(transport)),
	)
	if err != nil {
		t.Fatalf("failed to create REST client: %v", err)
	}

	data, err := client.Get().AbsPath("/").DoRaw(context.TODO())
	if err != nil {
		t.Fatalf("unexpected err: %s: %v", data, err)
	}
	if string(data) != "Hello, HTTP/1.1" {
		t.Fatalf("unexpected response: %s", data)
	}

	// Deliberately let the LB stop proxying traffic for the current
	// connection. This mimics a broken TCP connection that's not properly
	// closed.
	cancel()

	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()
	go lb.serve(ctx)
	// Close the idle connections
	utilnet.CloseIdleConnectionsFor(client.Client.(*http.Client).Transport)

	// If the client didn't close the idle connections, the broken connection
	// would still be in the connection pool, the following request would
	// then reuse the broken connection instead of creating a new one, and
	// thus would fail.
	data, err = client.Get().AbsPath("/").DoRaw(context.TODO())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(data) != "Hello, HTTP/1.1" {
		t.Fatalf("unexpected response: %s", data)
	}
	dials := atomic.LoadInt32(&lb.dials)
	if dials != 2 {
		t.Fatalf("expected %d dials, got %d", 2, dials)
	}
}

// 1. connect to https server with http1.1 using a TCP proxy making the connection to timeout
// 2. the connection has keepalive enabled so it will be reused
// 3. close the in-flight connection to force creating a new connection
// 4. count that there are 2 connection on the LB but only one succeeds
func TestReconnectBrokenTCPInFlight_HTTP1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	done := make(chan struct{})
	defer close(done)
	received := make(chan struct{})

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hang" {
			conn, _, _ := w.(http.Hijacker).Hijack()
			close(received)
			<-done
			conn.Close()
		}
		fmt.Fprintf(w, "Hello, %s", r.Proto)
	}))
	ts.EnableHTTP2 = false
	ts.StartTLS()
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("failed to parse URL from %q: %v", ts.URL, err)
	}

	lb := newLB(t, u.Host)
	defer lb.ln.Close()
	ctx1, cancel1 := context.WithCancel(ctx)
	go lb.serve(ctx1)

	transport, ok := ts.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatal("failed to assert *http.Transport")
	}
	client, err := Config{
		Host: "https://" + lb.ln.Addr().String(),
		// These fields are required to create a REST client.
		Negotiator: newNegotiator(t),
	}.BuildManual(
		http_client.ManualCloneRequest(),
		// large timeout, otherwise the broken connection will be cleaned by it
		http_client.Timeout(wait.ForeverTestTimeout),
		http_client.DisableHttp2(),
		http_client.ManualDefaultClient(),
		http_client.ManualTransport(utilnet.SetTransportDefaults(transport)),
	)
	if err != nil {
		t.Fatalf("failed to create REST client: %v", err)
	}

	// The request will connect, hang and eventually time out
	// but we can use a context to close once the test is done
	// we are only interested in have an inflight connection
	conCtx, conCancel := context.WithCancel(context.Background())
	reqErrCh := make(chan error, 1)
	defer close(reqErrCh)
	go func() {
		_, err = client.Get().AbsPath("/hang").DoRaw(conCtx)
		reqErrCh <- err
	}()

	// wait until it connect to the server
	select {
	case <-received:
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatal("Test timed out waiting for first request to fail")
	}

	// Deliberately let the LB stop proxying traffic for the current
	// connection. This mimics a broken TCP connection that's not properly
	// closed.
	cancel1()

	go lb.serve(ctx)

	// New request will fail if tries to reuse the connection
	data, err := client.Get().AbsPath("/").DoRaw(ctx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(data) != "Hello, HTTP/1.1" {
		t.Fatalf("unexpected response: %s", data)
	}
	dials := atomic.LoadInt32(&lb.dials)
	if dials != 2 {
		t.Fatalf("expected %d dials, got %d", 2, dials)
	}

	// cancel the in-flight connection
	conCancel()

	select {
	case <-reqErrCh:
		if err == nil {
			t.Fatal("Connection succeeded but was expected to timeout")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out waiting for the request to fail")
	}

}

func TestRestClientTimeout(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		fmt.Fprintf(w, "Hello, %s", r.Proto)
	}))
	ts.Start()
	defer ts.Close()

	client, err := Config{
		Host: ts.URL,
		// These fields are required to create a REST client.
		Negotiator: newNegotiator(t),
	}.Build(
		http_client.Timeout(1 * time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create REST client: %v", err)
	}
	_, err = client.Get().AbsPath("/").DoRaw(context.TODO())
	if err == nil {
		t.Fatalf("timeout error expected")
	}
	if !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("timeout error expected, received %v", err)
	}
}

/*
Copyright 2026.

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

// Package adminhttptest is the core of the fake administration servers that
// the client packages test against. It holds what every one of them needs:
// the httptest lifecycle, the lock over the state of the fake, the injected
// failures and connection drops, and the JSON reply.
//
// A fake embeds Fake, adds the state of the API it imitates, and serves its
// own handler. The body shape of an error stays with the fake, because each
// API has its own.
package adminhttptest

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync"
)

// Fake is the shared core of a fake administration server. Embed it, start
// it with Start or StartTLS, and read the injected failures from the handler
// with Failing and Dropping.
type Fake struct {
	mu sync.Mutex

	failures map[string]int
	drops    map[string]int

	server *httptest.Server
}

// Start serves handler over plain HTTP. Close it with Close.
func (f *Fake) Start(handler http.HandlerFunc) {
	f.start(httptest.NewServer, handler)
}

// StartTLS serves handler over HTTPS with a self-signed certificate, the way
// an operator meets a TLS-secured API. CertificatePEM returns the bundle
// that verifies it, so a test exercises the same CA path as production.
// Close it with Close.
func (f *Fake) StartTLS(handler http.HandlerFunc) {
	f.start(httptest.NewTLSServer, handler)
}

func (f *Fake) start(newServer func(http.Handler) *httptest.Server, handler http.HandlerFunc) {
	f.failures = map[string]int{}
	f.drops = map[string]int{}
	f.server = newServer(f.locked(handler))

	// Serve without keep-alive, so every call opens its own connection: the
	// Go transport retries an idempotent request that fails on a reused
	// connection, and a drop that it retried away would be invisible.
	f.server.Config.SetKeepAlivesEnabled(false)
}

// locked wraps handler so that one request holds the lock over the state of
// the fake for its whole duration. A handler therefore reads and writes that
// state directly, and an exported accessor sees one request at a time.
func (f *Fake) locked(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		handler(w, r)
	}
}

// Lock and Unlock guard the state of the fake. Every exported accessor of an
// embedding fake holds the lock. A handler must not take it, because it
// already runs under it.
func (f *Fake) Lock() { f.mu.Lock() }

// Unlock releases the lock that Lock takes.
func (f *Fake) Unlock() { f.mu.Unlock() }

// URL is the base URL of the fake.
func (f *Fake) URL() string { return f.server.URL }

// Close stops the fake.
func (f *Fake) Close() { f.server.Close() }

// CertificatePEM returns the PEM encoding of the certificate a StartTLS fake
// serves, which is its own CA: the certificate is self-signed. A fake served
// over plain HTTP has none and returns nil.
func (f *Fake) CertificatePEM() []byte {
	cert := f.server.Certificate()
	if cert == nil {
		return nil
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

// FailNext makes the next n calls of op fail. The handler of the embedding
// fake reads the count with Failing and decides what the failure looks like.
// The operation names are the ones that fake documents.
func (f *Fake) FailNext(op string, n int) {
	f.Lock()
	defer f.Unlock()
	f.failures[op] = n
}

// DropNext makes the next n calls of op close the connection without any
// response, the way a dropped route or a broken proxy does. A client reports
// such a call as unreachable rather than rejected. A fake can be reachable
// for one operation and unreachable for another, for example a proxy that
// serves GET and drops PUT. op takes the values of FailNext.
func (f *Fake) DropNext(op string, n int) {
	f.Lock()
	defer f.Unlock()
	f.drops[op] = n
}

// Failing consumes one injected failure of op and reports whether the
// handler must answer with one. It runs under the lock of the request.
func (f *Fake) Failing(op string) bool {
	if f.failures[op] <= 0 {
		return false
	}
	f.failures[op]--

	return true
}

// Dropping consumes one injected drop of op, closes the connection, and
// reports whether it did. A handler that gets true must return without
// writing. It runs under the lock of the request.
func (f *Fake) Dropping(w http.ResponseWriter, op string) bool {
	if f.drops[op] <= 0 {
		return false
	}
	f.drops[op]--

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		panic("adminhttptest: the response writer does not support hijacking")
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		panic("adminhttptest: hijacking the connection: " + err.Error())
	}
	_ = conn.Close()

	return true
}

// WriteJSON answers with status and the JSON encoding of body.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

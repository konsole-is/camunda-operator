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

package keycloakadmin

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
)

// Option changes a Client that New has built. Pass one to New. A Client is
// not safe to change after it has made a call.
type Option func(*Client)

// ErrNoCertificates says that a bundle holds no certificate in PEM form. It
// is the one failure of ParseCABundle that the bundle itself causes, so a
// caller reports it against the bundle and every other one against itself.
var ErrNoCertificates = errors.New("the bundle holds no certificate in PEM form")

// ParseCABundle returns the certificate pool that a Client verifies Keycloak
// with: the trust store of the operator image, plus every certificate in pem.
// An empty bundle comes back as ErrNoCertificates, so a caller can name it
// instead of reporting the failed handshake that comes of it.
func ParseCABundle(pem []byte) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("reading the trust store of the operator image: %w", err)
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, ErrNoCertificates
	}

	return pool, nil
}

// WithRootCAs verifies the certificate of Keycloak against pool instead of
// the trust store of the operator image. Build pool with ParseCABundle, which
// keeps that trust store in it.
func WithRootCAs(pool *x509.CertPool) Option {
	return func(c *Client) {
		transport := defaultTransport()
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.RootCAs = pool
		c.http.Transport = transport
	}
}

// defaultTransport copies the standard transport of net/http, so a client
// that changes the certificate pool keeps the proxy settings, the connection
// limits, and the TLS settings of every other client of the operator. The
// copy carries its own TLS settings, so a caller can write to them.
//
// A standard transport that another package replaced with an implementation
// of its own cannot be copied. A plain transport answers for it, because a
// type assertion that fails here would panic in a reconcile.
func defaultTransport() *http.Transport {
	if standard, ok := http.DefaultTransport.(*http.Transport); ok {
		return standard.Clone()
	}

	return &http.Transport{}
}

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

// ParseCABundle returns the certificate pool that a Client verifies Keycloak
// with: the trust store of the operator image, plus every certificate of pem.
// A bundle that holds no certificate in PEM form is an error, so a caller can
// name the empty bundle instead of reporting the failed handshake that comes
// of it.
func ParseCABundle(pem []byte) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("reading the trust store of the operator image: %w", err)
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("the bundle holds no certificate in PEM form")
	}

	return pool, nil
}

// WithRootCAs verifies the certificate of Keycloak against pool instead of
// the trust store of the operator image. Build pool with ParseCABundle, which
// keeps that trust store in it.
func WithRootCAs(pool *x509.CertPool) Option {
	return func(c *Client) {
		transport := defaultTransport()
		transport.TLSClientConfig = &tls.Config{RootCAs: pool}
		c.http.Transport = transport
	}
}

// defaultTransport copies the transport that a client without an Option uses,
// so the proxy settings and the connection limits of the operator stay the
// same when only the certificate pool changes.
func defaultTransport() *http.Transport {
	if standard, ok := http.DefaultTransport.(*http.Transport); ok {
		return standard.Clone()
	}

	return &http.Transport{}
}

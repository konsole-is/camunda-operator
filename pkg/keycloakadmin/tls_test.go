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
	"context"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRootCAsReachesAKeycloakBehindAPrivateAuthority(t *testing.T) {
	fake := &fakeKeycloak{token: "an-access-token", handler: oneClient}
	base, bundle := fake.startTLS(t)

	pool, err := ParseCABundle(bundle)
	require.NoError(t, err)

	client := New(base, "camunda-platform", "admin", "s3cret", WithRootCAs(pool))
	found, err := client.FindClient(context.Background(), "optimize")

	require.NoError(t, err)
	assert.Equal(t, "optimize", found["clientId"])
}

// Without the bundle the certificate of the fake is signed by an authority
// that no system trust store carries, so the handshake fails.
func TestNewRefusesAnUnknownAuthority(t *testing.T) {
	fake := &fakeKeycloak{token: "an-access-token", handler: oneClient}
	base, _ := fake.startTLS(t)

	client := New(base, "camunda-platform", "admin", "s3cret")
	_, err := client.FindClient(context.Background(), "optimize")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

// The caller reports the empty bundle against the Secret of the user, so it
// has to tell that failure from every other one.
func TestParseCABundleRefusesTextThatHoldsNoCertificate(t *testing.T) {
	_, err := ParseCABundle([]byte("not a certificate"))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoCertificates)
}

// The pool answers for the certificates of the bundle and for the ones the
// image already trusts. A Keycloak with a public certificate therefore keeps
// working after a bundle is added.
func TestParseCABundleKeepsTheSystemTrustStore(t *testing.T) {
	system, err := x509.SystemCertPool()
	require.NoError(t, err)
	if system.Equal(x509.NewCertPool()) {
		t.Skip("the system trust store of this machine holds no certificate")
	}

	fake := &fakeKeycloak{token: "an-access-token", handler: oneClient}
	_, bundle := fake.startTLS(t)

	pool, err := ParseCABundle(bundle)
	require.NoError(t, err)

	onlyBundle := x509.NewCertPool()
	require.True(t, onlyBundle.AppendCertsFromPEM(bundle))
	assert.False(t, pool.Equal(onlyBundle))
}

// oneClient answers the client lookup with the Optimize client of the realm.
func oneClient(w http.ResponseWriter, _ *http.Request, _ string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`[{"id":"6c4c0c5c","clientId":"optimize"}]`))
}

// startTLS runs the fake behind an https server and returns its base URL and
// the certificate of that server as a PEM bundle.
func (f *fakeKeycloak) startTLS(t *testing.T) (string, []byte) {
	t.Helper()

	server := httptest.NewTLSServer(f.serve(t))
	t.Cleanup(server.Close)

	bundle := pem.EncodeToMemory(
		&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw},
	)

	return server.URL + "/auth", bundle
}

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

package utils

import (
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
)

const (
	downloadAttempts = 4
	downloadBackoff  = 5 * time.Second
	downloadTimeout  = 2 * time.Minute
)

// applyRemoteManifest downloads the manifest at url and runs kubectl apply
// on it with applyArgs. A server error or a connection error is tried again a
// few times before it fails the apply.
func applyRemoteManifest(url string, applyArgs ...string) error {
	manifest, err := download(url, downloadAttempts, downloadBackoff)
	if err != nil {
		return err
	}

	args := append([]string{"apply"}, applyArgs...)
	args = append(args, "-f", "-")
	_, err = KubectlWithStdin(string(manifest), args...)

	return err
}

// download returns the body of url. It makes at most attempts tries and
// waits backoff before the second, twice as long before each next one. Only
// a connection error or a 5xx gets another try. Any other status fails at
// once.
func download(url string, attempts int, backoff time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: downloadTimeout}
	for attempt := 1; ; attempt++ {
		body, transient, err := downloadOnce(client, url)
		if err == nil || !transient || attempt == attempts {
			return body, err
		}

		_, _ = fmt.Fprintf(GinkgoWriter, "Download failed, trying again in %s: %v\n", backoff, err)
		time.Sleep(backoff)
		backoff *= 2
	}
}

// downloadOnce reports as transient an error that a later try can clear.
func downloadOnce(client *http.Client, url string) (body []byte, transient bool, err error) {
	resp, err := client.Get(url) // nolint:gosec // a release URL that this package builds
	if err != nil {
		return nil, true, fmt.Errorf("downloading %q: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode >= http.StatusInternalServerError,
			fmt.Errorf("downloading %q: HTTP %d", url, resp.StatusCode)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("reading %q: %w", url, err)
	}

	return body, false, nil
}

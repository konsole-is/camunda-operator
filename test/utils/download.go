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

	. "github.com/onsi/ginkgo/v2" // nolint:staticcheck
)

const (
	downloadAttempts = 4
	downloadBackoff  = 5 * time.Second
	downloadTimeout  = 2 * time.Minute
)

// applyRemoteManifest runs kubectl apply with applyArgs on the manifest at
// url. A 5xx or a connection error gets a few more tries.
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

// download retries a connection error or a 5xx, up to attempts tries in
// all. Any other status fails at once.
func download(url string, attempts int, backoff time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: downloadTimeout}

	return retry(attempts, backoff, func() ([]byte, bool, error) { return downloadOnce(client, url) })
}

// retry calls try at least once and at most attempts times. It stops at a
// success or at an error that try reports as not transient. It waits backoff
// before the second call and twice as long before each next one.
func retry[T any](attempts int, backoff time.Duration, try func() (T, bool, error)) (T, error) {
	for attempt := 1; ; attempt++ {
		result, transient, err := try()
		if err == nil || !transient || attempt >= attempts {
			return result, err
		}

		_, _ = fmt.Fprintf(GinkgoWriter, "Request failed, trying again in %s: %v\n", backoff, err)
		time.Sleep(backoff)
		backoff *= 2
	}
}

func downloadOnce(client *http.Client, url string) (body []byte, transient bool, err error) {
	resp, err := client.Get(url)
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

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

// Command helmrelease measures the release Secret that Helm writes for the
// chart. Helm stores a release as base64(gzip(json(release))) in one Secret,
// and the API server rejects a Secret with more than 1 MiB of data, so an
// install of a chart that passes that size fails. The tool reads the release
// that `helm install --dry-run=client -o json` prints, encodes it as Helm
// does, and prints the size and its share of the limit. It fails when the size
// is above -max. `make helm-verify` runs it.
//
// Usage: helm install ... --dry-run=client -o json | helmrelease -max bytes [-label text]
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// secretLimit is the most data, in bytes, that the API server accepts in one
// Secret.
const secretLimit = 1048576

func main() {
	maxBytes := flag.Int("max", secretLimit, "the largest release Secret, in bytes, that passes")
	label := flag.String("label", "", "a name for the measured configuration, printed with the size")
	flag.Parse()

	if err := run(os.Stdin, *maxBytes, *label); err != nil {
		fmt.Fprintln(os.Stderr, "helmrelease:", err)
		os.Exit(1)
	}
}

func run(input io.Reader, maxBytes int, label string) error {
	release, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	data, err := secretData(release)
	if err != nil {
		return err
	}
	size := len(data)

	fmt.Printf(
		"  %8d bytes, %3.0f%% of the Secret limit (max %d)  %s\n",
		size,
		100*float64(size)/secretLimit,
		maxBytes,
		label,
	)
	if size > maxBytes {
		return fmt.Errorf(
			"the release Secret is %d bytes, above the max of %d (%s)",
			size,
			maxBytes,
			label,
		)
	}

	return nil
}

// secretData returns the data that Helm stores in the release Secret for the
// release JSON. The JSON may hold whitespace, which Helm does not store. It
// fails when the input is not a JSON object.
func secretData(release []byte) ([]byte, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, release); err != nil {
		return nil, fmt.Errorf("reading the release JSON: %w", err)
	}
	if !bytes.HasPrefix(compact.Bytes(), []byte("{")) {
		return nil, errors.New("the release JSON is not an object")
	}

	// Helm compresses the release at gzip.BestCompression.
	var zipped bytes.Buffer
	zw, err := gzip.NewWriterLevel(&zipped, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(compact.Bytes()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	data := make([]byte, base64.StdEncoding.EncodedLen(zipped.Len()))
	base64.StdEncoding.Encode(data, zipped.Bytes())

	return data, nil
}

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

// Package imageref splits the image references that the chart generators
// write into the values of the chart.
package imageref

import (
	"fmt"
	"strings"
)

// Split separates repository and tag. It rejects a digest reference, a
// tagless image, and an empty repository or tag, because the chart exposes
// exactly repository and tag for each image.
func Split(image string) (string, string, error) {
	colon := strings.LastIndex(image, ":")
	if colon < 0 || strings.Contains(image[colon:], "/") || strings.Contains(image, "@") {
		return "", "", fmt.Errorf("image %q must be <repository>:<tag>", image)
	}
	repository, tag := image[:colon], image[colon+1:]
	if repository == "" || tag == "" {
		return "", "", fmt.Errorf("image %q must be <repository>:<tag>, both non-empty", image)
	}

	return repository, tag, nil
}

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

package reconcilers_test

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/konsole-is/camunda-operator/test/envtest"
)

// controllerDir holds one package per controller, some of them one level
// down under a group directory.
const controllerDir = "../controller"

var _ = Describe("Setup", func() {
	It("starts one controller for each controller package, named after the package", func() {
		want := controllerPackages()
		Expect(want).NotTo(BeEmpty())

		Eventually(startedControllers, time.Minute, envtest.Interval).Should(ConsistOf(want))
	})
})

// controllerPackages returns the name of each directory under controllerDir
// that holds a non-test .go file. A directory without one, such as a group
// directory or a package of tests only, is walked into and not counted.
func controllerPackages() []string {
	GinkgoHelper()

	var names []string
	err := filepath.WalkDir(controllerDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || path == controllerDir {
			return err
		}

		files, err := filepath.Glob(filepath.Join(path, "*.go"))
		if err != nil {
			return err
		}
		for _, file := range files {
			if !strings.HasSuffix(file, "_test.go") {
				names = append(names, entry.Name())
				break
			}
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	return names
}

// startedControllers returns the names of the controllers that started in
// this process. A controller sets its worker count gauge when it starts.
func startedControllers() []string {
	GinkgoHelper()

	families, err := ctrlmetrics.Registry.Gather()
	Expect(err).NotTo(HaveOccurred())

	var names []string
	for _, family := range families {
		if family.GetName() != "controller_runtime_max_concurrent_reconciles" {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "controller" {
					names = append(names, label.GetValue())
				}
			}
		}
	}

	return names
}

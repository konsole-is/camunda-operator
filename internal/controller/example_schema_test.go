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

package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// exampleDir holds the apply-able inventories, relative to the module
	// root. Every directory under it with a kustomization.yaml is one.
	exampleDir = "config/example"
	// kustomizationFile lists the manifests of an inventory. It is not itself
	// a Kubernetes object, so the walk marks a directory with it and the
	// apply skips it.
	kustomizationFile = "kustomization.yaml"
	// decodeBuffer is the read-ahead the YAML decoder uses to tell YAML from
	// JSON.
	decodeBuffer = 4096
)

var _ = Describe("config/example", func() {
	It("applies every inventory against the schema", func() {
		root, err := utils.ModuleRoot()
		Expect(err).NotTo(HaveOccurred())

		inventories := findInventories(filepath.Join(root, exampleDir))
		Expect(inventories).NotTo(BeEmpty(), "no directory under config/example holds a kustomization.yaml")

		for _, dir := range inventories {
			applyInventory(dir)
		}
	})
})

func findInventories(root string) []string {
	GinkgoHelper()

	var dirs []string
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && entry.Name() == kustomizationFile {
			dirs = append(dirs, filepath.Dir(path))
		}

		return nil
	}
	Expect(filepath.WalkDir(root, walk)).To(Succeed())

	return dirs
}

// applyInventory walks the manifests in file-name order, which is the order
// the README of the inventory applies them. Every one of them must also be a
// resource of the kustomization, because a file the kustomization does not
// list is a file that kubectl apply -k silently leaves out.
func applyInventory(dir string) {
	GinkgoHelper()

	listed := listedResources(dir)

	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred(), dir)

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == kustomizationFile {
			continue
		}

		if filepath.Ext(entry.Name()) == ".yaml" {
			path := filepath.Join(dir, entry.Name())
			Expect(listed).To(ContainElement(entry.Name()), path+" is not a resource of "+kustomizationFile)
			applyFile(path)
		}
	}
}

func listedResources(dir string) []string {
	GinkgoHelper()

	path := filepath.Join(dir, kustomizationFile)

	data, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), path)

	var manifest struct {
		Resources []string `json:"resources"`
	}
	Expect(utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), decodeBuffer).Decode(&manifest)).To(Succeed(), path)

	return manifest.Resources
}

func applyFile(path string) {
	GinkgoHelper()

	data, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), path)

	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), decodeBuffer)
	for {
		var obj unstructured.Unstructured

		err := decoder.Decode(&obj.Object)
		if errors.Is(err, io.EOF) {
			return
		}
		Expect(err).NotTo(HaveOccurred(), path)

		// A document that holds only comments decodes to nothing.
		if len(obj.Object) > 0 {
			applyObject(path, &obj)
		}
	}
}

// applyObject creates a Namespace for real and every other object as a dry
// run. The API server rejects a namespaced object whose namespace is absent,
// even on a dry run, so the namespaces an inventory declares must exist for
// the rest of it to validate.
//
// The namespaces are left behind on purpose. envtest runs no namespace
// controller, so a deleted namespace never leaves Terminating, and nothing
// can be created in it again. Cleaning up would break a later run of this
// spec rather than isolate it.
//
// Strict field validation turns a misspelled field into an error. A CRD
// schema prunes an unknown field without it, so a typo would pass unseen.
func applyObject(path string, obj *unstructured.Unstructured) {
	GinkgoHelper()

	where := fmt.Sprintf("%s: %s %s", path, obj.GetKind(), obj.GetName())
	strict := client.FieldValidation(metav1.FieldValidationStrict)

	if obj.GetKind() == "Namespace" {
		err := k8sClient.Create(ctx, obj, strict)
		if !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred(), where)
		}

		return
	}

	Expect(k8sClient.Create(ctx, obj, client.DryRunAll, strict)).To(Succeed(), where)
}

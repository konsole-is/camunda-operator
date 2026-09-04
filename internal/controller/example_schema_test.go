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
	"slices"

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
	// namespaceKind is the one kind the apply creates for real.
	namespaceKind = "Namespace"
	// decodeBuffer is the read-ahead the YAML decoder uses to tell YAML from
	// JSON.
	decodeBuffer = 4096
)

// manifestExtensions are the file types kustomize reads as a resource. A
// resource that is not a directory and carries none of them, a README for
// example, makes kubectl apply -k fail.
var manifestExtensions = []string{".yaml", ".yml", ".json"}

// fieldValidationStrict turns a misspelled field into an error. A CRD schema
// prunes an unknown field without it, so a typo passes unseen.
var fieldValidationStrict = client.FieldValidation(metav1.FieldValidationStrict)

// manifestObject is one decoded document with the file it came from, so that a
// failure names the file and not only the object.
type manifestObject struct {
	path   string
	object *unstructured.Unstructured
}

func (o manifestObject) where() string {
	return fmt.Sprintf("%s: %s %s", o.path, o.object.GetKind(), o.object.GetName())
}

var _ = Describe("config/example", func() {
	root := moduleRoot()

	inventories := findInventories(root)
	entries := make([]any, 0, len(inventories))
	for _, dir := range inventories {
		entries = append(entries, Entry(dir, filepath.Join(root, dir)))
	}

	DescribeTable(
		"lists every manifest of an inventory in its kustomization",
		append([]any{checkFileSet}, entries...)...,
	)
	DescribeTable(
		"applies every manifest of an inventory against the schema",
		append([]any{applyManifests}, entries...)...,
	)
})

// moduleRoot and findInventories run during tree construction, where a Gomega
// assertion is not allowed, so they panic instead.
func moduleRoot() string {
	root, err := utils.ModuleRoot()
	if err != nil {
		panic(err)
	}

	return root
}

// findInventories returns each inventory directory relative to root, which is
// also the name that a table entry carries.
func findInventories(root string) []string {
	var dirs []string
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || entry.Name() != kustomizationFile {
			return nil
		}

		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		dirs = append(dirs, rel)

		return nil
	}

	if err := filepath.WalkDir(filepath.Join(root, exampleDir), walk); err != nil {
		panic(err)
	}

	if len(dirs) == 0 {
		panic("no directory under " + exampleDir + " holds a " + kustomizationFile)
	}

	return dirs
}

// checkFileSet requires the manifests of an inventory and the resources of its
// kustomization to be the same set, so that the numbered path of the README and
// kubectl apply -k stand the same scenario up. A manifest that the
// kustomization does not list is one that kubectl apply -k leaves out. A
// resource that it lists but that is not there makes kubectl apply -k fail
// outright. A resource can be a directory, as the shared presets are.
//
// Only the set is checked, not the order. Kustomize sorts its output by kind
// and puts Namespace first, so the order of the resources list is not the order
// anything is applied in. The operator resolves a reference after the resources
// exist, so a scenario stands up whatever order it arrives in.
func checkFileSet(dir string) {
	GinkgoHelper()

	listed := listedResources(dir)

	for _, resource := range listed {
		path := filepath.Join(dir, resource)

		info, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred(), kustomizationFile+" of "+dir+" lists "+resource+", which is not there")

		if info.IsDir() {
			_, err := os.Stat(filepath.Join(path, kustomizationFile))
			Expect(err).NotTo(HaveOccurred(), path+" is a resource directory with no "+kustomizationFile)

			continue
		}

		ext := filepath.Ext(resource)
		Expect(manifestExtensions).To(ContainElement(ext), path+" is a resource kustomize cannot read")
	}

	for _, name := range manifestNames(dir) {
		Expect(listed).To(ContainElement(name), filepath.Join(dir, name)+" is not a resource of "+kustomizationFile)
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

// manifestNames returns the manifest files of dir in file-name order, which is
// the order the README of the inventory applies them.
func manifestNames(dir string) []string {
	GinkgoHelper()

	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred(), dir)

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == kustomizationFile {
			continue
		}

		if slices.Contains(manifestExtensions, filepath.Ext(entry.Name())) {
			names = append(names, entry.Name())
		}
	}

	return names
}

// applyManifests creates the namespaces of an inventory in a first pass and
// validates everything else in a second. The API server rejects a namespaced
// object whose namespace is absent, even on a dry run, so the namespaces must
// exist before the rest of the inventory reaches it.
func applyManifests(dir string) {
	GinkgoHelper()

	objects := decodeManifests(dir)

	for _, obj := range objects {
		if obj.object.GetKind() == namespaceKind {
			createNamespace(obj)
		}
	}

	for _, obj := range objects {
		if obj.object.GetKind() != namespaceKind {
			validateObject(obj)
		}
	}
}

func decodeManifests(dir string) []manifestObject {
	GinkgoHelper()

	var objects []manifestObject
	for _, name := range manifestNames(dir) {
		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred(), path)

		decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), decodeBuffer)
		for {
			var obj unstructured.Unstructured

			err := decoder.Decode(&obj.Object)
			if errors.Is(err, io.EOF) {
				break
			}
			Expect(err).NotTo(HaveOccurred(), path)

			// A document that holds only comments decodes to nothing.
			if len(obj.Object) > 0 {
				objects = append(objects, manifestObject{path: path, object: &obj})
			}
		}
	}

	return objects
}

// createNamespace creates the namespace for real, because a dry run leaves
// nothing for the rest of the inventory to land in.
//
// The namespaces are left behind on purpose. envtest runs no namespace
// controller, so a deleted namespace never leaves Terminating, and nothing can
// be created in it again. Cleanup breaks a later run of this spec rather than
// isolate it.
//
// Two inventories can declare the same namespace, and the second Create then
// reports AlreadyExists. That does not hide a bad manifest: the API server
// decodes and validates before it looks the name up, so a typo or a broken
// value in the later copy still comes back as its own error.
func createNamespace(obj manifestObject) {
	GinkgoHelper()

	err := k8sClient.Create(ctx, obj.object, fieldValidationStrict)
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred(), obj.where())
	}
}

func validateObject(obj manifestObject) {
	GinkgoHelper()

	Expect(k8sClient.Create(ctx, obj.object, client.DryRunAll, fieldValidationStrict)).To(Succeed(), obj.where())
}

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

package grantadmission

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// sampleRegistration, pluginNamespace, and pluginServiceAccount are the
	// names of config/samples/plugins_serve_clusterrole.yaml and of the
	// PluginRegistration sample.
	sampleRegistration   = "s3-backup-bucket"
	pluginNamespace      = "camunda-plugins"
	pluginServiceAccount = "s3-backup-bucket-controller"
	// strangerServiceAccount can write grants but serves no registration.
	strangerServiceAccount = "stranger"

	// messageMissingLabel and messageNoServe are parts of the messages of
	// config/plugins/grant_admission_policy.yaml.
	messageMissingLabel = "a Grant must carry the label plugins.camunda.io/registration"
	messageNoServe      = "does not have the verb serve on the PluginRegistration"
)

// applyKustomization creates each file that the resources list of the
// kustomization in dir names, relative to the module root.
func applyKustomization(dir string) {
	GinkgoHelper()

	root, err := utils.ModuleRoot()
	Expect(err).NotTo(HaveOccurred())

	data, err := os.ReadFile(filepath.Join(root, dir, "kustomization.yaml"))
	Expect(err).NotTo(HaveOccurred())

	var kustomization struct {
		Resources []string `json:"resources"`
	}
	Expect(yaml.Unmarshal(data, &kustomization)).To(Succeed())
	Expect(kustomization.Resources).NotTo(BeEmpty())

	for _, resource := range kustomization.Resources {
		applyManifests(filepath.Join(dir, resource))
	}
}

// applyManifests creates each object of the YAML file at path, relative to
// the module root, as the admin.
func applyManifests(path string) {
	GinkgoHelper()

	root, err := utils.ModuleRoot()
	Expect(err).NotTo(HaveOccurred())

	file, err := os.Open(filepath.Join(root, path))
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = file.Close() }()

	decoder := utilyaml.NewYAMLOrJSONDecoder(file, 4096)
	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(&obj.Object)
		if errors.Is(err, io.EOF) {
			return
		}
		Expect(err).NotTo(HaveOccurred(), path)

		if len(obj.Object) == 0 {
			continue
		}
		Expect(admin.Create(ctx, obj)).To(Succeed(), path)
	}
}

// asServiceAccount returns a client that sends each request as the
// ServiceAccount namespace/name.
func asServiceAccount(namespace, name string) client.Client {
	GinkgoHelper()

	cfg := rest.CopyConfig(env.Cfg)
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: "system:serviceaccount:" + namespace + ":" + name,
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	return c
}

// bindClusterRole creates the ClusterRole name with rules and binds it to the
// ServiceAccount namespace/serviceAccount.
func bindClusterRole(name string, rules []rbacv1.PolicyRule, namespace, serviceAccount string) {
	GinkgoHelper()

	Expect(admin.Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Rules:      rules,
	})).To(Succeed())

	Expect(admin.Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: namespace,
			Name:      serviceAccount,
		}},
	})).To(Succeed())
}

// newGrant returns a valid grant with a generated name and the given labels,
// in the shape that a plugin controller writes.
func newGrant(labels map[string]string) *pluginsv1alpha1.Grant {
	return &pluginsv1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "orders-backup-storage-",
			Namespace:    metav1.NamespaceDefault,
			Labels:       labels,
		},
		Spec: pluginsv1alpha1.GrantSpec{
			Cluster:  v1.ClusterRef{Name: "orders"},
			Template: "bucket-access",
			Format:   pluginsv1alpha1.GrantFormatAWSIAMStatement,
			Fragment: &apiextensionsv1.JSON{
				Raw: []byte(
					`{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject"],"Resource":["arn:aws:s3:::orders-backups/*"]}`,
				),
			},
		},
	}
}

// grantLabels returns the labels that a plugin controller sets on a Grant of
// registration.
func grantLabels(registration string) map[string]string {
	return map[string]string{
		pluginsv1alpha1.LabelRegistration: registration,
		pluginsv1alpha1.LabelRequest:      "orders-backup-storage",
		pluginsv1alpha1.LabelCluster:      "orders",
	}
}

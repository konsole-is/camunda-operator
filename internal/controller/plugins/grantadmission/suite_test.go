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

// Package grantadmission holds the admission specs of the Grant policy that
// the operator installs: who can write a Grant, and what the API server
// refuses, with which message. This package has only tests.
package grantadmission

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

var (
	env   *envtest.Env
	ctx   context.Context
	admin client.Client
)

func TestGrantAdmission(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Grant Admission Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// The specs test admission only, so the suite registers no reconciler.
	env = envtest.Start(func(ctrl.Manager) error { return nil })
	ctx, admin = env.Ctx, env.Client

	By("installing the admission objects that the operator ships")
	applyKustomization(filepath.Join("config", "plugins"))

	By("installing the example role of a plugin chart")
	applyManifests(filepath.Join("config", "plugins", "example_serve_clusterrole.yaml"))

	By("giving a second ServiceAccount write access to grants, without serve")
	bindClusterRole(
		"grant-writer",
		pluginNamespace,
		strangerServiceAccount,
		rbacv1.PolicyRule{
			APIGroups: []string{pluginsv1alpha1.SchemeGroupVersion.Group},
			Resources: []string{"grants"},
			Verbs:     []string{"get", "create", "update", "patch"},
		},
	)

	// The policy acts a short time after it is created. A refusal of an
	// admin request shows that it acts, because the binding covers every
	// requester.
	By("waiting until the policy refuses a Grant without the registration label")
	Eventually(func() error {
		return admin.Create(ctx, newGrant(nil))
	}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(messageMissingLabel)))
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})

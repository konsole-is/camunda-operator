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

package conformance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/conformance/echo"
)

var (
	env       *envtest.Environment
	restCfg   *rest.Config
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc
	pluginSeq atomic.Int32
)

func TestSelfTest(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Conformance self-test")
}

var _ = ginkgo.BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.Background())

	dir := envtestBinaryDir()
	Expect(dir).NotTo(BeEmpty(), "no envtest binaries: run make setup-envtest")

	env = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: dir,
	}

	var err error
	restCfg, err = env.Start()
	Expect(err).NotTo(HaveOccurred())

	scheme, err := newScheme()
	Expect(err).NotTo(HaveOccurred())
	k8sClient, err = client.New(restCfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	for name, outputs := range map[string][]string{
		"echo":        {"message"},
		"echo-source": {"value"},
		"access":      {"externalUrl", "grpcHost", "optimizeUrl"},
	} {
		section := &pluginsv1alpha1.PluginSection{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: pluginsv1alpha1.PluginSectionSpec{
				Label:     name,
				Selection: pluginsv1alpha1.SelectionOne,
				AppliesTo: []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim},
			},
		}
		for _, key := range outputs {
			section.Spec.Outputs = append(section.Spec.Outputs, pluginsv1alpha1.OutputDecl{Key: key})
		}
		Expect(k8sClient.Create(ctx, section)).To(Succeed())
	}
})

var _ = ginkgo.AfterSuite(func() {
	cancel()
	if env != nil {
		Expect(env.Stop()).To(Succeed())
	}
})

// envtestBinaryDir is KUBEBUILDER_ASSETS, else the first directory under
// ../bin/k8s, where make setup-envtest puts the binaries.
func envtestBinaryDir() string {
	if assets := os.Getenv("KUBEBUILDER_ASSETS"); assets != "" {
		return assets
	}

	base := filepath.Join("..", "bin", "k8s")
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(base, entry.Name())
		}
	}

	return ""
}

// installPlugin creates the namespace and the ServiceAccount of the plugin
// name, with a ClusterRole that a plugin chart gives its controller.
func installPlugin(name string) pluginsv1alpha1.ServiceAccountRef {
	sa := pluginsv1alpha1.ServiceAccountRef{Namespace: "plugin-" + name, Name: "echo"}

	Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: sa.Namespace}})).To(Succeed())
	Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Namespace: sa.Namespace, Name: sa.Name},
	})).To(Succeed())

	plugins := pluginsv1alpha1.GroupVersion.Group
	Expect(k8sClient.Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{plugins}, Resources: []string{"pluginregistrations"}, ResourceNames: []string{name},
				Verbs: []string{"serve"}},
			{APIGroups: []string{plugins}, Resources: []string{"pluginregistrations"},
				Verbs: []string{"get", "list", "watch", "create", "patch"}},
			{APIGroups: []string{plugins}, Resources: []string{"pluginregistrations/status"},
				Verbs: []string{"get", "update", "patch"}},
			{APIGroups: []string{plugins}, Resources: []string{"pluginrequests"},
				Verbs: []string{"get", "list", "watch", "update", "patch"}},
			{APIGroups: []string{plugins}, Resources: []string{"pluginrequests/status", "pluginrequests/finalizers"},
				Verbs: []string{"update"}},
			{APIGroups: []string{"", "storage.k8s.io", "networking.k8s.io"},
				Resources: []string{"configmaps", "storageclasses", "ingresses"},
				Verbs:     []string{"get", "list", "watch", "create", "patch", "delete"}},
			{APIGroups: []string{"events.k8s.io"}, Resources: []string{"events"}, Verbs: []string{"create", "patch"}},
		},
	})).To(Succeed())

	Expect(k8sClient.Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: name},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Namespace: sa.Namespace, Name: sa.Name},
		},
	})).To(Succeed())

	return sa
}

// startEcho runs an echo controller for opts in a manager of its own until
// the spec ends.
func startEcho(opts echo.Options) {
	scheme, err := newScheme()
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())

	c, err := echo.New(opts)
	Expect(err).NotTo(HaveOccurred())
	Expect(c.SetupWithManager(mgr)).To(Succeed())

	mgrCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer ginkgo.GinkgoRecover()
		defer close(done)
		Expect(mgr.Start(mgrCtx)).To(Succeed())
	}()

	ginkgo.DeferCleanup(func() {
		stop()
		Eventually(done, time.Minute).Should(BeClosed())
	})
}

// acceptRegistration writes Accepted=True on the registration name, in place
// of the registry controller. It writes again until a read shows both
// Accepted=True and the Ready=True of the plugin, so a write that the plugin
// raced is not lost.
func acceptRegistration(name string) {
	Eventually(func(g Gomega) {
		var reg pluginsv1alpha1.PluginRegistration
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &reg)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(reg.Status.Conditions, v1.ConditionReady)).To(BeTrue())

		if !meta.IsStatusConditionTrue(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted) {
			meta.SetStatusCondition(&reg.Status.Conditions, metav1.Condition{
				Type:    pluginsv1alpha1.ConditionAccepted,
				Status:  metav1.ConditionTrue,
				Reason:  v1.ReasonHealthy,
				Message: "The self-test accepts the registration",
			})
			g.Expect(k8sClient.Status().Update(ctx, &reg)).To(Succeed())
		}

		var again pluginsv1alpha1.PluginRegistration
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &again)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(again.Status.Conditions, pluginsv1alpha1.ConditionAccepted)).To(BeTrue())
		g.Expect(meta.IsStatusConditionTrue(again.Status.Conditions, v1.ConditionReady)).To(BeTrue())
	}, time.Minute).Should(Succeed())
}

// outcome is the result of one check of a self-test run.
type outcome struct {
	err     error
	skipped string
}

// runAll runs every registered check against cfg, and returns the outcome of
// each, so a spec can assert on each check.
func runAll(cfg Config) map[CheckID]outcome {
	steps, err := plan(cfg, Checks())
	Expect(err).NotTo(HaveOccurred())

	s := newSuite(cfg, restCfg, k8sClient)
	got := make(map[CheckID]outcome, len(steps))
	for _, st := range steps {
		if st.skipReason != "" {
			got[st.check.ID] = outcome{skipped: st.skipReason}
			continue
		}

		err := st.check.run(ctx, s)
		if reason, ok := skipReasonOf(err); ok {
			got[st.check.ID] = outcome{skipped: reason}
			continue
		}
		got[st.check.ID] = outcome{err: err}
	}

	Expect(s.close(ctx)).To(Succeed())

	return got
}

// skippedIDs returns the checks of got that were skipped.
func skippedIDs(got map[CheckID]outcome) []CheckID {
	var ids []CheckID
	for id, o := range got {
		if o.skipped != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

// selfTestTimeout is each timeout of a self-test run. A defect spec waits
// for it once or twice.
const selfTestTimeout = 20 * time.Second

// echoRun starts an echo controller of the echo section with defect, and
// returns the config of a run against it.
func echoRun(defect echo.Defect) Config {
	return pluginRun(echo.Options{
		Sections: []string{"echo"},
		Inputs:   []pluginsv1alpha1.InputDecl{{Name: "source", From: "echo-source/value"}},
		Outputs:  []echo.Output{{Key: "message", FromInput: "source"}},
		Defect:   defect,
	})
}

// accessRun starts an echo controller of the access section with defect,
// and returns the config of a run against it.
func accessRun(defect echo.Defect) Config {
	return pluginRun(echo.Options{
		Sections: []string{"access"},
		Outputs: []echo.Output{
			{Key: "externalUrl", Value: "https://camunda.example.com"},
			{Key: "grpcHost", Value: "grpc.camunda.example.com"},
			{Key: "optimizeUrl", Value: "https://camunda.example.com/optimize"},
		},
		Defect: defect,
	})
}

func pluginRun(opts echo.Options) Config {
	name := fmt.Sprintf("echo-%d", pluginSeq.Add(1))
	opts.Registration = name
	opts.ServiceAccount = installPlugin(name)

	startEcho(opts)
	acceptRegistration(name)

	var inputs map[string]string
	if len(opts.Inputs) > 0 {
		inputs = map[string]string{"source": "hello"}
	}

	return Config{
		Registration: name,
		Namespace:    "run-" + name,
		Cases: []Case{{
			Name:       "basic",
			Inputs:     inputs,
			Parameters: &apiextensionsv1.JSON{Raw: []byte(`{"message":"hi"}`)},
			Target: pluginsv1alpha1.TargetRef{
				Kind:               "CamundaCluster",
				Name:               "echo-target",
				ServiceAccountName: "echo-target-camunda",
			},
		}},
		Timeouts: Timeouts{OutputsReady: selfTestTimeout, Ready: selfTestTimeout, Deletion: selfTestTimeout},
		Skip: []Skip{
			{ID: ReqLogs, Reason: "envtest runs no pods"},
			{ID: OrderRegistrationFirst, Reason: "envtest runs no pods"},
		},
		RESTConfig: restCfg,
	}
}

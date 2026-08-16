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

// Package testenv boots the envtest control plane that the controller suites
// share. Each controller package owns its own Ginkgo suite. This package owns
// the bootstrap that is common to all suites: CRD loading, scheme
// registration, and a running manager. A suite then only declares which
// reconcilers it exercises.
package testenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/test/utils"
)

// Timeout and Interval bound the Eventually polling of every envtest assertion.
const (
	Timeout  = 10 * time.Second
	Interval = 250 * time.Millisecond
)

// eventBurst and eventQPS size the event spam filter of the manager, so every
// event of a suite reaches the API server.
const (
	eventBurst = 100000
	eventQPS   = 1000
)

// Env is a running envtest control plane with a manager started against it.
// Stop stops the manager and tears the control plane down.
type Env struct {
	// Ctx is the context that every spec must use for API calls. Stop cancels
	// it, so in-flight work unwinds with the suite.
	Ctx context.Context
	// Cfg is the client configuration of the control plane.
	Cfg *rest.Config
	// Client reads and writes against the control plane. It bypasses the cache
	// of the manager.
	Client client.Client

	cancel  context.CancelFunc
	control *envtest.Environment
}

// Start boots a control plane that carries the CRDs of the operator, registers
// the reconcilers of the caller through register, and starts the manager in
// the background. register runs before the manager starts and must not block.
//
// Start asserts through Gomega. Call it from a Ginkgo node that has a fail
// handler installed, normally BeforeSuite.
func Start(register func(mgr ctrl.Manager) error) *Env {
	ginkgo.GinkgoHelper()

	gomega.Expect(v1.AddToScheme(scheme.Scheme)).To(gomega.Succeed())
	gomega.Expect(esv1.AddToScheme(scheme.Scheme)).To(gomega.Succeed())
	gomega.Expect(monitoringv1.AddToScheme(scheme.Scheme)).To(gomega.Succeed())

	root, err := moduleRoot()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// The ECK CRDs come from the resolved module, so the rendered Elasticsearch
	// CR applies against the API server. ECK itself does not run in envtest.
	eckCRDPath, err := utils.ECKCRDPath()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("bootstrapping test environment")
	control := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(root, "config", "crd", "bases"),
			eckCRDPath,
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: firstEnvtestBinaryDir(filepath.Join(root, "bin", "k8s")),
	}

	cfg, err := control.Start()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(cfg).NotTo(gomega.BeNil())

	apiClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(apiClient).NotTo(gomega.BeNil())

	ginkgo.By("starting the manager with the suite's controllers")
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		// The component framework records an event per applied resource on
		// every reconcile. The default spam filter of client-go then drops
		// every later event of the object (25 per object, one more each 5
		// minutes), so a suite could not observe the events of a controller.
		EventBroadcaster: record.NewBroadcasterWithCorrelatorOptions(record.CorrelatorOptions{
			BurstSize: eventBurst,
			QPS:       eventQPS,
		}),
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(register(mgr)).To(gomega.Succeed(), "registering the suite's controllers")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer ginkgo.GinkgoRecover()
		gomega.Expect(mgr.Start(ctx)).To(gomega.Succeed())
	}()

	return &Env{Ctx: ctx, Cfg: cfg, Client: apiClient, cancel: cancel, control: control}
}

// Stop cancels the context of the manager and tears the control plane down.
// It is safe to retry. Callers retry it because the shutdown of envtest is
// sometimes slow to release its ports.
//
// Stop tolerates a nil receiver. AfterSuite still runs when BeforeSuite failed
// and the Env of the suite was never assigned. Without this guard the teardown
// panics and hides the failure that matters.
func (e *Env) Stop() error {
	if e == nil {
		return nil
	}

	e.cancel()
	return e.control.Stop()
}

// moduleRoot walks up from the working directory to the directory that holds
// go.mod. Suites then resolve the same CRD and envtest binary directories at
// any package depth.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found in or above the working directory")
		}
		dir = parent
	}
}

// firstEnvtestBinaryDir returns the first versioned binary directory under
// basePath. Suites can then run from an IDE without KUBEBUILDER_ASSETS set. It
// returns an empty string when the directory is absent, and envtest then falls
// back to KUBEBUILDER_ASSETS. Run 'make setup-envtest' to populate the
// directory.
func firstEnvtestBinaryDir(basePath string) string {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}

	return ""
}

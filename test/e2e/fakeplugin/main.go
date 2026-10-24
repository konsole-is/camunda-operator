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

// Command fakeplugin is the plugin controller of the kind harness of the
// fleet manager. It runs the echo controller of the conformance module for
// one registration that serves the sections access and backup-storage, with
// fixed outputs from flags.
package main

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/conformance/echo"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseOptions(args)
	if err != nil {
		return err
	}

	ctrl.SetLogger(zap.New())

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return fmt.Errorf("adding the client-go types to the scheme: %w", err)
	}

	if err := pluginsv1alpha1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("adding the plugin types to the scheme: %w", err)
	}

	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("reading the cluster config: %w", err)
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: cfg.HealthProbeBindAddress,
	})
	if err != nil {
		return fmt.Errorf("creating the manager: %w", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("adding the health check: %w", err)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("adding the ready check: %w", err)
	}

	controller, err := echo.New(cfg.Echo)
	if err != nil {
		return fmt.Errorf("creating the echo controller: %w", err)
	}

	if err := controller.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setting up the echo controller: %w", err)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("running the manager: %w", err)
	}

	return nil
}

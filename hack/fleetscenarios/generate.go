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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	crenvtest "sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/konsole-is/camunda-operator/internal/manager"
	"github.com/konsole-is/camunda-operator/internal/reconcilers"
	"github.com/konsole-is/camunda-operator/pkg/grace"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

// The settings of the manager that a scenario runs, and the field managers
// of the writes that do not come from a reconciler or the simulator.
const (
	operatorNamespace = "camunda-system"
	cliImage          = "ghcr.io/konsole-is/camunda-operator-cli:scenario"
	seedManager       = "kubectl"
	overlayManager    = "fleetscenarios-overlay"
	// deployedManager is the field manager of a write of the operator that
	// names none. The API server takes it from the user agent, which starts
	// with the name of the binary, and the image of the operator runs
	// /manager.
	deployedManager = "manager"
)

// objectsFile is the output file of the state after the seed.
const objectsFile = "objects.yaml"

// generate runs sc on a new envtest control plane and returns its outputs,
// keyed by file name: objectsFile for the state after the seed, and
// <state>.yaml for each named state. It returns an error when a settle
// times out, or when a workload step, an overlay, or a deletion names an
// object that does not exist.
func generate(ctx context.Context, sc *scenario) (_ map[string][]unstructured.Unstructured, err error) {
	plane, err := envtest.ControlPlane(envtest.Options{})
	if err != nil {
		return nil, fmt.Errorf("building the control plane: %w", err)
	}
	cfg, err := plane.Start()
	if err != nil {
		return nil, fmt.Errorf("starting the control plane: %w", err)
	}
	defer func() {
		err = errors.Join(err, stopPlane(plane))
	}()

	scheme := runtime.NewScheme()
	if err := manager.AddToScheme(scheme); err != nil {
		return nil, err
	}
	// The simulator reads and writes many objects on each tick of a settle.
	// The default client rate of 5 per second would slow each tick down.
	fast := rest.CopyConfig(cfg)
	fast.QPS, fast.Burst = 1000, 2000
	c, err := client.New(fast, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("creating the client: %w", err)
	}
	sco, err := newScope(ctx, cfg)
	if err != nil {
		return nil, err
	}
	sim := newSimulator(c)

	namespace := &unstructured.Unstructured{}
	namespace.SetAPIVersion("v1")
	namespace.SetKind(kindNamespace)
	namespace.SetName(operatorNamespace)
	seed, err := toUnstructured(sc.Seed)
	if err != nil {
		return nil, fmt.Errorf("reading the seed: %w", err)
	}
	if err := applyObjects(ctx, c, sco, sim, append([]unstructured.Unstructured{*namespace}, seed...)); err != nil {
		return nil, fmt.Errorf("applying the seed: %w", err)
	}
	sim.add(sc.Workload)

	outputs := map[string][]unstructured.Unstructured{}
	overlays := slices.Clone(sc.Overlays)
	ips := newServiceIPs()
	objs, err := settleAndList(ctx, cfg, scheme, c, sim, sco, ips, overlays)
	if err != nil {
		return nil, err
	}
	outputs[objectsFile] = objs

	for _, st := range sc.States {
		objs, err := runState(ctx, cfg, scheme, c, sim, sco, ips, st, &overlays)
		if err != nil {
			return nil, fmt.Errorf("state %q: %w", st.Name, err)
		}
		outputs[st.Name+".yaml"] = objs
	}

	return outputs, nil
}

// stopPlane stops plane, and tries again while it fails, because the
// shutdown of envtest is sometimes slow to release its ports. It returns the
// last error when every try failed.
func stopPlane(plane *crenvtest.Environment) error {
	var err error
	for range 5 {
		if err = plane.Stop(); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("stopping the control plane: %w", err)
}

// toUnstructured returns objs as unstructured objects, with each JSON
// number that is a whole number as an int64, as the API server returns it.
func toUnstructured(objs []map[string]any) ([]unstructured.Unstructured, error) {
	out := make([]unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		var u unstructured.Unstructured
		if err := u.UnmarshalJSON(data); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

// applyObjects applies objs with server-side apply as seedManager, the
// Namespaces first and then the rest in their order, and adds each
// Namespace to sco and sim.
func applyObjects(
	ctx context.Context,
	c client.Client,
	sco *scope,
	sim *simulator,
	objs []unstructured.Unstructured,
) error {
	ordered := slices.Clone(objs)
	slices.SortStableFunc(ordered, func(a, b unstructured.Unstructured) int {
		aNamespace, bNamespace := a.GetKind() == kindNamespace, b.GetKind() == kindNamespace
		switch {
		case aNamespace && !bNamespace:
			return -1
		case bNamespace && !aNamespace:
			return 1
		}
		return 0
	})

	for i := range ordered {
		obj := &ordered[i]
		err := c.Apply(
			ctx,
			client.ApplyConfigurationFromUnstructured(obj),
			client.FieldOwner(seedManager),
			client.ForceOwnership,
		)
		if err != nil {
			return fmt.Errorf("applying %s: %w", describe(obj), err)
		}
		if obj.GetKind() == kindNamespace {
			sco.add(obj.GetName())
			sim.watch(obj.GetName())
		}
	}
	return nil
}

// settleAndList runs the reconcilers until the scenario settles, applies
// overlays after they stop, and returns the normalized objects of the scope.
func settleAndList(
	ctx context.Context,
	cfg *rest.Config,
	scheme *runtime.Scheme,
	c client.Client,
	sim *simulator,
	sco *scope,
	ips *serviceIPs,
	overlays []overlay,
) ([]unstructured.Unstructured, error) {
	before, _, err := controllerActivity()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(settleTimeout)
	err = runReconcilers(ctx, cfg, scheme, deadline, func(ctx context.Context) error {
		return settle(ctx, sim, sco, time.Until(deadline), before)
	})
	if err != nil {
		return nil, err
	}
	if err := sim.missingSteps(ctx); err != nil {
		return nil, err
	}
	if err := applyOverlays(ctx, c, overlays); err != nil {
		return nil, err
	}

	objs, err := sco.list(ctx)
	if err != nil {
		return nil, err
	}
	return normalize(ips, objs)
}

// runReconcilers runs every reconciler of the operator against cfg while
// during runs, then stops them and waits for the manager to return. during
// starts once a reconciler has finished a reconcile. runReconcilers fails
// when no reconciler finishes one before deadline, or when the manager stops
// first. The manager skips the check of unique controller names,
// because one process starts the same controllers again for each state and
// each scenario.
func runReconcilers(
	ctx context.Context,
	cfg *rest.Config,
	scheme *runtime.Scheme,
	deadline time.Time,
	during func(context.Context) error,
) error {
	managerCfg := rest.CopyConfig(cfg)
	managerCfg.UserAgent = deployedManager
	mgr, err := ctrl.NewManager(managerCfg, ctrl.Options{
		Scheme:                 scheme,
		Cache:                  manager.CacheOptions(),
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Controller:             config.Controller{SkipNameValidation: new(true)},
	})
	if err != nil {
		return fmt.Errorf("creating the manager: %w", err)
	}

	// The grace periods are the defaults of the manager flags, so a scenario
	// reports what a cluster with the default chart reports.
	if err := reconcilers.Setup(mgr, reconcilers.Options{
		Namespace:    operatorNamespace,
		CLIImage:     cliImage,
		GracePeriods: grace.Periods{Workload: grace.DefaultWorkload, Datastore: grace.DefaultDatastore},
	}); err != nil {
		return err
	}

	before, _, err := controllerActivity()
	if err != nil {
		return err
	}

	mgrCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		done <- mgr.Start(mgrCtx)
		close(stopped)
	}()

	err = waitForReconcile(ctx, before, deadline, stopped)
	if err == nil {
		err = during(ctx)
	}
	cancel()

	return errors.Join(err, <-done)
}

// waitForReconcile returns nil once a controller has finished a reconcile
// since the counts before were read. A settle that starts before the
// controllers could find a scenario quiet that they have not seen yet.
func waitForReconcile(
	ctx context.Context,
	before map[string]float64,
	deadline time.Time,
	stopped <-chan struct{},
) error {
	for {
		reconciles, _, err := controllerActivity()
		if err != nil {
			return err
		}
		if len(rose(before, reconciles)) > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no reconciler finished a reconcile within %s", settleTimeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-stopped:
			return errors.New("the manager stopped before a reconciler finished a reconcile")
		case <-time.After(settleTick):
		}
	}
}

// applyOverlays writes the status of each overlay as a JSON merge patch on
// the status subresource, as overlayManager. A list in the patch replaces
// the whole list. An overlay that names an object that does not exist, or a
// field that its schema does not hold, is an error.
func applyOverlays(ctx context.Context, c client.Client, overlays []overlay) error {
	for _, o := range overlays {
		obj := refObject(o.Object)
		patch, err := json.Marshal(map[string]any{"status": o.Status})
		if err != nil {
			return fmt.Errorf("encoding the overlay of %s: %w", describe(obj), err)
		}

		err = c.Status().Patch(
			ctx,
			obj,
			client.RawPatch(types.MergePatchType, patch),
			client.FieldOwner(overlayManager),
			client.FieldValidation(metav1.FieldValidationStrict),
		)
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("overlay names %s, which does not exist", describe(obj))
		}
		if err != nil {
			return fmt.Errorf("applying the overlay of %s: %w", describe(obj), err)
		}
	}
	return nil
}

// refObject returns an unstructured object with the type and the name of
// ref.
func refObject(ref objectRef) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	obj.SetNamespace(ref.Namespace)
	obj.SetName(ref.Name)
	return obj
}

// runState applies the changes of st, runs the reconcilers until the
// scenario settles, and returns the normalized objects of the scope. It
// appends the overlays of st to overlays. It fails when an object that st
// deletes still exists after the settle.
func runState(
	ctx context.Context,
	cfg *rest.Config,
	scheme *runtime.Scheme,
	c client.Client,
	sim *simulator,
	sco *scope,
	ips *serviceIPs,
	st state,
	overlays *[]overlay,
) ([]unstructured.Unstructured, error) {
	apply, err := toUnstructured(st.Apply)
	if err != nil {
		return nil, fmt.Errorf("reading the objects to apply: %w", err)
	}
	if err := applyObjects(ctx, c, sco, sim, apply); err != nil {
		return nil, err
	}

	for _, ref := range st.Delete {
		obj := refObject(ref)
		err := c.Delete(ctx, obj)
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("delete names %s, which does not exist", describe(obj))
		}
		if err != nil {
			return nil, fmt.Errorf("deleting %s: %w", describe(obj), err)
		}
	}

	sim.add(st.Workload)
	*overlays = append(*overlays, st.Overlays...)

	objs, err := settleAndList(ctx, cfg, scheme, c, sim, sco, ips, *overlays)
	if err != nil {
		return nil, err
	}

	// envtest runs no controller that removes the finalizers of Kubernetes,
	// such as the one on each Namespace, so a deleted object can stay.
	for _, ref := range st.Delete {
		obj := refObject(ref)
		err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err == nil {
			return nil, fmt.Errorf(
				"delete names %s, which still exists after the settle, with the finalizers %v",
				describe(obj), obj.GetFinalizers(),
			)
		}
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("reading %s: %w", describe(obj), err)
		}
	}

	return objs, nil
}

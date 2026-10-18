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
	"errors"
	"fmt"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// The IDs of the ordering and deletion checks.
const (
	CheckOrderRegistrationFirst CheckID = "ORDER-REGISTRATION-FIRST"
	CheckDeleteDelete           CheckID = "DELETE-DELETE"
	CheckDeleteOrphan           CheckID = "DELETE-ORPHAN"
	CheckDeleteRoutes           CheckID = "DELETE-ROUTES"
)

// sectionAccess is the core section of the external access of a cluster. The
// module cannot import the startup writer of the operator, which declares it.
const sectionAccess = "access"

// routeKinds are the kinds that DELETE-ROUTES checks.
var routeKinds = []schema.GroupKind{
	{Group: "networking.k8s.io", Kind: "Ingress"},
	{Group: "gateway.networking.k8s.io", Kind: "HTTPRoute"},
	{Group: "gateway.networking.k8s.io", Kind: "GRPCRoute"},
	{Group: "gateway.networking.k8s.io", Kind: "TLSRoute"},
	{Group: "gateway.networking.k8s.io", Kind: "TCPRoute"},
}

// The keys of the order recorder.
const (
	registrationReadyKey = "registration-ready"
	requestServedKey     = "request-served"
)

// deletionChecks run after the lifecycle checks, in this order. The deletion
// checks delete the requests of the run.
var deletionChecks = []Check{
	{
		ID: CheckOrderRegistrationFirst,
		Description: "A request that is created while the plugin is down gets no finalizer and no output " +
			"before the plugin wrote its registration again with Ready=True.",
		Skippable: true,
		run:       checkRegistrationFirst,
	},
	{
		ID: CheckDeleteDelete,
		Description: "With the deletion policy Delete, the request is gone within the Deletion timeout, " +
			"and so is each of its objects.",
		run: checkDeleteDelete,
	},
	{
		ID: CheckDeleteOrphan,
		Description: "With the deletion policy Orphan, each row with onOrphan Kept stays with both writer " +
			"labels and no owner reference to the request, and each row with onOrphan Deleted is gone.",
		run: checkDeleteOrphan,
	},
	{
		ID:          CheckDeleteRoutes,
		Description: "For the access section, a route to a Service that is gone is deleted under both deletion policies.",
		run:         checkDeleteRoutes,
	},
}

func checkRegistrationFirst(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	sa := life.registration.Spec.ServiceAccount
	deployments, err := pluginDeployments(ctx, s, sa)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return SkipCheck(fmt.Sprintf("no Deployment in %s runs under the ServiceAccount %s", sa.Namespace, sa.Name))
	}

	s.onClose(func(ctx context.Context) error {
		return scaleBack(ctx, s, deployments)
	})

	if err := scaleToZero(ctx, s, deployments); err != nil {
		return err
	}

	if err := deleteRegistration(ctx, s, life.registration.Name); err != nil {
		return err
	}

	run := requestRun{
		caseName: life.runs[0].caseName,
		policy:   pluginsv1alpha1.DeletionPolicyDelete,
		name:     strings.ToLower(life.runs[0].caseName) + "-order",
		tc:       life.runs[0].tc,
	}

	rec, err := startOrderRecorder(ctx, s, life.registration.Name, run.name)
	if err != nil {
		return err
	}
	defer rec.stop()

	req := requestFor(life.registration.Name, life.section, s.cfg.Namespace, run)
	if err := s.client.Create(ctx, req); err != nil {
		return fmt.Errorf("creating the PluginRequest %s/%s: %w", req.Namespace, req.Name, err)
	}
	s.onClose(func(ctx context.Context) error {
		return s.deleteRequests(ctx, []requestRun{run})
	})

	if err := scaleBack(ctx, s, deployments); err != nil {
		return err
	}

	if _, err := waitForOutputsReady(ctx, s, run.name, true); err != nil {
		return fmt.Errorf("request %s: %w", run.name, err)
	}

	verdict := orderVerdict(rec, life.registration.Name)

	return errors.Join(verdict, deleteRequest(ctx, s, run.name))
}

// scaledDeployment is a plugin Deployment with the replica count that it had
// before the check scaled it to zero.
type scaledDeployment struct {
	key      client.ObjectKey
	replicas int32
}

// pluginDeployments returns the Deployments of the namespace of sa whose pods
// run under sa.
func pluginDeployments(
	ctx context.Context,
	s *suite,
	sa pluginsv1alpha1.ServiceAccountRef,
) ([]scaledDeployment, error) {
	var list appsv1.DeploymentList
	if err := s.client.List(ctx, &list, client.InNamespace(sa.Namespace)); err != nil {
		return nil, fmt.Errorf("listing the Deployments of %s: %w", sa.Namespace, err)
	}

	var deployments []scaledDeployment
	for _, d := range list.Items {
		if d.Spec.Template.Spec.ServiceAccountName != sa.Name {
			continue
		}

		replicas := int32(1)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		deployments = append(deployments, scaledDeployment{key: client.ObjectKeyFromObject(&d), replicas: replicas})
	}

	return deployments, nil
}

func scaleToZero(ctx context.Context, s *suite, deployments []scaledDeployment) error {
	for _, d := range deployments {
		if err := scale(ctx, s, d.key, 0); err != nil {
			return err
		}
	}

	return waitFor(ctx, s.cfg.Timeouts.Ready, "the plugin Deployments to run no pod", func(ctx context.Context) error {
		for _, d := range deployments {
			var current appsv1.Deployment
			if err := s.client.Get(ctx, d.key, &current); err != nil {
				return fmt.Errorf("getting the Deployment %s: %w", d.key, err)
			}

			if current.Status.Replicas != 0 {
				return fmt.Errorf("the Deployment %s still has %d pods", d.key, current.Status.Replicas)
			}
		}

		return nil
	})
}

func scaleBack(ctx context.Context, s *suite, deployments []scaledDeployment) error {
	var errs []error
	for _, d := range deployments {
		errs = append(errs, scale(ctx, s, d.key, d.replicas))
	}

	return errors.Join(errs...)
}

func scale(ctx context.Context, s *suite, key client.ObjectKey, replicas int32) error {
	d := &appsv1.Deployment{}
	d.Namespace = key.Namespace
	d.Name = key.Name

	patch := client.RawPatch(types.MergePatchType, fmt.Appendf(nil, `{"spec":{"replicas":%d}}`, replicas))
	if err := s.client.Patch(ctx, d, patch); err != nil {
		return fmt.Errorf("scaling the Deployment %s to %d: %w", key, replicas, err)
	}

	return nil
}

func deleteRegistration(ctx context.Context, s *suite, name string) error {
	reg := &pluginsv1alpha1.PluginRegistration{}
	reg.Name = name
	if err := client.IgnoreNotFound(s.client.Delete(ctx, reg)); err != nil {
		return fmt.Errorf("deleting the PluginRegistration %s: %w", name, err)
	}

	return waitFor(ctx, s.cfg.Timeouts.Deletion, "the PluginRegistration to be gone", func(ctx context.Context) error {
		err := s.client.Get(ctx, client.ObjectKey{Name: name}, reg)
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("the PluginRegistration %s still exists", name)
	})
}

// startOrderRecorder records when the registration name has Ready=True, and
// when the request gets the cleanup finalizer or an output.
func startOrderRecorder(ctx context.Context, s *suite, registration, request string) (*recorder, error) {
	c, err := client.NewWithWatch(s.restConfig, client.Options{Scheme: s.client.Scheme(), Mapper: s.client.RESTMapper()})
	if err != nil {
		return nil, fmt.Errorf("connecting the watches of the check: %w", err)
	}

	watchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	rec := &recorder{stop: cancel}

	err = rec.watch(watchCtx, c, &pluginsv1alpha1.PluginRegistrationList{}, func(obj client.Object) string {
		reg, ok := obj.(*pluginsv1alpha1.PluginRegistration)
		if ok && meta.IsStatusConditionTrue(reg.Status.Conditions, v1.ConditionReady) {
			return registrationReadyKey
		}
		return ""
	}, client.MatchingFields{"metadata.name": registration})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watching the PluginRegistration %s: %w", registration, err)
	}

	err = rec.watch(watchCtx, c, &pluginsv1alpha1.PluginRequestList{}, func(obj client.Object) string {
		req, ok := obj.(*pluginsv1alpha1.PluginRequest)
		if ok && (controllerutil.ContainsFinalizer(req, pluginsv1alpha1.FinalizerCleanup) || len(req.Status.Outputs) > 0) {
			return requestServedKey
		}
		return ""
	}, client.InNamespace(s.cfg.Namespace), client.MatchingFields{"metadata.name": request})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watching the PluginRequest %s: %w", request, err)
	}

	return rec, nil
}

// orderVerdict passes when the recorder saw the registration with Ready=True
// before the request got the finalizer or an output.
func orderVerdict(r *recorder, registration string) error {
	ready, ok := r.first(registrationReadyKey)
	if !ok {
		return fmt.Errorf("the plugin did not write the registration %s again with Ready=True", registration)
	}

	if served, ok := r.first(requestServedKey); ok && served.before(ready) {
		return errors.New("the plugin served the request before it wrote the registration with Ready=True: " +
			"the request got the finalizer " + pluginsv1alpha1.FinalizerCleanup + " or an output first")
	}

	return nil
}

func checkDeleteDelete(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	runs := runsWith(life.runs, pluginsv1alpha1.DeletionPolicyDelete)
	if len(runs) == 0 {
		return SkipCheck("no case of the run uses the deletion policy Delete")
	}

	return eachRun(runs, func(r requestRun) error {
		objs, err := writtenObjects(ctx, s, life.kinds, r.name, life.preexisting)
		if err != nil {
			return err
		}
		life.written[r.name] = objs

		if err := deleteRequest(ctx, s, r.name); err != nil {
			return err
		}

		return waitFor(ctx, s.cfg.Timeouts.Deletion, "the objects of the request to be gone", func(ctx context.Context) error {
			var problems []string
			for _, obj := range objs {
				current, err := s.current(ctx, obj)
				if err != nil {
					return err
				}
				if current != nil {
					problems = append(problems, objectRef(obj)+" still exists")
				}
			}

			return problemsError(problems)
		})
	})
}

func checkDeleteOrphan(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	runs := runsWith(life.runs, pluginsv1alpha1.DeletionPolicyOrphan)
	switch {
	case len(runs) > 0:
	case !slices.Contains(life.registration.Spec.DeletionPolicies, pluginsv1alpha1.DeletionPolicyOrphan):
		return SkipCheck("the registration does not support the deletion policy Orphan")
	default:
		return SkipCheck("no case of the run uses the deletion policy Orphan")
	}

	return eachRun(runs, func(r requestRun) error {
		req, err := s.request(ctx, r.name)
		if err != nil {
			return err
		}

		objs, err := writtenObjects(ctx, s, life.kinds, r.name, life.preexisting)
		if err != nil {
			return err
		}
		life.written[r.name] = objs

		if err := deleteRequest(ctx, s, r.name); err != nil {
			return err
		}

		what := "the objects of the request to follow onOrphan"
		return waitFor(ctx, s.cfg.Timeouts.Deletion, what, func(ctx context.Context) error {
			var problems []string
			for _, row := range req.Status.Resources {
				i := slices.IndexFunc(objs, func(o *unstructured.Unstructured) bool {
					return o.GetKind() == row.Kind && o.GetName() == row.Name
				})
				if i == -1 {
					continue
				}

				current, err := s.current(ctx, objs[i])
				if err != nil {
					return err
				}
				problems = append(problems, orphanProblems(row, objs[i], current, req)...)
			}

			return problemsError(problems)
		})
	})
}

// orphanProblems names how the object before, now current or nil when it is
// gone, breaks the onOrphan of its row after an Orphan deletion of request.
// A kept cluster-scoped object is checked by its labels only.
func orphanProblems(
	row pluginsv1alpha1.ResourceRef,
	before, current *unstructured.Unstructured,
	request *pluginsv1alpha1.PluginRequest,
) []string {
	ref := objectRef(before)

	if row.OnOrphan == pluginsv1alpha1.OnOrphanDeleted {
		if current != nil {
			return []string{ref + " has onOrphan Deleted and still exists"}
		}
		return nil
	}

	if current == nil {
		return []string{ref + " has onOrphan Kept and is gone"}
	}

	var problems []string
	for _, label := range []string{pluginsv1alpha1.LabelRegistration, pluginsv1alpha1.LabelRequest} {
		if current.GetLabels()[label] != before.GetLabels()[label] || current.GetLabels()[label] == "" {
			problems = append(problems, ref+" lost the label "+label)
		}
	}

	owned := slices.ContainsFunc(current.GetOwnerReferences(), func(o metav1.OwnerReference) bool {
		return o.UID == request.UID
	})
	if before.GetNamespace() != "" && owned {
		problems = append(problems, ref+" still has an owner reference to the request")
	}

	return problems
}

func checkDeleteRoutes(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(life.registration.Spec.Sections, sectionAccess) {
		return SkipCheck("the registration does not serve the access section")
	}

	return eachRun(life.runs, func(r requestRun) error {
		var errs []error
		for _, obj := range life.written[r.name] {
			if !isRoute(obj) {
				continue
			}

			missing, err := missingBackends(ctx, s, obj)
			if err != nil {
				return err
			}
			if len(missing) == 0 {
				continue
			}

			errs = append(errs, waitFor(ctx, s.cfg.Timeouts.Deletion, "the route to be gone", func(ctx context.Context) error {
				current, err := s.current(ctx, obj)
				if err != nil {
					return err
				}
				if current != nil {
					return fmt.Errorf("%s routes to the missing Service %s and still exists", objectRef(obj), missing[0])
				}
				return nil
			}))
		}

		return errors.Join(errs...)
	})
}

func isRoute(obj *unstructured.Unstructured) bool {
	return slices.Contains(routeKinds, obj.GroupVersionKind().GroupKind())
}

// missingBackends returns the backend Services of route when none of them
// exists, and nil when one exists or the route has none.
func missingBackends(ctx context.Context, s *suite, route *unstructured.Unstructured) ([]types.NamespacedName, error) {
	backends := routeBackends(route)
	for _, b := range backends {
		var svc corev1.Service
		err := s.client.Get(ctx, b, &svc)
		if err == nil {
			return nil, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("getting the Service %s: %w", b, err)
		}
	}

	return backends, nil
}

// routeBackends returns the Services that route sends traffic to, each one
// time. A Gateway API backend of another kind than Service is left out.
func routeBackends(route *unstructured.Unstructured) []types.NamespacedName {
	var backends []types.NamespacedName
	add := func(namespace, name string) {
		if namespace == "" {
			namespace = route.GetNamespace()
		}

		b := types.NamespacedName{Namespace: namespace, Name: name}
		if name != "" && !slices.Contains(backends, b) {
			backends = append(backends, b)
		}
	}

	rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")

	if route.GetKind() == "Ingress" {
		name, _, _ := unstructured.NestedString(route.Object, "spec", "defaultBackend", "service", "name")
		add("", name)

		for _, rule := range rules {
			paths, _, _ := unstructured.NestedSlice(asMap(rule), "http", "paths")
			for _, p := range paths {
				name, _, _ := unstructured.NestedString(asMap(p), "backend", "service", "name")
				add("", name)
			}
		}

		return backends
	}

	for _, rule := range rules {
		refs, _, _ := unstructured.NestedSlice(asMap(rule), "backendRefs")
		for _, ref := range refs {
			m := asMap(ref)
			group, _, _ := unstructured.NestedString(m, "group")
			kind, _, _ := unstructured.NestedString(m, "kind")
			if group != "" || (kind != "" && kind != "Service") {
				continue
			}

			namespace, _, _ := unstructured.NestedString(m, "namespace")
			name, _, _ := unstructured.NestedString(m, "name")
			add(namespace, name)
		}
	}

	return backends
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func runsWith(runs []requestRun, policy pluginsv1alpha1.DeletionPolicy) []requestRun {
	return slices.DeleteFunc(slices.Clone(runs), func(r requestRun) bool { return r.policy != policy })
}

// deleteRequest deletes the request name and waits until it is gone.
func deleteRequest(ctx context.Context, s *suite, name string) error {
	req, err := s.request(ctx, name)
	if err != nil {
		return err
	}

	if err := client.IgnoreNotFound(s.client.Delete(ctx, req)); err != nil {
		return fmt.Errorf("deleting the PluginRequest %s/%s: %w", req.Namespace, req.Name, err)
	}

	return waitFor(ctx, s.cfg.Timeouts.Deletion, "the request to be gone", func(ctx context.Context) error {
		err := s.client.Get(ctx, client.ObjectKeyFromObject(req), req)
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("the request still exists, with the finalizers %v", req.Finalizers)
	})
}

// current gets obj again, and returns nil when it is gone.
func (s *suite) current(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(obj.GroupVersionKind())

	err := s.client.Get(ctx, client.ObjectKeyFromObject(obj), got)
	switch {
	case apierrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("getting %s: %w", objectRef(obj), err)
	}

	return got, nil
}

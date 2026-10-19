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
	"time"

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

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The IDs of the ordering and deletion checks, which run after the
// lifecycle checks and delete the requests of the run.
const (
	// OrderRegistrationFirst checks that the plugin writes its registration
	// before it serves a request. It is skippable.
	OrderRegistrationFirst CheckID = "ORDER-REGISTRATION-FIRST"
	// DeleteDelete checks the deletion policy Delete.
	DeleteDelete CheckID = "DELETE-DELETE"
	// DeleteOrphan checks the deletion policy Orphan.
	DeleteOrphan CheckID = "DELETE-ORPHAN"
	// DeleteRoutes checks that a route to a missing Service is deleted.
	DeleteRoutes CheckID = "DELETE-ROUTES"
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
		ID: OrderRegistrationFirst,
		Description: "A request that is created while the plugin is down gets no finalizer and no output " +
			"before the plugin wrote its registration again with Ready=True.",
		Skippable: true,
		run:       checkRegistrationFirst,
	},
	{
		ID: DeleteDelete,
		Description: "With the deletion policy Delete, the request is gone within the Deletion timeout, " +
			"and so is each of its objects.",
		run: checkDeleteDelete,
	},
	{
		ID: DeleteOrphan,
		Description: "With the deletion policy Orphan, each row with onOrphan Kept stays with both writer " +
			"labels and no owner reference to the request, and each row with onOrphan Deleted is gone.",
		run: checkDeleteOrphan,
	},
	{
		ID:          DeleteRoutes,
		Description: "For the access section, a route to a Service that is gone is deleted under both deletion policies.",
		run:         checkDeleteRoutes,
	},
}

func checkRegistrationFirst(ctx context.Context, s *suite) (err error) {
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

	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), restoreTimeout)
		defer cancel()
		err = errors.Join(err, scaleBack(restoreCtx, s, deployments))
	}()

	if err := scaleToZero(ctx, s, sa, deployments); err != nil {
		return err
	}

	if err := deleteRegistration(ctx, s, life.registration.Name); err != nil {
		return err
	}

	run := orderRun(life.runs[0], runIDOf(s.cfg.Namespace))

	rec, err := startOrderRecorder(ctx, s, life.registration.Name, run.name)
	if err != nil {
		return err
	}
	defer rec.stop()

	req := requestFor(life.registration.Name, life.sections[run.section], s.cfg.Namespace, run)
	if err := s.client.Create(ctx, req); err != nil {
		return fmt.Errorf("creating the PluginRequest %s/%s: %w", req.Namespace, req.Name, err)
	}
	s.onClose(func(ctx context.Context) error {
		return s.deleteCreated(ctx, []*pluginsv1alpha1.PluginRequest{req})
	})

	if err := scaleBack(ctx, s, deployments); err != nil {
		return err
	}
	scaledBack := time.Now()

	if err := waitForOutputsReady(ctx, s, run.name, scaledBack, true); err != nil {
		return fmt.Errorf("request %s: %w", run.name, err)
	}

	what := "the watches of the check to see the registration and the request"
	verdict := waitFor(ctx, s.cfg.Timeouts.OutputsReady, what, func(context.Context) error {
		return orderVerdict(rec, life.registration.Name)
	})

	return errors.Join(verdict, deleteRequest(ctx, s, run.name, req.UID, time.Now().Add(s.cfg.Timeouts.Deletion)))
}

// orderRun is the run of the request of the check, made from first, with a
// target of its own.
func orderRun(first requestRun, runID string) requestRun {
	run := first
	run.policy = pluginsv1alpha1.DeletionPolicyDelete
	run.name = requestName(first.caseName, "", "order", runID)
	run.tc.Target = targetWithSuffix(first.tc.Target, "order")

	return run
}

// restoreTimeout bounds the scale back of the plugin, which also runs when
// the context of the check ended.
const restoreTimeout = 30 * time.Second

// scaledDeployment is a plugin Deployment with the replica count that it had
// before the check scaled it to zero.
type scaledDeployment struct {
	key      client.ObjectKey
	replicas int32
}

// pluginDeployments returns the Deployments of the namespace of sa whose pods
// run under sa, and fails for one with no replica.
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
		account := d.Spec.Template.Spec.ServiceAccountName
		if account == "" {
			account = "default"
		}
		if account != sa.Name {
			continue
		}

		replicas := int32(1)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		if replicas == 0 {
			return nil, fmt.Errorf(
				"the Deployment %s/%s runs no replica: another run of the suite can hold it, so run one at a time",
				d.Namespace,
				d.Name,
			)
		}
		deployments = append(deployments, scaledDeployment{key: client.ObjectKeyFromObject(&d), replicas: replicas})
	}

	return deployments, nil
}

// scaleToZero scales deployments to zero and waits until no pod runs under
// sa, terminating pods too.
func scaleToZero(
	ctx context.Context,
	s *suite,
	sa pluginsv1alpha1.ServiceAccountRef,
	deployments []scaledDeployment,
) error {
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

		var pods corev1.PodList
		if err := s.client.List(ctx, &pods, client.InNamespace(sa.Namespace)); err != nil {
			return fmt.Errorf("listing the pods of %s: %w", sa.Namespace, err)
		}

		for _, p := range pods.Items {
			if p.Spec.ServiceAccountName == sa.Name {
				return fmt.Errorf("the pod %s/%s of the plugin still runs", p.Namespace, p.Name)
			}
		}

		return nil
	})
}

func scaleBack(ctx context.Context, s *suite, deployments []scaledDeployment) error {
	errs := make([]error, 0, len(deployments))
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
	c, err := client.NewWithWatch(
		s.restConfig,
		client.Options{Scheme: s.client.Scheme(), Mapper: s.client.RESTMapper()},
	)
	if err != nil {
		return nil, fmt.Errorf("connecting the watches of the check: %w", err)
	}

	watchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	rec := &recorder{stop: cancel}

	err = rec.watch(
		watchCtx,
		c,
		func() client.ObjectList { return &pluginsv1alpha1.PluginRegistrationList{} },
		func(obj client.Object) string {
			reg, ok := obj.(*pluginsv1alpha1.PluginRegistration)
			if ok && meta.IsStatusConditionTrue(reg.Status.Conditions, v1.ConditionReady) {
				return registrationReadyKey
			}
			return ""
		},
		"",
		client.MatchingFields{"metadata.name": registration},
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watching the PluginRegistration %s: %w", registration, err)
	}

	err = rec.watch(
		watchCtx,
		c,
		func() client.ObjectList { return &pluginsv1alpha1.PluginRequestList{} },
		func(obj client.Object) string {
			req, ok := obj.(*pluginsv1alpha1.PluginRequest)
			if ok &&
				(controllerutil.ContainsFinalizer(req, pluginsv1alpha1.FinalizerCleanup) || len(req.Status.Outputs) > 0) {
				return requestServedKey
			}
			return ""
		},
		"",
		client.InNamespace(s.cfg.Namespace),
		client.MatchingFields{"metadata.name": request},
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watching the PluginRequest %s: %w", request, err)
	}

	return rec, nil
}

// orderVerdict passes when the recorder saw the registration with Ready=True
// before the request got the finalizer or an output.
func orderVerdict(r *recorder, registration string) error {
	if err := r.err(); err != nil {
		return err
	}

	ready, ok := r.first(registrationReadyKey)
	if !ok {
		return fmt.Errorf("the plugin did not write the registration %s again with Ready=True", registration)
	}

	served, ok := r.first(requestServedKey)
	if !ok {
		return errors.New("the watch did not see the request get the finalizer " +
			pluginsv1alpha1.FinalizerCleanup + " or an output")
	}

	if served.before(ready) {
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
		now, err := writtenObjects(ctx, s, life.kinds, r.name)
		if err != nil {
			return err
		}
		objs := life.inventory(r.name, now)
		life.written[r.name] = objs

		deadline := time.Now().Add(s.cfg.Timeouts.Deletion)
		if err := deleteRequest(ctx, s, r.name, life.uids[r.name], deadline); err != nil {
			return err
		}

		return waitUntil(
			ctx,
			deadline,
			s.cfg.Timeouts.Deletion,
			"the objects of the request to be gone",
			func(ctx context.Context) error {
				// An object that the plugin wrote while it deleted the
				// request has the label too.
				later, err := writtenObjects(ctx, s, life.kinds, r.name)
				if err != nil {
					return err
				}

				var problems []string
				for _, obj := range append(slices.Clone(objs), later...) {
					current, err := s.current(ctx, obj)
					if err != nil {
						return err
					}
					if current != nil {
						problems = append(problems, objectRef(obj)+" still exists")
					}
				}

				return problemsError(problems)
			},
		)
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
		if req.UID != life.uids[r.name] {
			return fmt.Errorf(
				"the PluginRequest %s/%s is not the request that the run created",
				req.Namespace,
				req.Name,
			)
		}

		now, err := writtenObjects(ctx, s, life.kinds, r.name)
		if err != nil {
			return err
		}
		// An object that the run saw and that is gone now is still checked
		// against its row.
		objs := life.inventory(r.name, now)
		life.written[r.name] = objs

		deadline := time.Now().Add(s.cfg.Timeouts.Deletion)
		if err := deleteRequest(ctx, s, r.name, life.uids[r.name], deadline); err != nil {
			return err
		}

		what := "the objects of the request to follow onOrphan"
		return waitUntil(ctx, deadline, s.cfg.Timeouts.Deletion, what, func(ctx context.Context) error {
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

	switch {
	case current == nil:
		return []string{ref + " has onOrphan Kept and is gone"}
	case current.GetUID() != before.GetUID():
		return []string{ref + " was deleted and written again"}
	case current.GetDeletionTimestamp() != nil:
		return []string{ref + " is being deleted"}
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

			errs = append(
				errs,
				waitFor(ctx, s.cfg.Timeouts.Deletion, "the route to be gone", func(ctx context.Context) error {
					current, err := s.current(ctx, obj)
					if err != nil {
						return err
					}
					if current != nil {
						return fmt.Errorf(
							"%s routes to the missing Service %s and still exists",
							objectRef(obj),
							missing[0],
						)
					}
					return nil
				}),
			)
		}

		return errors.Join(errs...)
	})
}

func isRoute(obj *unstructured.Unstructured) bool {
	return slices.Contains(routeKinds, obj.GroupVersionKind().GroupKind())
}

// missingBackends returns the backend Services of route that do not exist.
func missingBackends(ctx context.Context, s *suite, route *unstructured.Unstructured) ([]types.NamespacedName, error) {
	var missing []types.NamespacedName
	for _, b := range routeBackends(route) {
		var svc corev1.Service
		err := s.client.Get(ctx, b, &svc)
		switch {
		case apierrors.IsNotFound(err):
			missing = append(missing, b)
		case err != nil:
			return nil, fmt.Errorf("getting the Service %s: %w", b, err)
		}
	}

	return missing, nil
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

// deleteRequest deletes the request name that the run created with uid, and
// waits until it is gone, at most until deadline. It fails, and deletes
// nothing, when another request of that name replaced it.
func deleteRequest(ctx context.Context, s *suite, name string, uid types.UID, deadline time.Time) error {
	req := &pluginsv1alpha1.PluginRequest{}
	req.Namespace = s.cfg.Namespace
	req.Name = name

	err := s.client.Delete(ctx, req, client.Preconditions{UID: &uid})
	switch {
	case apierrors.IsConflict(err):
		return fmt.Errorf("the PluginRequest %s/%s is not the request that the run created", req.Namespace, name)
	case err != nil && !apierrors.IsNotFound(err):
		return fmt.Errorf("deleting the PluginRequest %s/%s: %w", req.Namespace, name, err)
	}

	return waitUntil(ctx, deadline, s.cfg.Timeouts.Deletion, "the request to be gone", func(ctx context.Context) error {
		err := s.client.Get(ctx, client.ObjectKeyFromObject(req), req)
		if apierrors.IsNotFound(err) || (err == nil && req.UID != uid) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("the request still exists, with the finalizers %v", req.Finalizers)
	})
}

// mergeObjects returns the objects of earlier and later, one time for each
// UID, with the copy of later where both have one.
func mergeObjects(earlier, later []*unstructured.Unstructured) []*unstructured.Unstructured {
	merged := slices.Clone(later)
	for _, obj := range earlier {
		if !slices.ContainsFunc(merged, func(o *unstructured.Unstructured) bool { return o.GetUID() == obj.GetUID() }) {
			merged = append(merged, obj)
		}
	}

	return merged
}

// current gets obj again, and returns nil when it is gone, or when another
// object without its request label took its name.
func (s *suite) current(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(obj.GroupVersionKind())

	err := s.client.Get(ctx, client.ObjectKeyFromObject(obj), got)
	switch {
	case apierrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("getting %s: %w", objectRef(obj), err)
	case got.GetUID() != obj.GetUID() &&
		got.GetLabels()[pluginsv1alpha1.LabelRequest] != obj.GetLabels()[pluginsv1alpha1.LabelRequest]:
		return nil, nil
	}

	return got, nil
}

// inventory returns each object of the request name that the watches, the
// lifecycle checks, or now found, with the newest copy of each.
func (life *lifecycle) inventory(name string, now []*unstructured.Unstructured) []*unstructured.Unstructured {
	watched := life.rec.objectsOf(objectKey(name))
	return mergeObjects(mergeObjects(watched, life.seen[name]), now)
}

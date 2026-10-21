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

package echo

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The vocabulary of the controller.
const (
	fieldManager       = "conformance-echo"
	sectionAccess      = "access"
	reasonEchoDefect   = "EchoDefect"
	eventReasonApply   = "Applied"
	eventActionApply   = "Apply"
	storageProvisioner = "conformance.camunda.io/echo"
	docsURL            = "https://example.com/echo"
)

// Controller is one echo controller. It writes its registration when it
// starts and sets Ready=True on it. It serves each request of the
// registration: the cleanup finalizer first, then a ConfigMap
// <request>-echo, a StorageClass <namespace>-<request>-echo, and with the
// access section an Ingress <request>-echo to the Service <target>-echo, and
// a Grant for each grant template whose required inputs have a value, then
// the status of the request. With Delete it deletes these objects. With
// Orphan it keeps the ConfigMap and the Grants without their owner
// reference, keeps the StorageClass, and deletes the Ingress.
type Controller struct {
	opts     Options
	client   client.Client
	recorder events.EventRecorder
	// lateApplied holds the UIDs of the requests whose objects
	// DefectLateFinalizer wrote before the finalizer.
	lateApplied sync.Map
}

// New returns a controller for opts. It refuses options that are not
// complete, and names each field that is wrong.
func New(opts Options) (*Controller, error) {
	if err := opts.validate(); err != nil {
		return nil, fmt.Errorf("echo: %w", err)
	}

	return &Controller{opts: opts}, nil
}

// SetupWithManager adds the controller to mgr. The controllers are named
// echo-<registration> and echo-<registration>-registration, so one manager
// can run one Controller for each registration.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	name := "echo-" + c.opts.Registration
	c.client = mgr.GetClient()
	c.recorder = mgr.GetEventRecorder(name)

	if err := mgr.Add(manager.RunnableFunc(c.publish)); err != nil {
		return fmt.Errorf("adding the registration writer: %w", err)
	}

	isRegistration := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == c.opts.Registration
	})
	err := ctrl.NewControllerManagedBy(mgr).
		Named(name+"-registration").
		For(&pluginsv1alpha1.PluginRegistration{}, builder.WithPredicates(isRegistration)).
		Complete(reconcileFunc(c.reconcileRegistration))
	if err != nil {
		return fmt.Errorf("adding the registration controller: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&pluginsv1alpha1.PluginRequest{}, builder.WithPredicates(predicate.NewPredicateFuncs(c.serves))).
		Complete(reconcileFunc(c.reconcileRequest))
	if err != nil {
		return fmt.Errorf("adding the request controller: %w", err)
	}

	return nil
}

// serves says whether obj is a request of the registration. It reads the
// immutable spec.registration, so a changed label does not strand a request
// that holds the finalizer.
func (c *Controller) serves(obj client.Object) bool {
	req, ok := obj.(*pluginsv1alpha1.PluginRequest)
	return ok && req.Spec.Registration == c.opts.Registration
}

type reconcileFunc func(ctx context.Context, req ctrl.Request) (ctrl.Result, error)

func (f reconcileFunc) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return f(ctx, req)
}

func (c *Controller) reconcileRequest(ctx context.Context, key ctrl.Request) (ctrl.Result, error) {
	var req pluginsv1alpha1.PluginRequest
	if err := c.client.Get(ctx, key.NamespacedName, &req); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The key can name a replacement that another registration serves.
	if !c.serves(&req) {
		return ctrl.Result{}, nil
	}

	if !req.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&req, pluginsv1alpha1.FinalizerCleanup) {
			return ctrl.Result{}, nil
		}

		if err := c.cleanUp(ctx, &req); err != nil {
			return ctrl.Result{}, err
		}

		controllerutil.RemoveFinalizer(&req, pluginsv1alpha1.FinalizerCleanup)
		return requeueOnConflict(c.client.Update(ctx, &req))
	}

	ready, err := c.registrationReady(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	if !controllerutil.ContainsFinalizer(&req, pluginsv1alpha1.FinalizerCleanup) {
		if c.opts.Defect == DefectLateFinalizer {
			if _, applied := c.lateApplied.LoadOrStore(req.UID, true); !applied {
				if err := c.applyObjects(ctx, &req); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
		}

		controllerutil.AddFinalizer(&req, pluginsv1alpha1.FinalizerCleanup)
		return requeueOnConflict(c.client.Update(ctx, &req))
	}

	if err := c.applyObjects(ctx, &req); err != nil {
		return ctrl.Result{}, err
	}

	return c.writeStatus(ctx, &req)
}

// registrationReady says whether the registration has Ready=True, so a
// request is served only after the registration, as the contract orders.
func (c *Controller) registrationReady(ctx context.Context) (bool, error) {
	var reg pluginsv1alpha1.PluginRegistration
	err := c.client.Get(ctx, types.NamespacedName{Name: c.opts.Registration}, &reg)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return meta.IsStatusConditionTrue(reg.Status.Conditions, v1.ConditionReady), nil
}

// cleanUp applies the deletion policy of req to its objects.
func (c *Controller) cleanUp(ctx context.Context, req *pluginsv1alpha1.PluginRequest) error {
	cm, sc, ing := c.objects(req, true)
	grants, err := c.existingGrants(ctx, req)
	if err != nil {
		return err
	}

	if req.Spec.DeletionPolicy != pluginsv1alpha1.DeletionPolicyOrphan {
		if c.opts.Defect == DefectKeepOnDelete {
			return nil
		}

		objs := []client.Object{cm, sc}
		if ing != nil {
			objs = append(objs, ing)
		}
		for _, g := range grants {
			objs = append(objs, g)
		}
		return c.deleteOwn(ctx, req, objs...)
	}

	if c.opts.Defect != DefectKeepOwnerOnOrphan {
		if err := c.releaseOwner(ctx, cm, req.UID); err != nil {
			return err
		}
	}

	for _, g := range grants {
		if err := c.releaseOwner(ctx, g, req.UID); err != nil {
			return err
		}
	}

	switch {
	case ing == nil:
		return nil
	case c.opts.Defect == DefectKeepRouteOnOrphan:
		return c.releaseOwner(ctx, ing, req.UID)
	default:
		return c.deleteOwn(ctx, req, ing)
	}
}

// releaseOwner removes the owner reference with uid from obj, and keeps the
// rest of obj. It never creates obj: an object that is gone stays gone.
func (c *Controller) releaseOwner(ctx context.Context, obj client.Object, uid types.UID) error {
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return client.IgnoreNotFound(err)
	}

	before, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("copying the %T %s", obj, obj.GetName())
	}

	obj.SetOwnerReferences(slices.DeleteFunc(obj.GetOwnerReferences(), func(o metav1.OwnerReference) bool {
		return o.UID == uid
	}))

	err := c.client.Patch(ctx, obj, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the owner reference of the %T %s: %w", obj, obj.GetName(), err)
	}

	return nil
}

// applyObjects applies the objects of req, and deletes each Grant of req that
// is not one of them. It fails, and writes nothing, when an object of one of
// their names exists without the request label of req.
func (c *Controller) applyObjects(ctx context.Context, req *pluginsv1alpha1.PluginRequest) error {
	grants, err := c.grants(req)
	if err != nil {
		return err
	}

	objs := c.objectList(req, grants)

	for _, obj := range objs {
		current, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("copying the %T %s", obj, obj.GetName())
		}

		err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), current)
		switch {
		case apierrors.IsNotFound(err):
		case err != nil:
			return err
		case current.GetLabels()[pluginsv1alpha1.LabelRequest] != req.Name:
			return fmt.Errorf("the %T %s is not an object of the request", obj, obj.GetName())
		}
	}

	for _, obj := range objs {
		if err := c.apply(ctx, obj); err != nil {
			return err
		}
	}

	existing, err := c.existingGrants(ctx, req)
	if err != nil {
		return err
	}

	var stale []client.Object
	for _, g := range existing {
		if !slices.ContainsFunc(grants, func(want *pluginsv1alpha1.Grant) bool { return want.Name == g.Name }) {
			stale = append(stale, g)
		}
	}

	return c.deleteOwn(ctx, req, stale...)
}

// existingGrants lists the Grants in the namespace of req that carry the
// labels of req.
func (c *Controller) existingGrants(
	ctx context.Context,
	req *pluginsv1alpha1.PluginRequest,
) ([]*pluginsv1alpha1.Grant, error) {
	var list pluginsv1alpha1.GrantList
	err := c.client.List(ctx, &list, client.InNamespace(req.Namespace), client.MatchingLabels{
		pluginsv1alpha1.LabelRegistration: c.opts.Registration,
		pluginsv1alpha1.LabelRequest:      req.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("listing the Grants of the request: %w", err)
	}

	grants := make([]*pluginsv1alpha1.Grant, 0, len(list.Items))
	for i := range list.Items {
		grants = append(grants, &list.Items[i])
	}

	return grants, nil
}

// objectList is the objects of req, with grants, in one list.
func (c *Controller) objectList(req *pluginsv1alpha1.PluginRequest, grants []*pluginsv1alpha1.Grant) []client.Object {
	cm, sc, ing := c.objects(req, false)
	objs := []client.Object{cm, sc}
	if ing != nil {
		objs = append(objs, ing)
	}
	for _, g := range grants {
		objs = append(objs, g)
	}

	return objs
}

// objects returns the ConfigMap, the StorageClass, and the Ingress of req.
// The Ingress is nil when the controller does not serve the access section.
// orphaned leaves out the owner references that an orphaned request no
// longer gives.
func (c *Controller) objects(
	req *pluginsv1alpha1.PluginRequest,
	orphaned bool,
) (*corev1.ConfigMap, *storagev1.StorageClass, *networkingv1.Ingress) {
	labels := map[string]string{
		pluginsv1alpha1.LabelRegistration: c.opts.Registration,
		pluginsv1alpha1.LabelRequest:      req.Name,
	}
	ownerUnless := func(drop bool) []metav1.OwnerReference {
		if drop {
			return nil
		}
		return requestOwner(req)
	}

	data := map[string]string{"message": message(req)}
	maps.Copy(data, c.outputs(req))

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            req.Name + "-echo",
			Namespace:       req.Namespace,
			Labels:          labels,
			OwnerReferences: ownerUnless(c.opts.Defect == DefectNoOwnerReference || orphaned),
		},
		Data: data,
	}

	sc := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            req.Namespace + "-" + req.Name + "-echo",
			Labels:          labels,
			OwnerReferences: ownerUnless(c.opts.Defect != DefectClusterOwnerReference),
		},
		Provisioner: storageProvisioner,
	}

	if !slices.Contains(c.opts.Sections, sectionAccess) {
		return cm, sc, nil
	}

	ing := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            req.Name + "-echo",
			Namespace:       req.Namespace,
			Labels:          labels,
			OwnerReferences: ownerUnless(orphaned),
		},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: req.Spec.Target.Name + "-echo",
					Port: networkingv1.ServiceBackendPort{Number: 80},
				},
			},
		},
	}

	return cm, sc, ing
}

// requestOwner is the owner reference of an object of req.
func requestOwner(req *pluginsv1alpha1.PluginRequest) []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         pluginsv1alpha1.GroupVersion.String(),
		Kind:               "PluginRequest",
		Name:               req.Name,
		UID:                req.UID,
		Controller:         new(true),
		BlockOwnerDeletion: new(true),
	}}
}

// message is the parameter message of req, or empty.
func message(req *pluginsv1alpha1.PluginRequest) string {
	var params struct {
		Message string `json:"message"`
	}
	if req.Spec.Parameters != nil {
		_ = json.Unmarshal(req.Spec.Parameters.Raw, &params)
	}

	return params.Message
}

// outputs are the outputs of req: each copies its input or holds its value.
func (c *Controller) outputs(req *pluginsv1alpha1.PluginRequest) map[string]string {
	declared := slices.DeleteFunc(slices.Clone(c.opts.Outputs), func(o Output) bool {
		return o.Section != "" && o.Section != req.Spec.Section
	})
	if c.opts.Defect == DefectMissingOutput && len(declared) > 0 {
		declared = declared[:len(declared)-1]
	}

	outputs := make(map[string]string, len(declared))
	for _, o := range declared {
		outputs[o.Key] = o.Value
		if o.FromInput != "" {
			outputs[o.Key] = req.Spec.Inputs[o.FromInput]
		}
	}

	return outputs
}

// writeStatus writes the status of req on the copy that it read. It records
// one event for each resource row that the status did not list yet.
func (c *Controller) writeStatus(ctx context.Context, req *pluginsv1alpha1.PluginRequest) (ctrl.Result, error) {
	grants, err := c.grants(req)
	if err != nil {
		return ctrl.Result{}, err
	}

	status := req.Status.DeepCopy()
	status.ObservedGeneration = req.Generation
	status.Outputs = c.outputs(req)

	outputsGeneration := req.Generation
	if c.opts.Defect == DefectStaleOutputsReady {
		outputsGeneration--
	}
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               pluginsv1alpha1.ConditionOutputsReady,
		Status:             metav1.ConditionTrue,
		Reason:             v1.ReasonHealthy,
		Message:            "The echo controller wrote the outputs",
		ObservedGeneration: outputsGeneration,
	})

	ready := metav1.Condition{
		Type:               v1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             v1.ReasonHealthy,
		Message:            "The echo controller serves the request",
		ObservedGeneration: req.Generation,
	}
	if c.opts.Defect == DefectNotReady {
		ready.Status = metav1.ConditionFalse
		ready.Reason = reasonEchoDefect
		ready.Message = "The echo controller reports a defect"
	}
	meta.SetStatusCondition(&status.Conditions, ready)

	summary := []pluginsv1alpha1.SummaryItem{
		{Label: "Message", Value: message(req), Severity: pluginsv1alpha1.SeverityInfo},
	}
	if status.SummaryTime == nil || !equality.Semantic.DeepEqual(status.Summary, summary) {
		status.SummaryTime = new(metav1.Now())
	}
	status.Summary = summary

	link := docsURL
	if c.opts.Defect == DefectRelativeLink {
		link = "/echo"
	}
	status.Links = []pluginsv1alpha1.Link{{Name: "Docs", URL: link}}
	status.Resources = c.resources(req, grants)

	if equality.Semantic.DeepEqual(*status, req.Status) {
		if c.opts.Defect == DefectDropAccepted {
			return c.dropAccepted(ctx)
		}
		return ctrl.Result{}, nil
	}

	c.recordNewResources(req, status.Resources, grants)

	req.Status = *status
	if err := c.client.Status().Update(ctx, req); err != nil {
		return requeueOnConflict(err)
	}
	log.FromContext(ctx).Info("Wrote the status of the PluginRequest", "request", req.Name)

	if c.opts.Defect == DefectDropAccepted {
		return c.dropAccepted(ctx)
	}

	return ctrl.Result{}, nil
}

// resources are the rows of status.resources of req, which has grants.
func (c *Controller) resources(
	req *pluginsv1alpha1.PluginRequest,
	grants []*pluginsv1alpha1.Grant,
) []pluginsv1alpha1.ResourceRef {
	cm, sc, ing := c.objects(req, false)
	rows := []pluginsv1alpha1.ResourceRef{
		{Kind: "ConfigMap", Name: cm.Name, Role: "echo", OnOrphan: pluginsv1alpha1.OnOrphanKept},
	}

	if c.opts.Defect != DefectUnlistedResource {
		rows = append(rows, pluginsv1alpha1.ResourceRef{
			Kind:     "StorageClass",
			Name:     sc.Name,
			Role:     "echo",
			OnOrphan: pluginsv1alpha1.OnOrphanKept,
		})
	}

	if ing != nil {
		onOrphan := pluginsv1alpha1.OnOrphanDeleted
		if c.opts.Defect == DefectKeepRouteOnOrphan {
			onOrphan = pluginsv1alpha1.OnOrphanKept
		}
		rows = append(
			rows,
			pluginsv1alpha1.ResourceRef{Kind: "Ingress", Name: ing.Name, Role: "route", OnOrphan: onOrphan},
		)
	}

	for _, g := range grants {
		rows = append(
			rows,
			pluginsv1alpha1.ResourceRef{
				Kind:     "Grant",
				Name:     g.Name,
				Role:     "grant",
				OnOrphan: pluginsv1alpha1.OnOrphanKept,
			},
		)
	}

	return rows
}

// recordNewResources records one event for each object of rows that the
// status of req, which has grants, does not list yet. The object is the
// related object of the event, so the recorder does not fold the events of
// two objects into one.
func (c *Controller) recordNewResources(
	req *pluginsv1alpha1.PluginRequest,
	rows []pluginsv1alpha1.ResourceRef,
	grants []*pluginsv1alpha1.Grant,
) {
	if c.opts.Defect == DefectNoEvents {
		return
	}

	objs := c.objectList(req, grants)

	for _, row := range rows {
		listed := slices.ContainsFunc(req.Status.Resources, func(r pluginsv1alpha1.ResourceRef) bool {
			return r.Kind == row.Kind && r.Name == row.Name
		})
		i := slices.IndexFunc(objs, func(o client.Object) bool {
			return o.GetObjectKind().GroupVersionKind().Kind == row.Kind && o.GetName() == row.Name
		})
		if listed || i == -1 {
			continue
		}

		c.recorder.Eventf(
			req,
			objs[i],
			corev1.EventTypeNormal,
			eventReasonApply,
			eventActionApply,
			"Applied the %s %s",
			row.Kind,
			row.Name,
		)
	}
}

// dropAccepted writes the conditions of the registration as Ready only. It
// breaks the rule that the plugin keeps the conditions of other writers.
func (c *Controller) dropAccepted(ctx context.Context) (ctrl.Result, error) {
	var reg pluginsv1alpha1.PluginRegistration
	if err := c.client.Get(ctx, types.NamespacedName{Name: c.opts.Registration}, &reg); err != nil {
		return ctrl.Result{}, err
	}

	ready := meta.FindStatusCondition(reg.Status.Conditions, v1.ConditionReady)
	if ready == nil || len(reg.Status.Conditions) == 1 {
		return ctrl.Result{}, nil
	}

	reg.Status.Conditions = []metav1.Condition{*ready}

	return requeueOnConflict(c.client.Status().Update(ctx, &reg))
}

// apply applies obj with server-side apply.
func (c *Controller) apply(ctx context.Context, obj client.Object) error {
	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	err := c.client.Patch(ctx, obj, client.Apply, client.FieldOwner(fieldManager), client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("applying the %s %s: %w", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
	}

	return nil
}

// deleteOwn deletes each of objs that carries the labels of req, and
// ignores an object that is gone.
func (c *Controller) deleteOwn(ctx context.Context, req *pluginsv1alpha1.PluginRequest, objs ...client.Object) error {
	for _, obj := range objs {
		if err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}

		labels := obj.GetLabels()
		if labels[pluginsv1alpha1.LabelRequest] != req.Name ||
			labels[pluginsv1alpha1.LabelRegistration] != c.opts.Registration {
			continue
		}

		uid := obj.GetUID()
		err := c.client.Delete(ctx, obj, client.Preconditions{UID: &uid})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting the %T %s: %w", obj, obj.GetName(), err)
		}
	}

	return nil
}

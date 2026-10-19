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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The IDs of the lifecycle checks, which check what the plugin writes for
// each request of the run.
const (
	// ReqFinalizer checks that the finalizer comes before any object.
	ReqFinalizer CheckID = "REQ-FINALIZER"
	// ReqOutputsReady checks OutputsReady for the current generation.
	ReqOutputsReady CheckID = "REQ-OUTPUTS-READY"
	// ReqOutputs checks the keys and the values of status.outputs.
	ReqOutputs CheckID = "REQ-OUTPUTS"
	// ReqReady checks the Ready condition of each request.
	ReqReady CheckID = "REQ-READY"
	// ReqLabels checks the writer labels and the owner references.
	ReqLabels CheckID = "REQ-LABELS"
	// ReqResources checks that status.resources lists each object.
	ReqResources CheckID = "REQ-RESOURCES"
	// ReqEvents checks the events of each request.
	ReqEvents CheckID = "REQ-EVENTS"
	// ReqDisplay checks the display fields of the request status.
	ReqDisplay CheckID = "REQ-DISPLAY"
	// ReqLogs checks the log lines of the controller pods. It is skippable.
	ReqLogs CheckID = "REQ-LOGS"
	// ReqAcceptedKept checks that the registration keeps Accepted.
	ReqAcceptedKept CheckID = "REQ-ACCEPTED-KEPT"
)

// kindPluginRequest is the kind of an owner reference to a request.
const kindPluginRequest = "PluginRequest"

// lifecycleChecks run for each request of the run, in this order.
var lifecycleChecks = []Check{
	{
		ID: ReqFinalizer,
		Description: "The request gets the finalizer plugins.camunda.io/cleanup before any object that the " +
			"plugin writes for it.",
		run: checkFinalizer,
	},
	{
		ID:          ReqOutputsReady,
		Description: "The request gets OutputsReady=True for its current generation within the OutputsReady timeout.",
		run:         checkOutputsReady,
	},
	{
		ID:          ReqOutputs,
		Description: "status.outputs has exactly the output keys of the registration, and each value is not empty.",
		run:         checkRequestOutputs,
	},
	{
		ID: ReqReady,
		Description: "The request gets Ready=True within the Ready timeout, or Ready=False with the reason " +
			"GrantRejected while one of its Grants has Ready=False.",
		run: checkReady,
	},
	{
		ID: ReqLabels,
		Description: "Each object of the request carries both writer labels, and a namespaced object has an " +
			"owner reference to the request.",
		run: checkLabels,
	},
	{
		ID:          ReqResources,
		Description: "status.resources lists each object of the request, and each row has kind, name, and onOrphan.",
		run:         checkResources,
	},
	{
		ID:          ReqEvents,
		Description: "The request has at least one event for each row of status.resources.",
		run:         checkEvents,
	},
	{
		ID: ReqDisplay,
		Description: "The summary, summaryTime, links, and podSelector of the request status are complete and " +
			"valid.",
		run: checkDisplay,
	},
	{
		ID: ReqLogs,
		Description: "The controller pods log JSON lines, and a line carries the key request with the name of " +
			"each request.",
		Skippable: true,
		run:       checkLogs,
	},
	{
		ID: ReqAcceptedKept,
		Description: "The registration keeps the Accepted condition of the registry controller while the " +
			"plugin serves requests.",
		run: checkAcceptedKept,
	},
}

func checkFinalizer(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		// A request that never gets outputs fails REQ-OUTPUTS-READY. This
		// check only needs the plugin to have written its objects.
		_ = waitForOutputsReady(ctx, s, r.name, life.createdAt[r.name], false)

		if err := waitForRecorder(ctx, s, life, r.name); err != nil {
			return err
		}

		return finalizerProblem(life.rec, r.name)
	})
}

// waitForRecorder waits until the watches of the run saw the finalizer of
// the request name, when it has one, and each object of the request, so
// that their marks are complete.
func waitForRecorder(ctx context.Context, s *suite, life *lifecycle, name string) error {
	what := "the watches of the run to see the request and its objects"
	return waitFor(ctx, s.cfg.Timeouts.OutputsReady, what, func(ctx context.Context) error {
		if err := life.rec.err(); err != nil {
			return err
		}

		req, err := s.request(ctx, name)
		if err != nil {
			return err
		}

		if controllerutil.ContainsFinalizer(req, pluginsv1alpha1.FinalizerCleanup) && !life.rec.saw(req.UID) {
			return errors.New("the watch has not seen the finalizer of the request yet")
		}

		objs, err := writtenObjects(ctx, s, life.kinds, name)
		if err != nil {
			return err
		}

		for _, obj := range objs {
			if !life.rec.saw(obj.GetUID()) {
				return fmt.Errorf("the watch has not seen %s yet", objectRef(obj))
			}
		}

		return nil
	})
}

func checkOutputsReady(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		if err := waitForOutputsReady(ctx, s, r.name, life.createdAt[r.name], true); err != nil {
			return err
		}

		return s.lateCondition(ctx, r.name, pluginsv1alpha1.ConditionOutputsReady, s.cfg.Timeouts.OutputsReady)
	})
}

// lateCondition fails when the condition condType of the request name became
// True after timeout, also when an earlier check waited past that timeout.
func (s *suite) lateCondition(ctx context.Context, name, condType string, timeout time.Duration) error {
	req, err := s.request(ctx, name)
	if err != nil {
		return err
	}

	return lateProblem(meta.FindStatusCondition(req.Status.Conditions, condType), req.CreationTimestamp, timeout)
}

func checkRequestOutputs(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		_ = waitForOutputsReady(ctx, s, r.name, life.createdAt[r.name], false)

		req, err := s.request(ctx, r.name)
		if err != nil {
			return err
		}

		section := life.sections[r.section].Spec.Outputs
		return problemsError(outputProblems(section, life.registration.Spec.Outputs, req.Status.Outputs))
	})
}

func checkReady(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		deadline := life.createdAt[r.name].Add(s.cfg.Timeouts.Ready)
		err := waitUntil(ctx, deadline, s.cfg.Timeouts.Ready, "Ready=True", func(ctx context.Context) error {
			req, err := s.request(ctx, r.name)
			if err != nil {
				return err
			}

			owned, err := s.ownedGrants(ctx, req)
			if err != nil {
				return err
			}

			return readyVerdict(meta.FindStatusCondition(req.Status.Conditions, v1.ConditionReady), owned)
		})
		if err != nil {
			return err
		}

		return s.lateReady(ctx, r.name)
	})
}

// ownedGrants returns the Grants that carry the label of req and have an
// owner reference to it.
func (s *suite) ownedGrants(ctx context.Context, req *pluginsv1alpha1.PluginRequest) ([]pluginsv1alpha1.Grant, error) {
	var grants pluginsv1alpha1.GrantList
	err := s.client.List(
		ctx,
		&grants,
		client.InNamespace(req.Namespace),
		client.MatchingLabels{pluginsv1alpha1.LabelRequest: req.Name},
	)
	if err != nil {
		return nil, fmt.Errorf("listing the Grants of the request: %w", err)
	}

	return slices.DeleteFunc(grants.Items, func(g pluginsv1alpha1.Grant) bool {
		return !slices.ContainsFunc(g.OwnerReferences, func(o metav1.OwnerReference) bool { return o.UID == req.UID })
	}), nil
}

// lateReady fails when the request name reached the verdict of REQ-READY
// later than the Ready timeout after its creation.
func (s *suite) lateReady(ctx context.Context, name string) error {
	req, err := s.request(ctx, name)
	if err != nil {
		return err
	}

	grants, err := s.ownedGrants(ctx, req)
	if err != nil {
		return err
	}

	ready := meta.FindStatusCondition(req.Status.Conditions, v1.ConditionReady)
	if ready == nil {
		return nil
	}

	verdict := "Ready became " + string(ready.Status)
	if ready.Status != metav1.ConditionTrue {
		verdict += " with the reason " + ready.Reason
	}

	return lateAt(verdict, readySince(ready, grants), req.CreationTimestamp, s.cfg.Timeouts.Ready)
}

// readySince is when ready reached its verdict: its own transition, or for
// GrantRejected, the later of that and the first failure of a Grant.
func readySince(ready *metav1.Condition, grants []pluginsv1alpha1.Grant) metav1.Time {
	since := ready.LastTransitionTime
	if ready.Status == metav1.ConditionTrue {
		return since
	}

	var firstFailure *metav1.Time
	for _, g := range grants {
		c := meta.FindStatusCondition(g.Status.Conditions, v1.ConditionReady)
		if c != nil && c.Status == metav1.ConditionFalse &&
			(firstFailure == nil || c.LastTransitionTime.Before(firstFailure)) {
			firstFailure = &c.LastTransitionTime
		}
	}

	if firstFailure != nil && since.Before(firstFailure) {
		return *firstFailure
	}

	return since
}

func checkLabels(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		req, err := s.request(ctx, r.name)
		if err != nil {
			return err
		}

		objs, err := writtenObjects(ctx, s, life.kinds, r.name)
		if err != nil {
			return err
		}

		var problems []string
		for _, obj := range objs {
			problems = append(problems, labelProblems(obj, req)...)
		}

		return problemsError(problems)
	})
}

func checkResources(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		req, err := s.request(ctx, r.name)
		if err != nil {
			return err
		}

		objs, err := writtenObjects(ctx, s, life.kinds, r.name)
		if err != nil {
			return err
		}
		life.seen[r.name] = mergeObjects(life.seen[r.name], objs)

		return problemsError(resourceProblems(objs, req.Status.Resources))
	})
}

func checkEvents(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		what := "an event for each row of status.resources"
		return waitFor(ctx, s.cfg.Timeouts.Ready, what, func(ctx context.Context) error {
			req, err := s.request(ctx, r.name)
			if err != nil {
				return err
			}

			var events corev1.EventList
			err = s.client.List(
				ctx,
				&events,
				client.InNamespace(req.Namespace),
				client.MatchingFields{"involvedObject.uid": string(req.UID)},
			)
			if err != nil {
				return fmt.Errorf("listing the events of the request: %w", err)
			}

			if n, rows := len(events.Items), len(req.Status.Resources); n < rows {
				return fmt.Errorf("the request has %d events, and status.resources lists %d rows", n, rows)
			}

			return nil
		})
	})
}

func checkDisplay(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		req, err := s.request(ctx, r.name)
		if err != nil {
			return err
		}

		problems := displayProblems(req.Status)

		if req.Status.PodSelector != nil {
			problem, err := podSelectorProblem(ctx, s, req)
			if err != nil {
				return err
			}
			if problem != "" {
				problems = append(problems, problem)
			}
		}

		return problemsError(problems)
	})
}

func podSelectorProblem(ctx context.Context, s *suite, req *pluginsv1alpha1.PluginRequest) (string, error) {
	selector, err := metav1.LabelSelectorAsSelector(req.Status.PodSelector)
	if err != nil {
		return "status.podSelector is not a valid label selector: " + err.Error(), nil
	}

	var pods corev1.PodList
	err = s.client.List(
		ctx,
		&pods,
		client.InNamespace(req.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	)
	if err != nil {
		return "", fmt.Errorf("listing the pods of status.podSelector: %w", err)
	}

	if !slices.ContainsFunc(pods.Items, podRunningAndReady) {
		return "no pod that status.podSelector selects is Running with Ready=True", nil
	}

	return "", nil
}

func podRunningAndReady(p corev1.Pod) bool {
	return p.Status.Phase == corev1.PodRunning &&
		slices.ContainsFunc(p.Status.Conditions, func(c corev1.PodCondition) bool {
			return c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue
		})
}

func checkLogs(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	sa := life.registration.Spec.ServiceAccount
	lines, err := controllerLogLines(ctx, s, sa, life.started)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		return problemsError(logProblems(lines, r.name))
	})
}

// controllerLogLines reads the log lines since started of each container of
// each pod that runs under sa.
func controllerLogLines(
	ctx context.Context,
	s *suite,
	sa pluginsv1alpha1.ServiceAccountRef,
	started time.Time,
) ([]string, error) {
	var pods corev1.PodList
	if err := s.client.List(ctx, &pods, client.InNamespace(sa.Namespace)); err != nil {
		return nil, fmt.Errorf("listing the pods of %s: %w", sa.Namespace, err)
	}

	pods.Items = slices.DeleteFunc(pods.Items, func(p corev1.Pod) bool { return p.Spec.ServiceAccountName != sa.Name })
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pod runs under the ServiceAccount %s/%s", sa.Namespace, sa.Name)
	}

	clientset, err := kubernetes.NewForConfig(s.restConfig)
	if err != nil {
		return nil, fmt.Errorf("connecting to the pod logs: %w", err)
	}

	since := metav1.NewTime(started)
	var lines []string
	for _, p := range pods.Items {
		for _, c := range p.Spec.Containers {
			opts := &corev1.PodLogOptions{Container: c.Name, SinceTime: &since}
			raw, err := clientset.CoreV1().Pods(p.Namespace).GetLogs(p.Name, opts).DoRaw(ctx)
			if err != nil {
				return nil, fmt.Errorf(
					"reading the log of the container %s of the pod %s/%s: %w",
					c.Name,
					p.Namespace,
					p.Name,
					err,
				)
			}
			lines = append(lines, strings.Split(string(raw), "\n")...)
		}
	}

	return lines, nil
}

func checkAcceptedKept(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	err = acceptedProblem(
		life.accepted,
		meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted),
	)
	if err != nil {
		return err
	}

	what := "the watch of the registration to reach the read of the check"
	err = waitFor(ctx, s.cfg.Timeouts.OutputsReady, what, func(context.Context) error {
		if err := life.rec.err(); err != nil {
			return err
		}
		if !life.rec.caughtUp(registrationStream, reg.ResourceVersion) {
			return fmt.Errorf("the watch has not seen resource version %s yet", reg.ResourceVersion)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if m, ok := life.rec.first(acceptedChangedKey); ok {
		return fmt.Errorf(
			"the Accepted condition of the registration changed during the run, at resource version %s",
			m.resourceVersion,
		)
	}

	return nil
}

// registrationStream names the watch of the registration in the recorder.
const registrationStream = "registration"

// acceptedChangedKey is the recorder key of a registration whose Accepted
// condition is not the one that the run started with.
const acceptedChangedKey = "accepted-changed"

// acceptedKey gives a registration whose Accepted condition is not kept the
// key acceptedChangedKey.
func acceptedKey(kept *metav1.Condition) func(client.Object) string {
	return func(obj client.Object) string {
		reg, ok := obj.(*pluginsv1alpha1.PluginRegistration)
		if !ok ||
			acceptedProblem(
				kept,
				meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted),
			) == nil {
			return ""
		}

		return acceptedChangedKey
	}
}

// eachRun calls check for each run, and joins the errors, each prefixed with
// the name of its request.
func eachRun(runs []requestRun, check func(r requestRun) error) error {
	var errs []error
	for _, r := range runs {
		if err := check(r); err != nil {
			errs = append(errs, fmt.Errorf("request %s: %w", r.name, err))
		}
	}

	return errors.Join(errs...)
}

// problemsError joins problems into one error, or returns nil when there is
// none.
func problemsError(problems []string) error {
	errs := make([]error, 0, len(problems))
	for _, p := range problems {
		errs = append(errs, errors.New(p))
	}

	return errors.Join(errs...)
}

// request gets the request name of the run.
func (s *suite) request(ctx context.Context, name string) (*pluginsv1alpha1.PluginRequest, error) {
	var req pluginsv1alpha1.PluginRequest
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: s.cfg.Namespace, Name: name}, &req); err != nil {
		return nil, fmt.Errorf("getting the PluginRequest %s/%s: %w", s.cfg.Namespace, name, err)
	}

	return &req, nil
}

// waitForOutputsReady waits for OutputsReady=True on the request name, for
// its current generation when current is true, until the OutputsReady
// timeout after since.
func waitForOutputsReady(ctx context.Context, s *suite, name string, since time.Time, current bool) error {
	deadline := since.Add(s.cfg.Timeouts.OutputsReady)
	return waitUntil(ctx, deadline, s.cfg.Timeouts.OutputsReady, "OutputsReady=True", func(ctx context.Context) error {
		req, err := s.request(ctx, name)
		if err != nil {
			return err
		}

		return outputsReadyProblem(req, current)
	})
}

func outputsReadyProblem(req *pluginsv1alpha1.PluginRequest, current bool) error {
	c := meta.FindStatusCondition(req.Status.Conditions, pluginsv1alpha1.ConditionOutputsReady)

	switch {
	case c == nil:
		return errors.New("the request has no OutputsReady condition")
	case c.Status != metav1.ConditionTrue:
		return fmt.Errorf("the OutputsReady condition is %s, reason %s: %s", c.Status, c.Reason, c.Message)
	case current && c.ObservedGeneration != req.Generation:
		return fmt.Errorf(
			"the OutputsReady condition is True at observedGeneration %d, and the request has generation %d",
			c.ObservedGeneration,
			req.Generation,
		)
	}

	return nil
}

// outputProblems names each key of section that outputs lacks, each key of
// outputs not in declared, and each declared key with an empty value.
func outputProblems(section, declared []pluginsv1alpha1.OutputDecl, outputs map[string]string) []string {
	var problems []string
	for _, d := range section {
		if _, ok := outputs[d.Key]; !ok {
			problems = append(problems, fmt.Sprintf("output %q is missing", d.Key))
		}
	}

	keys := slices.Sorted(maps.Keys(outputs))
	for _, k := range keys {
		switch {
		case !slices.ContainsFunc(declared, func(d pluginsv1alpha1.OutputDecl) bool { return d.Key == k }):
			problems = append(problems, fmt.Sprintf("output %q is not declared in spec.outputs of the registration", k))
		case outputs[k] == "":
			problems = append(problems, fmt.Sprintf("output %q is empty", k))
		}
	}

	return problems
}

// readyVerdict passes Ready=True, and Ready=False with the reason
// GrantRejected while one of grants has Ready=False.
func readyVerdict(ready *metav1.Condition, grants []pluginsv1alpha1.Grant) error {
	switch {
	case ready == nil:
		return errors.New("the request has no Ready condition")
	case ready.Status == metav1.ConditionTrue:
		return nil
	case ready.Status == metav1.ConditionFalse && ready.Reason == pluginsv1alpha1.ReasonGrantRejected:
		failed := slices.ContainsFunc(grants, func(g pluginsv1alpha1.Grant) bool {
			return meta.IsStatusConditionFalse(g.Status.Conditions, v1.ConditionReady)
		})
		if failed {
			return nil
		}

		return errors.New(
			"the Ready condition is False with the reason GrantRejected while no Grant of the request has Ready=False",
		)
	}

	return fmt.Errorf("the Ready condition is %s, reason %s: %s", ready.Status, ready.Reason, ready.Message)
}

// labelProblems names each writer rule of the contract that obj breaks. A
// namespaced object is in the namespace of request and has an owner
// reference to it. A cluster-scoped object has no owner reference to a
// PluginRequest.
func labelProblems(obj *unstructured.Unstructured, request *pluginsv1alpha1.PluginRequest) []string {
	ref := objectRef(obj)
	var problems []string

	if obj.GetLabels()[pluginsv1alpha1.LabelRegistration] != request.Spec.Registration {
		problems = append(problems, fmt.Sprintf(
			"%s has no label %s: %q",
			ref,
			pluginsv1alpha1.LabelRegistration,
			request.Spec.Registration,
		))
	}

	switch {
	case obj.GetNamespace() == "":
		if slices.ContainsFunc(obj.GetOwnerReferences(), isPluginRequestOwner) {
			problems = append(problems, ref+" is cluster-scoped and has an owner reference to the request. "+
				"A cluster-scoped object carries the labels only")
		}
	case obj.GetNamespace() != request.Namespace:
		problems = append(problems, ref+" is outside the namespace of the request")
	case !slices.ContainsFunc(obj.GetOwnerReferences(), func(o metav1.OwnerReference) bool {
		return isPluginRequestOwner(o) && o.Name == request.Name && o.UID == request.UID
	}):
		problems = append(problems, ref+" has no owner reference to the request")
	}

	return problems
}

func isPluginRequestOwner(o metav1.OwnerReference) bool {
	return o.APIVersion == pluginsv1alpha1.GroupVersion.String() && o.Kind == kindPluginRequest
}

// resourceProblems names each written object that rows does not list, and
// each row without kind, name, or onOrphan. A row without an object is a
// resource outside the cluster.
func resourceProblems(written []*unstructured.Unstructured, rows []pluginsv1alpha1.ResourceRef) []string {
	var problems []string
	for _, obj := range written {
		listed := slices.ContainsFunc(rows, func(r pluginsv1alpha1.ResourceRef) bool {
			return r.Kind == obj.GetKind() && r.Name == obj.GetName()
		})
		if !listed {
			problems = append(problems, objectRef(obj)+" is not listed in status.resources")
		}
	}

	for _, r := range rows {
		fields := []struct{ name, value string }{{"kind", r.Kind}, {"name", r.Name}, {"onOrphan", string(r.OnOrphan)}}
		for _, f := range fields {
			if f.value == "" {
				problems = append(problems, fmt.Sprintf("status.resources row %s %q has no %s", r.Kind, r.Name, f.name))
			}
		}
	}

	return problems
}

// displayProblems names each display field of status that breaks the
// contract. An empty severity is Info, the default of the CRD.
func displayProblems(status pluginsv1alpha1.PluginRequestStatus) []string {
	var problems []string
	if len(status.Summary) == 0 {
		problems = append(problems, "status.summary has no item")
	}

	for _, item := range status.Summary {
		switch item.Severity {
		case "", pluginsv1alpha1.SeverityInfo, pluginsv1alpha1.SeverityWarning, pluginsv1alpha1.SeverityError:
		default:
			problems = append(problems, fmt.Sprintf(
				"status.summary item %q has the severity %q, which is not Info, Warning, or Error",
				item.Label,
				item.Severity,
			))
		}
	}

	if status.SummaryTime == nil {
		problems = append(problems, "status.summaryTime is not set")
	}

	for _, l := range status.Links {
		if u, err := url.Parse(l.URL); err != nil || u.Scheme == "" || u.Host == "" {
			problems = append(
				problems,
				fmt.Sprintf("status.links item %q has the URL %q, which is not absolute", l.Name, l.URL),
			)
		}
	}

	return problems
}

// maxQuotedLine is the length at which a log problem cuts the line it quotes.
const maxQuotedLine = 120

// logProblems names the first line of lines that is not a JSON object, and
// says when no line carries the key request with the name request, as a
// string or as an object with that name.
func logProblems(lines []string, request string) []string {
	var problems []string
	found := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry == nil {
			if len(problems) == 0 {
				quoted := line[:min(len(line), maxQuotedLine)]
				problems = append(problems, fmt.Sprintf("the log line %q is not a JSON object", quoted))
			}
			continue
		}

		switch v := entry["request"].(type) {
		case string:
			found = found || v == request
		case map[string]any:
			found = found || v["name"] == request
		}
	}

	if !found {
		problems = append(
			problems,
			fmt.Sprintf("no log line carries the key request with the request name %q", request),
		)
	}

	return problems
}

// acceptedProblem says whether now is the Accepted condition kept, with the
// same status, reason, and lastTransitionTime.
func acceptedProblem(kept, now *metav1.Condition) error {
	switch {
	case kept == nil:
		return errors.New("the registration had no Accepted condition when the run started")
	case now == nil:
		return errors.New("the registration has no Accepted condition any more")
	case now.Status != kept.Status || now.Reason != kept.Reason || now.Message != kept.Message ||
		now.ObservedGeneration != kept.ObservedGeneration || !now.LastTransitionTime.Equal(&kept.LastTransitionTime):
		return fmt.Errorf(
			"the Accepted condition of the registration was %s, and is now %s",
			describeCondition(kept),
			describeCondition(now),
		)
	}

	return nil
}

func describeCondition(c *metav1.Condition) string {
	return fmt.Sprintf(
		"%s, reason %s, message %q, generation %d, since %s",
		c.Status,
		c.Reason,
		c.Message,
		c.ObservedGeneration,
		c.LastTransitionTime.UTC().Format(time.RFC3339),
	)
}

// lateProblem says whether cond became True later than timeout after created.
// Both times come from the API server. A condition that is not True passes.
func lateProblem(cond *metav1.Condition, created metav1.Time, timeout time.Duration) error {
	if cond == nil || cond.Status != metav1.ConditionTrue {
		return nil
	}

	return lateAt(cond.Type+" became True", cond.LastTransitionTime, created, timeout)
}

// lateAt says whether the verdict, for example "Ready became True", came at
// a time later than timeout after created.
func lateAt(verdict string, at, created metav1.Time, timeout time.Duration) error {
	// The times have whole seconds.
	if after := at.Sub(created.Time); after > timeout+time.Second {
		return fmt.Errorf(
			"%s %s after the request was created, which is more than the timeout of %s",
			verdict,
			after,
			timeout,
		)
	}

	return nil
}

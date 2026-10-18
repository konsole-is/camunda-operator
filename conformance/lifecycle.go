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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// claimName is the claim that every request of a run names. No such claim
// exists.
const claimName = "conformance"

// requestRun is one request of the run: a case with one deletion policy.
type requestRun struct {
	caseName string
	policy   pluginsv1alpha1.DeletionPolicy
	name     string
	tc       Case
}

// lifecycle is the state that the lifecycle and deletion checks of one run
// share. When err is set, the run could not create its requests, and every
// such check fails with err.
type lifecycle struct {
	err          error
	started      time.Time
	registration *pluginsv1alpha1.PluginRegistration
	accepted     *metav1.Condition
	section      *pluginsv1alpha1.PluginSection
	kinds        []kindInfo
	runs         []requestRun
	rec          *recorder
	// preexisting holds the UIDs of the objects that carried the label of a
	// request of the run before the run created its requests.
	preexisting map[types.UID]bool
	// written holds, for each request name, the objects that the request had
	// before the deletion checks deleted it.
	written map[string][]*unstructured.Unstructured
}

// lifecycle prepares the requests of the run on the first call, and returns
// the same state and error on every later call.
func (s *suite) lifecycle(ctx context.Context) (*lifecycle, error) {
	if s.life == nil {
		life, err := s.prepare(ctx)
		if err != nil {
			life = &lifecycle{err: fmt.Errorf("the run could not create its requests: %w", err)}
		}
		s.life = life
	}

	return s.life, s.life.err
}

// prepare creates the namespace of the run when it is missing, starts the
// recorder, and creates one request for each run. The cleanups of the suite
// remove what it created, also when it fails halfway.
func (s *suite) prepare(ctx context.Context) (*lifecycle, error) {
	reg, err := s.registration(ctx)
	if err != nil {
		return nil, err
	}

	runs, err := runsOf(s.cfg.Cases, reg.Spec.DeletionPolicies)
	if err != nil {
		return nil, err
	}

	var section pluginsv1alpha1.PluginSection
	if err := s.client.Get(ctx, client.ObjectKey{Name: reg.Spec.Sections[0]}, &section); err != nil {
		return nil, fmt.Errorf("PluginSection %q of the registration: %w", reg.Spec.Sections[0], err)
	}

	kinds, err := pluginKinds(ctx, s, reg.Spec.ServiceAccount)
	if err != nil {
		return nil, err
	}

	if err := s.ensureNamespace(ctx); err != nil {
		return nil, err
	}

	rec, preexisting, err := startRecorder(ctx, s, kinds, runs)
	if err != nil {
		return nil, err
	}
	s.onClose(func(ctx context.Context) error {
		rec.stop()
		return s.deleteRequests(ctx, runs)
	})

	life := &lifecycle{
		started:      time.Now(),
		registration: reg,
		accepted:     meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted),
		section:      &section,
		kinds:        kinds,
		runs:         runs,
		rec:          rec,
		preexisting:  preexisting,
		written:      map[string][]*unstructured.Unstructured{},
	}

	for _, run := range runs {
		req := requestFor(reg.Name, &section, s.cfg.Namespace, run)
		if err := s.client.Create(ctx, req); err != nil {
			return nil, fmt.Errorf(
				"creating the PluginRequest %s/%s of case %q: %w",
				req.Namespace,
				req.Name,
				run.caseName,
				err,
			)
		}
	}

	return life, nil
}

// runsOf gives each case one run, or one run for each of policies when the
// case leaves its deletion policy empty. It refuses a request name that is
// not a DNS label, and two runs with one request name.
func runsOf(cases []Case, policies []pluginsv1alpha1.DeletionPolicy) ([]requestRun, error) {
	var runs []requestRun
	var errs []error
	caseOf := map[string]string{}

	for _, tc := range cases {
		casePolicies := []pluginsv1alpha1.DeletionPolicy{tc.DeletionPolicy}
		if tc.DeletionPolicy == "" {
			casePolicies = policies
		}

		for _, policy := range casePolicies {
			name := requestName(tc.Name, policy)
			if problems := validation.IsDNS1123Label(name); len(problems) > 0 {
				errs = append(errs, fmt.Errorf(
					"case %q gives the request name %q, which is not a DNS label: %s",
					tc.Name,
					name,
					strings.Join(problems, ", "),
				))
				continue
			}

			if other, ok := caseOf[name]; ok {
				errs = append(errs, fmt.Errorf(
					"case %q gives the request name %q, which case %q also gives",
					tc.Name,
					name,
					other,
				))
				continue
			}

			caseOf[name] = tc.Name
			runs = append(runs, requestRun{caseName: tc.Name, policy: policy, name: name, tc: tc})
		}
	}

	return runs, errors.Join(errs...)
}

// requestName is the name of the request of a case under one policy.
func requestName(caseName string, policy pluginsv1alpha1.DeletionPolicy) string {
	return strings.ToLower(caseName) + "-" + strings.ToLower(string(policy))
}

func (s *suite) ensureNamespace(ctx context.Context) error {
	var ns corev1.Namespace
	err := s.client.Get(ctx, client.ObjectKey{Name: s.cfg.Namespace}, &ns)
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("getting the namespace %s: %w", s.cfg.Namespace, err)
	}

	ns.Name = s.cfg.Namespace
	if err := s.client.Create(ctx, &ns); err != nil {
		return fmt.Errorf("creating the namespace %s: %w", s.cfg.Namespace, err)
	}
	s.onClose(func(ctx context.Context) error {
		return client.IgnoreNotFound(s.client.Delete(ctx, &ns))
	})

	return nil
}

// startRecorder watches the requests of the run for the cleanup finalizer,
// and each of kinds for the first object of each request. It also returns the
// UIDs of the objects that carry the label of a request already. The watches
// start after them and outlive the check that starts them, until the recorder
// stops.
func startRecorder(
	ctx context.Context,
	s *suite,
	kinds []kindInfo,
	runs []requestRun,
) (*recorder, map[types.UID]bool, error) {
	c, err := client.NewWithWatch(
		s.restConfig,
		client.Options{Scheme: s.client.Scheme(), Mapper: s.client.RESTMapper()},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting the watches of the run: %w", err)
	}

	names := make([]string, 0, len(runs))
	for _, r := range runs {
		names = append(names, r.name)
	}

	ofRun, err := labels.NewRequirement(pluginsv1alpha1.LabelRequest, selection.In, names)
	if err != nil {
		return nil, nil, fmt.Errorf("selecting the objects of the requests: %w", err)
	}
	ofRunSelector := client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*ofRun)}

	watchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	rec := &recorder{stop: cancel}

	err = rec.watch(
		watchCtx, c, &pluginsv1alpha1.PluginRequestList{}, func(obj client.Object) string {
			if controllerutil.ContainsFinalizer(obj, pluginsv1alpha1.FinalizerCleanup) {
				return finalizerKey(obj.GetName())
			}
			return ""
		}, client.InNamespace(s.cfg.Namespace),
	)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("watching the PluginRequests of the run: %w", err)
	}

	preexisting := map[types.UID]bool{}
	for _, k := range kinds {
		listKind := k.gvk.GroupVersion().WithKind(k.gvk.Kind + "List")
		existing := &unstructured.UnstructuredList{}
		existing.SetGroupVersionKind(listKind)
		if err := s.client.List(ctx, existing, ofRunSelector); err != nil {
			cancel()
			return nil, nil, fmt.Errorf("listing the %s objects of the run: %w", k.gvk.Kind, err)
		}

		for _, item := range existing.Items {
			preexisting[item.GetUID()] = true
		}

		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(listKind)
		after := &client.ListOptions{Raw: &metav1.ListOptions{ResourceVersion: existing.GetResourceVersion()}}

		err := rec.watch(
			watchCtx, c, list, func(obj client.Object) string {
				return objectKey(obj.GetLabels()[pluginsv1alpha1.LabelRequest])
			}, ofRunSelector, after,
		)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("watching the %s objects of the run: %w", k.gvk.Kind, err)
		}
	}

	return rec, preexisting, nil
}

// deleteRequests deletes the requests of runs that still exist. It does not
// wait until they are gone.
func (s *suite) deleteRequests(ctx context.Context, runs []requestRun) error {
	errs := make([]error, 0, len(runs))
	for _, r := range runs {
		req := &pluginsv1alpha1.PluginRequest{}
		req.Namespace = s.cfg.Namespace
		req.Name = r.name
		errs = append(errs, client.IgnoreNotFound(s.client.Delete(ctx, req)))
	}

	return errors.Join(errs...)
}

// requestFor is the request of run, as a claim controller writes it. It names
// the plane claim kind when section applies to plane claims only.
func requestFor(
	registration string,
	section *pluginsv1alpha1.PluginSection,
	namespace string,
	run requestRun,
) *pluginsv1alpha1.PluginRequest {
	kind := pluginsv1alpha1.ClaimKindCamundaClusterClaim
	if len(section.Spec.AppliesTo) > 0 &&
		!slices.Contains(section.Spec.AppliesTo, pluginsv1alpha1.ClaimKindCamundaClusterClaim) {
		kind = pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim
	}

	return &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.name,
			Namespace: namespace,
			Labels: map[string]string{
				pluginsv1alpha1.LabelSection:      section.Name,
				pluginsv1alpha1.LabelRegistration: registration,
				fleetv1alpha1.LabelClaim:          claimName,
			},
		},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Section:        section.Name,
			Registration:   registration,
			ClaimRef:       pluginsv1alpha1.ClaimRef{Kind: kind, Name: claimName},
			Target:         run.tc.Target,
			Inputs:         run.tc.Inputs,
			Parameters:     run.tc.Parameters,
			DeletionPolicy: run.policy,
		},
	}
}

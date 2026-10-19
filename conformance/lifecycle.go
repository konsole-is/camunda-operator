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
	"crypto/sha256"
	"encoding/hex"
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

// requestRun is one request of the run: one case, section and deletion policy.
type requestRun struct {
	caseName string
	section  string
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
	sections     map[string]*pluginsv1alpha1.PluginSection
	kinds        []kindInfo
	runs         []requestRun
	rec          *recorder
	// uids holds the UID of each request that the run created, by name.
	uids map[string]types.UID
	// createdAt holds when the run created each request, by name. The
	// OutputsReady and Ready timeouts count from it.
	createdAt map[string]time.Time
	// seen holds, for each request name, the objects that the lifecycle
	// checks found.
	seen map[string][]*unstructured.Unstructured
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

	runs, err := runsOf(s.cfg.Cases, reg.Spec.Sections, reg.Spec.DeletionPolicies, runIDOf(s.cfg.Namespace))
	if err != nil {
		return nil, err
	}

	sections := make(map[string]*pluginsv1alpha1.PluginSection, len(reg.Spec.Sections))
	for _, name := range reg.Spec.Sections {
		if sections[name], err = s.section(ctx, name); err != nil {
			return nil, err
		}
	}

	kinds, err := pluginKinds(ctx, s, reg.Spec.ServiceAccount)
	if err != nil {
		return nil, err
	}

	if err := s.ensureNamespace(ctx); err != nil {
		return nil, err
	}

	rec, err := startRecorder(ctx, s, kinds, runs, reg)
	if err != nil {
		return nil, err
	}
	var created []*pluginsv1alpha1.PluginRequest
	// This cleanup runs after the next one, when the requests are deleted,
	// so that a rerun in the namespace finds no object of this run.
	s.onClose(func(ctx context.Context) error {
		return s.deleteRunObjects(ctx, kinds, created)
	})
	s.onClose(func(ctx context.Context) error {
		rec.stop()
		return s.deleteCreated(ctx, created)
	})

	life := &lifecycle{
		started:      time.Now(),
		registration: reg,
		accepted:     meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted),
		sections:     sections,
		kinds:        kinds,
		runs:         runs,
		rec:          rec,
		uids:         map[string]types.UID{},
		createdAt:    map[string]time.Time{},
		written:      map[string][]*unstructured.Unstructured{},
		seen:         map[string][]*unstructured.Unstructured{},
	}

	for _, run := range runs {
		req := requestFor(reg.Name, sections[run.section], s.cfg.Namespace, run)
		if err := s.client.Create(ctx, req); err != nil {
			return nil, fmt.Errorf(
				"creating the PluginRequest %s/%s of case %q: %w",
				req.Namespace,
				req.Name,
				run.caseName,
				err,
			)
		}
		created = append(created, req)
		life.uids[run.name] = req.UID
		life.createdAt[run.name] = time.Now()
	}

	return life, nil
}

// runsOf gives each case one run for each of sections, with its deletion
// policy, or with each of policies when the case leaves its deletion policy
// empty. A request name holds the section only when there are several. It
// refuses a request name that is not a DNS label, and two runs with one
// request name.
func runsOf(
	cases []Case,
	sections []string,
	policies []pluginsv1alpha1.DeletionPolicy,
	runID string,
) ([]requestRun, error) {
	var runs []requestRun
	var errs []error
	caseOf := map[string]string{}

	for _, tc := range cases {
		casePolicies := []pluginsv1alpha1.DeletionPolicy{tc.DeletionPolicy}
		if tc.DeletionPolicy == "" {
			casePolicies = policies
		}

		for _, section := range sections {
			for _, policy := range casePolicies {
				nameSection := ""
				if len(sections) > 1 {
					nameSection = section
				}
				name := requestName(tc.Name, nameSection, string(policy), runID)
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
				run := requestRun{caseName: tc.Name, section: section, policy: policy, name: name, tc: tc}
				if len(casePolicies) > 1 {
					run.tc.Target = targetWithSuffix(tc.Target, strings.ToLower(string(policy)))
				}
				runs = append(runs, run)
			}
		}
	}

	return runs, errors.Join(errs...)
}

// targetWithSuffix gives a run that shares a case with another run a target
// of its own, so that the deletion of one does not delete what the other
// keeps.
func targetWithSuffix(target pluginsv1alpha1.TargetRef, suffix string) pluginsv1alpha1.TargetRef {
	target.Name += "-" + suffix
	if target.ServiceAccountName != "" {
		target.ServiceAccountName += "-" + suffix
	}

	return target
}

// requestName is the name of the request of a case for a section under one
// policy, in the run runID. The section and runID can be empty.
func requestName(caseName, section, policy, runID string) string {
	parts := []string{strings.ToLower(caseName), section, strings.ToLower(policy), runID}
	return strings.Join(slices.DeleteFunc(parts, func(p string) bool { return p == "" }), "-")
}

// runIDOf is the part of the request names of a run in namespace. Two runs
// at the same time use two namespaces, so their requests, and the objects
// that carry the names of their requests, stay apart.
func runIDOf(namespace string) string {
	sum := sha256.Sum256([]byte(namespace))
	return hex.EncodeToString(sum[:5])
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
	uid := ns.UID
	s.onClose(func(ctx context.Context) error {
		err := s.client.Delete(ctx, &ns, client.Preconditions{UID: &uid})
		if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
			return nil
		}
		return err
	})

	return nil
}

// startRecorder watches the requests of the run for the cleanup finalizer,
// and each of kinds for the first object of each request. It fails when an
// object carries the label of a request already, which an earlier run in the
// namespace left. The watches outlive the check that starts them, until the
// recorder stops.
func startRecorder(
	ctx context.Context,
	s *suite,
	kinds []kindInfo,
	runs []requestRun,
	reg *pluginsv1alpha1.PluginRegistration,
) (*recorder, error) {
	c, err := client.NewWithWatch(
		s.restConfig,
		client.Options{Scheme: s.client.Scheme(), Mapper: s.client.RESTMapper()},
	)
	if err != nil {
		return nil, fmt.Errorf("connecting the watches of the run: %w", err)
	}

	names := make([]string, 0, len(runs))
	for _, r := range runs {
		names = append(names, r.name)
	}

	ofRun, err := labels.NewRequirement(pluginsv1alpha1.LabelRequest, selection.In, names)
	if err != nil {
		return nil, fmt.Errorf("selecting the objects of the requests: %w", err)
	}
	ofRunSelector := client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*ofRun)}

	watchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	rec := &recorder{stop: cancel}

	err = rec.watch(
		watchCtx,
		c,
		func() client.ObjectList { return &pluginsv1alpha1.PluginRequestList{} },
		func(obj client.Object) string {
			if controllerutil.ContainsFinalizer(obj, pluginsv1alpha1.FinalizerCleanup) {
				return finalizerKey(obj.GetName())
			}
			return ""
		},
		"",
		client.InNamespace(s.cfg.Namespace),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watching the PluginRequests of the run: %w", err)
	}

	accepted := meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted)
	rec.advance(registrationStream, reg.ResourceVersion)
	err = rec.watch(
		watchCtx,
		c,
		func() client.ObjectList { return &pluginsv1alpha1.PluginRegistrationList{} },
		func(obj client.Object) string {
			rec.advance(registrationStream, obj.GetResourceVersion())
			return acceptedKey(accepted)(obj)
		},
		"",
		client.MatchingFields{"metadata.name": reg.Name},
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watching the PluginRegistration %s: %w", reg.Name, err)
	}

	for _, k := range kinds {
		listKind := k.gvk.GroupVersion().WithKind(k.gvk.Kind + "List")
		existing := &unstructured.UnstructuredList{}
		existing.SetGroupVersionKind(listKind)
		if err := s.client.List(ctx, existing, ofRunSelector); err != nil {
			cancel()
			return nil, fmt.Errorf("listing the %s objects of the run: %w", k.gvk.Kind, err)
		}

		if len(existing.Items) > 0 {
			cancel()
			return nil, fmt.Errorf(
				"%s is of an earlier run in the namespace %s: use a new namespace",
				objectRef(&existing.Items[0]),
				s.cfg.Namespace,
			)
		}

		newList := func() client.ObjectList {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(listKind)
			return list
		}

		err := rec.watch(
			watchCtx, c, newList, func(obj client.Object) string {
				return objectKey(obj.GetLabels()[pluginsv1alpha1.LabelRequest])
			}, existing.GetResourceVersion(), ofRunSelector,
		)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("watching the %s objects of the run: %w", k.gvk.Kind, err)
		}
	}

	return rec, nil
}

// deleteRunObjects deletes each object that carries the request label of one
// of created, such as an object that an Orphan request kept, and waits within
// the Deletion timeout until they are gone. It keeps an object that changed
// its UID meanwhile.
func (s *suite) deleteRunObjects(
	ctx context.Context,
	kinds []kindInfo,
	created []*pluginsv1alpha1.PluginRequest,
) error {
	var errs []error
	var deleted []*unstructured.Unstructured
	for _, req := range created {
		objs, err := writtenObjects(ctx, s, kinds, req.Name)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, obj := range objs {
			uid := obj.GetUID()
			err := s.client.Delete(ctx, obj, client.Preconditions{UID: &uid})
			switch {
			case err == nil:
				deleted = append(deleted, obj)
			case !apierrors.IsNotFound(err) && !apierrors.IsConflict(err):
				errs = append(errs, fmt.Errorf("deleting %s: %w", objectRef(obj), err))
			}
		}
	}

	errs = append(
		errs,
		waitFor(ctx, s.cfg.Timeouts.Deletion, "the objects of the run to be gone", func(ctx context.Context) error {
			var problems []string
			for _, obj := range deleted {
				got := &unstructured.Unstructured{}
				got.SetGroupVersionKind(obj.GroupVersionKind())
				err := s.client.Get(ctx, client.ObjectKeyFromObject(obj), got)
				switch {
				case apierrors.IsNotFound(err):
				case err != nil:
					return err
				case got.GetUID() == obj.GetUID():
					problems = append(problems, objectRef(obj)+" still exists")
				}
			}

			return problemsError(problems)
		}),
	)

	return errors.Join(errs...)
}

// deleteCreated deletes the requests that the run created and that still
// exist. A request of the same name that the run did not create stays. It
// does not wait until they are gone.
func (s *suite) deleteCreated(ctx context.Context, created []*pluginsv1alpha1.PluginRequest) error {
	errs := make([]error, 0, len(created))
	for _, req := range created {
		err := s.client.Delete(ctx, req, client.Preconditions{UID: &req.UID})
		if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			errs = append(errs, err)
		}
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

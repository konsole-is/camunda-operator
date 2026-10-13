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
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// camundaGroupSuffix ends the name of each API group of the operator.
const camundaGroupSuffix = ".camunda.io"

// scope is the set of objects that a scenario holds: every object of the
// namespaces of the scenario, those Namespaces, and every cluster-scoped
// object of the operator. It leaves out the core Events, because the
// events.k8s.io Events show the same records.
type scope struct {
	client        dynamic.Interface
	namespaced    []schema.GroupVersionResource
	clusterScoped []schema.GroupVersionResource
	observed      map[schema.GroupKind]bool
	namespaces    []string
}

// newScope reads the API resources and the CRDs of the cluster at cfg once.
// A resource or a CRD that appears later is not in the scope.
func newScope(ctx context.Context, cfg *rest.Config) (*scope, error) {
	// list sends one request per resource and namespace, and settle calls it
	// four times a second. The default client rate of 5 per second would
	// make each list take seconds.
	cfg = rest.CopyConfig(cfg)
	cfg.QPS, cfg.Burst = 1000, 2000
	// A list of a deprecated resource, such as v1 Endpoints, warns on each
	// call.
	cfg.WarningHandler = rest.NoWarnings{}

	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating the discovery client: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating the dynamic client: %w", err)
	}

	lists, err := disc.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("reading the API resources: %w", err)
	}

	s := &scope{
		client: dyn,
		observed: map[schema.GroupKind]bool{
			{Group: "apps", Kind: "StatefulSet"}: true,
			{Group: "apps", Kind: "Deployment"}:  true,
			{Group: "apps", Kind: "ReplicaSet"}:  true,
		},
	}
	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return nil, fmt.Errorf("reading the API resources: %w", err)
		}
		for _, r := range list.APIResources {
			if !slices.Contains(r.Verbs, "list") || strings.Contains(r.Name, "/") ||
				(gv.Group == "" && r.Kind == "Event") {
				continue
			}
			gvr := gv.WithResource(r.Name)
			switch {
			case r.Namespaced:
				s.namespaced = append(s.namespaced, gvr)
			case strings.HasSuffix(gv.Group, camundaGroupSuffix):
				s.clusterScoped = append(s.clusterScoped, gvr)
			}
		}
	}

	crds, err := dyn.Resource(schema.GroupVersionResource{
		Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions",
	}).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing the CRDs: %w", err)
	}
	for _, crd := range crds.Items {
		group, _, _ := unstructured.NestedString(crd.Object, "spec", "group")
		kind, _, _ := unstructured.NestedString(crd.Object, "spec", "names", "kind")
		if strings.HasSuffix(group, camundaGroupSuffix) && hasStatusSubresource(&crd) {
			s.observed[schema.GroupKind{Group: group, Kind: kind}] = true
		}
	}

	if err := s.warmUp(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

// warmUp lists each resource of s once, all at the same time. The API
// server takes about a second to answer the first request for each custom
// resource, so the first list of the scope would take half a minute of the
// first settle.
func (s *scope) warmUp(ctx context.Context) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, gvr := range slices.Concat(s.namespaced, s.clusterScoped) {
		wg.Go(func() {
			_, err := s.client.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("listing %s: %w", gvr.String(), err))
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	return errors.Join(errs...)
}

// hasStatusSubresource reports whether a version of crd serves a status
// subresource.
func hasStatusSubresource(crd *unstructured.Unstructured) bool {
	versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	for _, v := range versions {
		version, _ := v.(map[string]any)
		if _, found, _ := unstructured.NestedMap(version, "subresources", "status"); found {
			return true
		}
	}
	return false
}

// add adds namespaces to the scope.
func (s *scope) add(namespaces ...string) {
	for _, ns := range namespaces {
		if !slices.Contains(s.namespaces, ns) {
			s.namespaces = append(s.namespaces, ns)
		}
	}
}

// list returns every object of the scope.
func (s *scope) list(ctx context.Context) ([]unstructured.Unstructured, error) {
	var objs []unstructured.Unstructured
	namespaces := s.client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"})
	for _, ns := range s.namespaces {
		obj, err := namespaces.Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("reading Namespace %s: %w", ns, err)
		}
		objs = append(objs, *obj)

		for _, gvr := range s.namespaced {
			list, err := s.client.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing %s in %s: %w", gvr.String(), ns, err)
			}
			objs = append(objs, list.Items...)
		}
	}

	for _, gvr := range s.clusterScoped {
		list, err := s.client.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing %s: %w", gvr.String(), err)
		}
		objs = append(objs, list.Items...)
	}

	return objs, nil
}

// unobserved returns the description of each object of objs that must
// report an observed generation and does not report its generation. An
// object without status.observedGeneration has not observed it.
func (s *scope) unobserved(objs []unstructured.Unstructured) []string {
	var names []string
	for i := range objs {
		if !s.observed[objs[i].GroupVersionKind().GroupKind()] {
			continue
		}
		observed, found, _ := unstructured.NestedInt64(objs[i].Object, "status", "observedGeneration")
		if !found || observed != objs[i].GetGeneration() {
			names = append(names, describe(&objs[i]))
		}
	}
	return names
}

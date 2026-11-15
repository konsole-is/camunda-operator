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

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// requestTimeout bounds each API request of the kubectl calls of this file, so
// that one stalled request does not use all of the context of the caller.
const requestTimeout = "--request-timeout=10s"

// CRD is a custom resource definition that the cluster serves.
type CRD struct {
	// Name is the name of the definition, <plural>.<group>. kubectl accepts
	// it as a resource type.
	Name  string
	Group string
	Kind  string
}

// CustomResource is a custom resource in the cluster, with the fields that
// tell why its deletion does not end.
type CustomResource struct {
	// Resource is the Name of the CRD of the resource.
	Resource string
	Kind     string
	// Namespace is empty for a cluster-scoped resource.
	Namespace         string
	Name              string
	Finalizers        []string
	DeletionTimestamp *metav1.Time
}

// CRDsOfGroups returns the CRDs that the cluster serves in one of groups.
func CRDsOfGroups(ctx context.Context, groups ...string) ([]CRD, error) {
	out, err := RunContext(ctx, "kubectl", "get", "crd", "-o", "json", requestTimeout)
	if err != nil {
		return nil, err
	}

	return parseCRDs(out, groups)
}

// parseCRDs returns the CRDs of a kubectl list of definitions whose group is
// one of groups.
func parseCRDs(out string, groups []string) ([]CRD, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Group string `json:"group"`
				Names struct {
					Kind string `json:"kind"`
				} `json:"names"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, fmt.Errorf("decoding the CRD list: %w", err)
	}

	var crds []CRD
	for _, item := range list.Items {
		if slices.Contains(groups, item.Spec.Group) {
			crds = append(crds, CRD{Name: item.Metadata.Name, Group: item.Spec.Group, Kind: item.Spec.Names.Kind})
		}
	}

	return crds, nil
}

// ListCustomResources returns every resource of crds in every namespace. No
// CRDs is no resources.
func ListCustomResources(ctx context.Context, crds []CRD) ([]CustomResource, error) {
	if len(crds) == 0 {
		return nil, nil
	}

	out, err := RunContext(ctx, "kubectl", "get", CRDNames(crds), "--all-namespaces", "-o", "json", requestTimeout)
	if err != nil {
		return nil, err
	}

	return parseCustomResources(out, crds)
}

// CRDNames returns the names of crds as one comma-separated resource type
// argument of kubectl.
func CRDNames(crds []CRD) string {
	names := make([]string, 0, len(crds))
	for _, crd := range crds {
		names = append(names, crd.Name)
	}

	return strings.Join(names, ",")
}

// parseCustomResources returns the resources of a kubectl list of objects of
// crds. A resource whose group and kind match no CRD of crds is an error.
func parseCustomResources(out string, crds []CRD) ([]CustomResource, error) {
	var list struct {
		Items []struct {
			APIVersion string            `json:"apiVersion"`
			Kind       string            `json:"kind"`
			Metadata   metav1.ObjectMeta `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, fmt.Errorf("decoding the custom resource list: %w", err)
	}

	resources := make([]CustomResource, 0, len(list.Items))
	for _, item := range list.Items {
		group, _, _ := strings.Cut(item.APIVersion, "/")
		i := slices.IndexFunc(crds, func(crd CRD) bool { return crd.Group == group && crd.Kind == item.Kind })
		if i < 0 {
			return nil, fmt.Errorf("%s %s has no CRD in the list", item.APIVersion, item.Kind)
		}

		resources = append(resources, CustomResource{
			Resource:          crds[i].Name,
			Kind:              item.Kind,
			Namespace:         item.Metadata.Namespace,
			Name:              item.Metadata.Name,
			Finalizers:        item.Metadata.Finalizers,
			DeletionTimestamp: item.Metadata.DeletionTimestamp,
		})
	}

	return resources, nil
}

// FormatCustomResources returns one line for each resource: the kind, the
// namespace and name, the finalizers, and the deletion timestamp, or "none"
// when the deletion was not requested.
func FormatCustomResources(resources []CustomResource) string {
	var b strings.Builder
	for _, r := range resources {
		name := r.Name
		if r.Namespace != "" {
			name = r.Namespace + "/" + r.Name
		}

		deletion := "none"
		if r.DeletionTimestamp != nil {
			deletion = r.DeletionTimestamp.UTC().Format(time.RFC3339)
		}

		_, _ = fmt.Fprintf(
			&b,
			"%s %s finalizers=[%s] deletionTimestamp=%s\n",
			r.Kind,
			name,
			strings.Join(r.Finalizers, ","),
			deletion,
		)
	}

	return b.String()
}

// ForceDelete deletes resource and removes its finalizers, so that it goes
// without the controller that set them.
func ForceDelete(ctx context.Context, resource CustomResource) error {
	target := append([]string{resource.Resource, resource.Name}, namespaceArgs(resource.Namespace)...)
	deleteArgs := append([]string{"delete"}, target...)
	deleteArgs = append(deleteArgs, "--ignore-not-found", "--wait=false", requestTimeout)
	if _, err := RunContext(ctx, "kubectl", deleteArgs...); err != nil {
		return err
	}

	patchArgs := append([]string{"patch"}, target...)
	patchArgs = append(patchArgs, "--type=merge", "-p", `{"metadata":{"finalizers":null}}`, requestTimeout)
	_, err := RunContext(ctx, "kubectl", patchArgs...)

	return err
}

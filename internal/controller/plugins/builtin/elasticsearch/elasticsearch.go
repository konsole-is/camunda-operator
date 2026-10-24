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

// Package elasticsearch serves the built-in registration elasticsearch.
package elasticsearch

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	escomponents "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/pluginbacking"
	eswrapper "github.com/konsole-is/camunda-operator/pkg/wrappers/elasticsearchcluster"
)

// Handler serves the built-in registration elasticsearch. It applies
// the ElasticsearchCluster <claim>-es, from the preset, the release, and the
// overrides of the parameters and from the inputs, and publishes the
// SecondaryStorageConfig of the same name. An override never lowers the
// live storage size.
type Handler struct{}

// elasticsearchParameters are the parameters of an elasticsearch request.
type elasticsearchParameters struct {
	Preset     string
	ReleaseRef string
	Overrides  elasticsearchOverrides
}

type elasticsearchOverrides struct {
	Replicas    *int32
	Resources   *corev1.ResourceRequirements
	StorageSize *resource.Quantity
	Scheduling  *v1.SchedulingSpec
	PodLabels   map[string]string
}

// Build implements adapter.Handler.
func (Handler) Build(request *pluginsv1alpha1.PluginRequest, live adapter.LiveObjects) (adapter.Served, error) {
	p, err := decodeElasticsearchParameters(request.Spec.Parameters)
	if err != nil {
		return adapter.Served{}, err
	}

	desired, shrink, err := elasticsearchClusterFor(request, p, live)
	if err != nil {
		return adapter.Served{}, err
	}

	backing, cell, err := backingComponent(desired, pluginbacking.Lifecycle{})
	if err != nil {
		return adapter.Served{}, err
	}

	contract := elasticsearchContract(request)
	outputs, _, err := adapter.OutputsComponent([]adapter.ContractRef{contract}, request.Namespace)
	if err != nil {
		return adapter.Served{}, err
	}

	return adapter.Served{
		Outputs: outputs,
		Backing: backing,
		Report: func() adapter.Report {
			summary := elasticsearchSummary(cell.Get())
			if shrink != nil {
				summary = append(summary, *shrink)
			}
			return adapter.Report{
				Outputs: map[string]string{contract.Output: contract.Name},
				Summary: summary,
				Resources: []pluginsv1alpha1.ResourceRef{{
					Kind:     adapter.KindElasticsearchCluster,
					Name:     desired.Name,
					OnOrphan: pluginsv1alpha1.OnOrphanKept,
				}},
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					labels.ElasticsearchClusterKey: labels.ElasticsearchCluster(desired.Name).Name,
				}},
			}
		},
	}, nil
}

// elasticsearchClusterFor returns the ElasticsearchCluster to apply for
// request. An asked storage size below the live size of the cluster keeps
// the live size, and shrink is the summary item that says so.
func elasticsearchClusterFor(
	request *pluginsv1alpha1.PluginRequest,
	p elasticsearchParameters,
	live adapter.LiveObjects,
) (cluster *v1.ElasticsearchCluster, shrink *pluginsv1alpha1.SummaryItem, err error) {
	name := elasticsearchClusterName(request)
	cluster = &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: request.Namespace,
			Labels:    adapter.BackingLabels(request),
		},
		Spec: v1.ElasticsearchClusterSpec{
			PresetRef:              p.Preset,
			ReleaseRef:             p.ReleaseRef,
			SecondaryStorageConfig: name,
			Replicas:               p.Overrides.Replicas,
			Resources:              p.Overrides.Resources,
			StorageSize:            p.Overrides.StorageSize,
			Scheduling:             p.Overrides.Scheduling,
			PodLabels:              p.Overrides.PodLabels,
			SnapshotStorageRef:     request.Spec.Inputs[pluginstartup.OutputObjectStorageConfig],
		},
	}

	if class := request.Spec.Inputs[pluginstartup.OutputStorageClassName]; class != "" {
		cluster.Spec.StorageClassName = &class
	}

	if current, ok := adapter.LiveAs[*v1.ElasticsearchCluster](live, adapter.KindElasticsearchCluster, name); ok {
		cluster.Spec.StorageSize, shrink = adapter.KeepLiveStorageSize(
			cluster.Spec.StorageSize, current.Spec.StorageSize, current.Status.Volumes,
		)
	}

	cluster.Spec.ServiceAccount, err = serviceAccountFor(cluster, request.Spec.Inputs)
	if err != nil {
		return nil, nil, err
	}

	return cluster, shrink, nil
}

// serviceAccountFor returns the ServiceAccount block of cluster for inputs.
// With the bucket input or the annotations input, it pins the name that the
// cloud identity request trusts, so that a preset cannot move the account
// away from it. Without both, it is nil and the preset decides.
func serviceAccountFor(cluster *v1.ElasticsearchCluster, inputs map[string]string) (*v1.ServiceAccountSpec, error) {
	annotations, err := adapter.DecodeAnnotations(inputs)
	if err != nil {
		return nil, err
	}
	if annotations == nil && inputs[pluginstartup.OutputObjectStorageConfig] == "" {
		return nil, nil
	}

	return &v1.ServiceAccountSpec{
		Name:        escomponents.ServiceAccountName(cluster, v1.ElasticsearchClusterSpec{}),
		Annotations: annotations,
	}, nil
}

// backingComponent returns the component that writes cluster for a request in
// lifecycle, and the cell that holds the live cluster once it is read.
func backingComponent(
	cluster *v1.ElasticsearchCluster,
	lifecycle pluginbacking.Lifecycle,
) (*component.Component, *concepts.Data[v1.ElasticsearchCluster], error) {
	cell := concepts.NewData[v1.ElasticsearchCluster]("cluster")
	b := eswrapper.NewBuilder(cluster)
	eswrapper.ExtractInto(b, cell, func(c v1.ElasticsearchCluster) (v1.ElasticsearchCluster, error) {
		return c, nil
	})
	res, err := b.Build()
	if err != nil {
		return nil, nil, fmt.Errorf("building the ElasticsearchCluster: %w", err)
	}

	backing, err := component.NewComponentBuilder().
		WithName("backing").
		WithConditionType(component.ConditionType(v1.ConditionReady)).
		WithResource(res, lifecycle.Options(false)...).
		Build()
	if err != nil {
		return nil, nil, fmt.Errorf("building the backing component: %w", err)
	}

	return backing, cell, nil
}

// elasticsearchSummary returns the summary of a request from the
// ElasticsearchCluster as the backing component last read it. ok is false
// before the first read.
func elasticsearchSummary(cluster v1.ElasticsearchCluster, ok bool) []pluginsv1alpha1.SummaryItem {
	headline := pluginsv1alpha1.SummaryItem{Label: "Elasticsearch", Severity: pluginsv1alpha1.SeverityWarning}
	if !ok {
		headline.Value = "waiting"
		return []pluginsv1alpha1.SummaryItem{headline}
	}

	// Each node has one bound data volume.
	nodes := strconv.Itoa(len(cluster.Status.Volumes)) + " nodes"
	ready := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionReady)
	switch {
	case cluster.Status.ObservedGeneration != cluster.Generation ||
		ready != nil && ready.ObservedGeneration != cluster.Generation:
		headline.Value = "updating, " + nodes
	case ready != nil && ready.Status == metav1.ConditionTrue:
		headline.Value = "Ready, " + nodes
		headline.Severity = pluginsv1alpha1.SeverityInfo
	case ready != nil && ready.Reason != "":
		headline.Value = "not Ready (" + ready.Reason + "), " + nodes
	default:
		headline.Value = "not Ready, " + nodes
	}

	summary := []pluginsv1alpha1.SummaryItem{headline}
	if len(cluster.Status.Volumes) > 0 {
		largest := slices.MaxFunc(cluster.Status.Volumes, func(a, b v1.VolumeStatus) int {
			return a.Capacity.Cmp(b.Capacity)
		})
		summary = append(summary, pluginsv1alpha1.SummaryItem{
			Label:    "Disk",
			Value:    largest.Capacity.String() + " per node",
			Severity: pluginsv1alpha1.SeverityInfo,
		})
	}

	return summary
}

// Cleanup deletes the ElasticsearchCluster with the Delete policy, only
// when the request owns it, and keeps it without the owner reference with
// the Orphan policy.
func (Handler) Cleanup(
	request *pluginsv1alpha1.PluginRequest,
	policy pluginsv1alpha1.DeletionPolicy,
	_ adapter.LiveObjects,
) (adapter.Cleanup, error) {
	cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{
		Name: elasticsearchClusterName(request), Namespace: request.Namespace,
	}}
	backing, _, err := backingComponent(cluster.DeepCopy(), pluginbacking.Deleting(policy))
	if err != nil {
		return adapter.Cleanup{}, err
	}

	return adapter.Cleanup{Backing: backing, Remaining: []client.Object{cluster}}, nil
}

// Contracts returns the SecondaryStorageConfig of the cluster.
func (Handler) Contracts(request *pluginsv1alpha1.PluginRequest) []adapter.ContractRef {
	if _, err := decodeElasticsearchParameters(request.Spec.Parameters); err != nil {
		return nil
	}

	return []adapter.ContractRef{elasticsearchContract(request)}
}

// Backing implements adapter.Handler.
func (Handler) Backing(request *pluginsv1alpha1.PluginRequest) []adapter.BackingRef {
	return []adapter.BackingRef{{
		Kind: adapter.KindElasticsearchCluster,
		Object: &v1.ElasticsearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: elasticsearchClusterName(request), Namespace: request.Namespace},
		},
	}}
}

func elasticsearchClusterName(request *pluginsv1alpha1.PluginRequest) string {
	return request.Spec.ClaimRef.Name + pluginstartup.BackingSuffixElasticsearch
}

// elasticsearchContract returns the SecondaryStorageConfig that the cluster
// of request publishes.
func elasticsearchContract(request *pluginsv1alpha1.PluginRequest) adapter.ContractRef {
	return adapter.ContractRef{
		Output: pluginstartup.OutputSecondaryStorageConfig,
		Kind:   adapter.KindSecondaryStorageConfig,
		Name:   elasticsearchClusterName(request),
	}
}

// decodeElasticsearchParameters decodes parameters with no unknown field.
// A value that does not decode returns a *conditions.PreCheckFailure with
// reason InvalidParameters, whose message names the field.
func decodeElasticsearchParameters(parameters *apiextensionsv1.JSON) (elasticsearchParameters, error) {
	var top map[string]json.RawMessage
	if err := adapter.DecodeStrict(parameters, &top); err != nil {
		return elasticsearchParameters{}, adapter.InvalidParameter("spec.parameters", err)
	}

	var p elasticsearchParameters
	var overrides map[string]json.RawMessage
	if err := adapter.DecodeFields(top, "spec.parameters.", map[string]any{
		"preset":     &p.Preset,
		"releaseRef": &p.ReleaseRef,
		"overrides":  &overrides,
	}); err != nil {
		return elasticsearchParameters{}, err
	}

	if err := adapter.DecodeFields(overrides, "spec.parameters.overrides.", map[string]any{
		"replicas":    &p.Overrides.Replicas,
		"resources":   &p.Overrides.Resources,
		"storageSize": &p.Overrides.StorageSize,
		"scheduling":  &p.Overrides.Scheduling,
		"podLabels":   &p.Overrides.PodLabels,
	}); err != nil {
		return elasticsearchParameters{}, err
	}

	return p, nil
}

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

package builtin

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
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	escomponents "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	eswrapper "github.com/konsole-is/camunda-operator/pkg/wrappers/elasticsearchcluster"
)

// kindElasticsearchCluster is the kind of the backing object of elasticsearch.
const kindElasticsearchCluster = "ElasticsearchCluster"

// elasticsearch serves the built-in registration elasticsearch. It applies
// the ElasticsearchCluster <claim>-es, from the preset, the release, and the
// overrides of the parameters and from the inputs, and publishes the
// SecondaryStorageConfig of the same name. The storage size only grows.
type elasticsearch struct{}

// elasticsearchParameters are the parameters of an elasticsearch request.
type elasticsearchParameters struct {
	Preset     string                 `json:"preset"`
	ReleaseRef string                 `json:"releaseRef"`
	Overrides  elasticsearchOverrides `json:"overrides"`
}

type elasticsearchOverrides struct {
	Replicas    *int32                       `json:"replicas,omitempty"`
	Resources   *corev1.ResourceRequirements `json:"resources,omitempty"`
	StorageSize *resource.Quantity           `json:"storageSize,omitempty"`
	Scheduling  *v1.SchedulingSpec           `json:"scheduling,omitempty"`
	PodLabels   map[string]string            `json:"podLabels,omitempty"`
}

func (elasticsearch) build(request *pluginsv1alpha1.PluginRequest, live liveObjects) (served, error) {
	p, err := decodeElasticsearchParameters(request.Spec.Parameters)
	if err != nil {
		return served{}, err
	}

	desired, shrink, err := elasticsearchClusterFor(request, p, live)
	if err != nil {
		return served{}, err
	}

	cell := concepts.NewData[v1.ElasticsearchCluster]("cluster")
	b := eswrapper.NewBuilder(desired)
	eswrapper.ExtractInto(b, cell, func(c v1.ElasticsearchCluster) (v1.ElasticsearchCluster, error) {
		return c, nil
	})
	res, err := b.Build()
	if err != nil {
		return served{}, fmt.Errorf("building the ElasticsearchCluster: %w", err)
	}
	backing, err := component.NewComponentBuilder().
		WithName("backing").
		WithConditionType(component.ConditionType(v1.ConditionReady)).
		WithResource(res, component.DeleteOnlyIfOwned()).
		Build()
	if err != nil {
		return served{}, fmt.Errorf("building the backing component: %w", err)
	}

	contract := contractRef{
		output: pluginstartup.OutputSecondaryStorageConfig,
		kind:   kindSecondaryStorageConfig,
		name:   desired.Name,
	}
	read, _, err := readContract(contract, request.Namespace)
	if err != nil {
		return served{}, err
	}
	outputs, err := component.NewComponentBuilder().
		WithName("outputs").
		WithConditionType(component.ConditionType(pluginsv1alpha1.ConditionOutputsReady)).
		WithResource(read, component.ReadOnly(), component.BlockOnAbsence()).
		Build()
	if err != nil {
		return served{}, fmt.Errorf("building the outputs component: %w", err)
	}

	return served{
		outputs: outputs,
		backing: backing,
		report: func() report {
			summary := elasticsearchSummary(cell.Get())
			if shrink != nil {
				summary = append(summary, *shrink)
			}
			return report{
				outputs: map[string]string{contract.output: desired.Name},
				summary: summary,
				resources: []pluginsv1alpha1.ResourceRef{{
					Kind:     kindElasticsearchCluster,
					Name:     desired.Name,
					OnOrphan: pluginsv1alpha1.OnOrphanKept,
				}},
				podSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					labels.ElasticsearchClusterKey: labels.ElasticsearchCluster(desired.Name).Name,
				}},
			}
		},
	}, nil
}

// elasticsearchClusterFor returns the ElasticsearchCluster to apply for
// request. When the asked storage size is smaller than liveStorageSize, the
// cluster keeps that size, and shrink is the summary item that says so.
func elasticsearchClusterFor(
	request *pluginsv1alpha1.PluginRequest,
	p elasticsearchParameters,
	live liveObjects,
) (cluster *v1.ElasticsearchCluster, shrink *pluginsv1alpha1.SummaryItem, err error) {
	name := elasticsearchClusterName(request)
	cluster = &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: request.Namespace,
			Labels: map[string]string{
				pluginsv1alpha1.LabelRegistration: pluginstartup.RegistrationElasticsearch,
				pluginsv1alpha1.LabelRequest:      request.Name,
			},
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

	current, _ := liveAs[*v1.ElasticsearchCluster](live, kindElasticsearchCluster, name)
	if size := liveStorageSize(current); cluster.Spec.StorageSize != nil && size != nil &&
		cluster.Spec.StorageSize.Cmp(*size) < 0 {
		asked, kept := cluster.Spec.StorageSize.String(), size.String()
		cluster.Spec.StorageSize = size
		shrink = &pluginsv1alpha1.SummaryItem{
			Label:    "Storage size",
			Value:    "Storage size " + asked + " is smaller than " + kept + ", kept " + kept,
			Severity: pluginsv1alpha1.SeverityWarning,
		}
	}

	cluster.Spec.ServiceAccount, err = serviceAccountFor(cluster, request.Spec.Inputs)
	if err != nil {
		return nil, nil, err
	}

	return cluster, shrink, nil
}

// liveStorageSize returns the larger of spec.storageSize and the largest data
// volume of cluster, or nil when cluster is nil or has neither.
func liveStorageSize(cluster *v1.ElasticsearchCluster) *resource.Quantity {
	if cluster == nil {
		return nil
	}

	var size *resource.Quantity
	if cluster.Spec.StorageSize != nil {
		size = new(cluster.Spec.StorageSize.DeepCopy())
	}
	for _, volume := range cluster.Status.Volumes {
		if size == nil || volume.Capacity.Cmp(*size) > 0 {
			size = new(volume.Capacity.DeepCopy())
		}
	}

	return size
}

// serviceAccountFor returns the ServiceAccount block of cluster for inputs.
// With the bucket input or the annotations input, it pins the name that the
// cloud identity request trusts, so that a preset cannot move the account
// away from it. Without both, it is nil and the preset decides.
func serviceAccountFor(cluster *v1.ElasticsearchCluster, inputs map[string]string) (*v1.ServiceAccountSpec, error) {
	raw := inputs[pluginstartup.OutputServiceAccountAnnotations]
	if raw == "" && inputs[pluginstartup.OutputObjectStorageConfig] == "" {
		return nil, nil
	}

	account := &v1.ServiceAccountSpec{Name: escomponents.ServiceAccountName(cluster, v1.ElasticsearchClusterSpec{})}
	if raw == "" {
		return account, nil
	}

	var annotations map[string]*string
	invalid := &conditions.PreCheckFailure{
		Reason:  pluginsv1alpha1.ReasonInvalidParameters,
		Message: "spec.inputs." + pluginstartup.OutputServiceAccountAnnotations + " is not a JSON object of strings",
	}
	if err := json.Unmarshal([]byte(raw), &annotations); err != nil || annotations == nil {
		return nil, invalid
	}
	account.Annotations = make(map[string]string, len(annotations))
	for key, value := range annotations {
		if value == nil {
			return nil, invalid
		}
		account.Annotations[key] = *value
	}

	return account, nil
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

// cleanup deletes the ElasticsearchCluster with the Delete policy, only
// when the request owns it, and keeps it without the owner reference with
// the Orphan policy.
func (elasticsearch) cleanup(
	request *pluginsv1alpha1.PluginRequest,
	policy pluginsv1alpha1.DeletionPolicy,
	live liveObjects,
) (cleanup, error) {
	name := elasticsearchClusterName(request)
	b := eswrapper.NewBuilder(&v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: request.Namespace},
	})
	res, err := b.Build()
	if err != nil {
		return cleanup{}, fmt.Errorf("building the ElasticsearchCluster: %w", err)
	}

	options := []component.ResourceOption{component.OrphanWhen(true)}
	if policy == pluginsv1alpha1.DeletionPolicyDelete {
		options = []component.ResourceOption{component.DeleteWhen(true), component.DeleteOnlyIfOwned()}
	}
	comp, err := component.NewComponentBuilder().
		WithName("cleanup").
		WithConditionType(component.ConditionType(v1.ConditionReady)).
		WithResource(res, options...).
		Build()
	if err != nil {
		return cleanup{}, fmt.Errorf("building the cleanup component: %w", err)
	}

	c := cleanup{components: []*component.Component{comp}}
	// Only an owned cluster is deleted, so only an owned cluster can hold the
	// finalizer.
	if current, ok := liveAs[*v1.ElasticsearchCluster](live, kindElasticsearchCluster, name); ok &&
		policy == pluginsv1alpha1.DeletionPolicyDelete && metav1.IsControlledBy(current, request) {
		c.remaining = []client.Object{&v1.ElasticsearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: request.Namespace},
		}}
	}

	return c, nil
}

// contracts returns the SecondaryStorageConfig of the cluster.
func (elasticsearch) contracts(request *pluginsv1alpha1.PluginRequest) []contractRef {
	if _, err := decodeElasticsearchParameters(request.Spec.Parameters); err != nil {
		return nil
	}

	return []contractRef{{
		output: pluginstartup.OutputSecondaryStorageConfig,
		kind:   kindSecondaryStorageConfig,
		name:   elasticsearchClusterName(request),
	}}
}

func (elasticsearch) backing(request *pluginsv1alpha1.PluginRequest) []backingRef {
	return []backingRef{{
		kind: kindElasticsearchCluster,
		object: &v1.ElasticsearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: elasticsearchClusterName(request), Namespace: request.Namespace},
		},
	}}
}

func elasticsearchClusterName(request *pluginsv1alpha1.PluginRequest) string {
	return request.Spec.ClaimRef.Name + pluginstartup.BackingSuffixElasticsearch
}

// decodeElasticsearchParameters decodes parameters with no unknown field.
// A value that does not decode returns a *conditions.PreCheckFailure with
// reason InvalidParameters, whose message names the field.
func decodeElasticsearchParameters(parameters *apiextensionsv1.JSON) (elasticsearchParameters, error) {
	var top map[string]json.RawMessage
	if err := decodeStrict(parameters, &top); err != nil {
		return elasticsearchParameters{}, invalidParameter("spec.parameters", err)
	}

	var p elasticsearchParameters
	var overrides map[string]json.RawMessage
	if err := decodeFields(top, "spec.parameters.", map[string]any{
		"preset":     &p.Preset,
		"releaseRef": &p.ReleaseRef,
		"overrides":  &overrides,
	}); err != nil {
		return elasticsearchParameters{}, err
	}

	if err := decodeFields(overrides, "spec.parameters.overrides.", map[string]any{
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

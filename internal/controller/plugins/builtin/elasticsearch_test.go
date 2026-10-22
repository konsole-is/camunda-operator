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
	"flag"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./internal/controller/plugins/builtin/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

func TestElasticsearchGolden(t *testing.T) {
	tests := []struct {
		name, params string
		inputs       map[string]string
	}{
		{"preset", `{"preset":"small","releaseRef":"camunda-8-9"}`, nil},
		{
			"overrides",
			`{"preset":"small","releaseRef":"camunda-8-9","overrides":{"replicas":5,` +
				`"resources":{"limits":{"memory":"4Gi"}},"storageSize":"80Gi",` +
				`"scheduling":{"tolerations":[{"key":"pool","value":"es"}]},"podLabels":{"team":"a"}}}`,
			nil,
		},
		{
			"inputs",
			`{"preset":"small","releaseRef":"camunda-8-9"}`,
			map[string]string{"storageClassName": "encrypted", "objectStorageConfig": "orders-backup"},
		},
		{
			"annotations",
			`{"preset":"small","releaseRef":"camunda-8-9"}`,
			map[string]string{
				"serviceAccountAnnotations": `{"eks.amazonaws.com/role-arn":"arn:aws:iam::1:role/orders"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := elasticsearch{}.build(elasticsearchRequest(tt.params, tt.inputs), nil)
			require.NoError(t, err)

			golden.AssertComponentYAML(
				t, "testdata/golden/elasticsearch-"+tt.name+".yaml", s.backing,
				golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
			)
		})
	}
}

func TestElasticsearchMapsReplicasToSpecReplicas(t *testing.T) {
	s, err := elasticsearch{}.build(elasticsearchRequest(`{"preset":"small","overrides":{"replicas":5}}`, nil), nil)
	require.NoError(t, err)

	assert.Equal(t, int32(5), *rendered(t, s).Spec.Replicas)
}

func TestElasticsearchNamesTheClusterAfterTheClaim(t *testing.T) {
	request := elasticsearchRequest(`{"preset":"small","releaseRef":"camunda-8-9"}`, nil)

	s, err := elasticsearch{}.build(request, nil)
	require.NoError(t, err)

	cluster := rendered(t, s)
	assert.Equal(t, "orders-es", cluster.Name)
	assert.Equal(t, request.Namespace, cluster.Namespace)
	assert.Equal(t, "small", cluster.Spec.PresetRef)
	assert.Equal(t, "camunda-8-9", cluster.Spec.ReleaseRef)
	assert.Equal(t, "orders-es", cluster.Spec.SecondaryStorageConfig)
	assert.Equal(
		t, map[string]string{
			pluginsv1alpha1.LabelRegistration: pluginstartup.RegistrationElasticsearch,
			pluginsv1alpha1.LabelRequest:      request.Name,
		}, cluster.Labels,
	)
	assert.Equal(t, map[string]string{pluginstartup.OutputSecondaryStorageConfig: "orders-es"}, s.report().outputs)
	assert.Equal(
		t, []contractRef{{
			output: pluginstartup.OutputSecondaryStorageConfig, kind: kindSecondaryStorageConfig, name: "orders-es",
		}}, elasticsearch{}.contracts(request),
	)
	refs := elasticsearch{}.backing(request)
	require.Len(t, refs, 1)
	assert.Equal(t, kindElasticsearchCluster, refs[0].kind)
	assert.Equal(t, "orders-es", refs[0].object.GetName())
	assert.Equal(t, request.Namespace, refs[0].object.GetNamespace())
}

func TestElasticsearchKeepsTheLiveStorageSize(t *testing.T) {
	tests := []struct {
		name, asked, live, volume, wantSize string
		wantItem                            bool
	}{
		{"smaller", "20Gi", "50Gi", "", "50Gi", true},
		{"larger", "80Gi", "50Gi", "", "80Gi", false},
		{"equal", "50Gi", "50Gi", "", "50Gi", false},
		{"no live size", "20Gi", "", "", "20Gi", false},
		{"no override, live 50Gi", "", "50Gi", "", "", false},
		{"smaller than the volume of a preset size", "20Gi", "", "50Gi", "50Gi", true},
		{"smaller than a volume that grew past the spec", "20Gi", "30Gi", "50Gi", "50Gi", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := `{"preset":"small"}`
			if tt.asked != "" {
				params = `{"preset":"small","overrides":{"storageSize":"` + tt.asked + `"}}`
			}
			current := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "orders-es"}}
			if tt.live != "" {
				current.Spec.StorageSize = new(resource.MustParse(tt.live))
			}
			if tt.volume != "" {
				current.Status.Volumes = []v1.VolumeStatus{{Name: "data", Capacity: resource.MustParse(tt.volume)}}
			}
			live := liveObjects{{kind: kindElasticsearchCluster, name: "orders-es"}: current}

			s, err := elasticsearch{}.build(elasticsearchRequest(params, nil), live)
			require.NoError(t, err)

			size := rendered(t, s).Spec.StorageSize
			if tt.wantSize == "" {
				assert.Nil(t, size)
			} else {
				require.NotNil(t, size)
				assert.Equal(t, tt.wantSize, size.String())
			}
			item := pluginsv1alpha1.SummaryItem{
				Label:    "Storage size",
				Value:    "Storage size 20Gi is smaller than 50Gi, kept 50Gi",
				Severity: pluginsv1alpha1.SeverityWarning,
			}
			if tt.wantItem {
				assert.Contains(t, s.report().summary, item)
			} else {
				for _, got := range s.report().summary {
					assert.NotEqual(t, "Storage size", got.Label)
				}
			}
		})
	}
}

func TestElasticsearchRefusesOverridesThatDoNotDecode(t *testing.T) {
	tests := []struct{ params, field string }{
		{`{"preset":"small","overrides":{"resources":{"limitz":{}}}}`, "limitz"},
		{`{"preset":"small","overrides":{"storageSize":"lots"}}`, "storageSize"},
		{`{"preset":"small","overrides":{"nodeCount":3}}`, "nodeCount"},
		{`{"preset":"small","overrides":{"scheduling":{"nodeSelector":{"pool":"es"}}}}`, "nodeSelector"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			_, err := elasticsearch{}.build(elasticsearchRequest(tt.params, nil), nil)

			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
			assert.Contains(t, failure.Message, tt.field)
		})
	}
}

func TestElasticsearchRefusesParameterNamesInAnotherCase(t *testing.T) {
	schema, err := newParameterSchema(schemaOf(t, pluginstartup.RegistrationElasticsearch))
	require.NoError(t, err)
	r := &Reconciler{
		APIReader: fakeReader(t),
		schemas:   map[string]*parameterSchema{pluginstartup.RegistrationElasticsearch: schema},
	}
	tests := []struct{ params, field string }{
		{`{"preset":"small","releaseRef":"approved","releaseref":"other"}`, "releaseref"},
		{`{"preset":"small","releaseRef":"approved","Overrides":{"replicas":0}}`, "Overrides"},
		{`{"preset":"small","releaseRef":"approved","overrides":{"Replicas":0}}`, "Replicas"},
		{
			`{"preset":"small","releaseRef":"approved","overrides":{"resources":` +
				`{"limits":{"memory":"1Gi"},"Limits":{"memory":"4Gi"}}}}`,
			"Limits",
		},
		{
			`{"preset":"small","releaseRef":"approved","overrides":{"scheduling":` +
				`{"tolerations":[{"Key":"pool","value":"es"}]}}}`,
			"Key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			_, err := r.build(t.Context(), elasticsearchRequest(tt.params, nil), elasticsearch{})

			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
			assert.Contains(t, failure.Message, tt.field)
		})
	}
}

func TestElasticsearchRefusesAnnotationsThatAreNotAnObject(t *testing.T) {
	for _, value := range []string{`["a"]`, `{"a":1}`, `not json`, `null`, `{"a":null}`} {
		t.Run(value, func(t *testing.T) {
			request := elasticsearchRequest(`{"preset":"small"}`, map[string]string{"serviceAccountAnnotations": value})

			_, err := elasticsearch{}.build(request, nil)

			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
			assert.Contains(t, failure.Message, "spec.inputs.serviceAccountAnnotations")
		})
	}
}

func TestElasticsearchPinsTheServiceAccountForTheBucket(t *testing.T) {
	tests := []struct {
		name   string
		inputs map[string]string
		want   *v1.ServiceAccountSpec
	}{
		{"no inputs", nil, nil},
		{
			"bucket only",
			map[string]string{"objectStorageConfig": "orders-backup"},
			&v1.ServiceAccountSpec{Name: "orders-es-es"},
		},
		{
			"annotations only",
			map[string]string{"serviceAccountAnnotations": `{"iam.gke.io/gcp-service-account":"orders@p.iam"}`},
			&v1.ServiceAccountSpec{
				Name:        "orders-es-es",
				Annotations: map[string]string{"iam.gke.io/gcp-service-account": "orders@p.iam"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := elasticsearch{}.build(elasticsearchRequest(`{"preset":"small"}`, tt.inputs), nil)
			require.NoError(t, err)

			assert.Equal(t, tt.want, rendered(t, s).Spec.ServiceAccount)
		})
	}
}

func TestElasticsearchReportsTheResourceAndThePods(t *testing.T) {
	s, err := elasticsearch{}.build(elasticsearchRequest(`{"preset":"small"}`, nil), nil)
	require.NoError(t, err)

	rep := s.report()
	assert.Equal(
		t, []pluginsv1alpha1.ResourceRef{{
			Kind: "ElasticsearchCluster", Name: "orders-es", OnOrphan: pluginsv1alpha1.OnOrphanKept,
		}}, rep.resources,
	)
	assert.Equal(t, map[string]string{"camunda.io/elasticsearch-cluster": "orders-es"}, rep.podSelector.MatchLabels)
	assert.Equal(
		t, []pluginsv1alpha1.SummaryItem{{
			Label: "Elasticsearch", Value: "waiting", Severity: pluginsv1alpha1.SeverityWarning,
		}}, rep.summary,
	)
}

func TestElasticsearchSummary(t *testing.T) {
	volume := func(size string) v1.VolumeStatus {
		return v1.VolumeStatus{Name: "data", Capacity: resource.MustParse(size)}
	}
	tests := []struct {
		name                         string
		generation, statusGeneration int64
		ready                        *metav1.Condition
		volumes                      []v1.VolumeStatus
		want                         []pluginsv1alpha1.SummaryItem
	}{
		{
			"Ready of an older generation",
			2, 2,
			&metav1.Condition{
				Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy, ObservedGeneration: 1,
			},
			nil,
			[]pluginsv1alpha1.SummaryItem{{
				Label: "Elasticsearch", Value: "updating, 0 nodes", Severity: pluginsv1alpha1.SeverityWarning,
			}},
		},
		{
			"status of an older generation",
			2, 1,
			&metav1.Condition{
				Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy, ObservedGeneration: 2,
			},
			nil,
			[]pluginsv1alpha1.SummaryItem{{
				Label: "Elasticsearch", Value: "updating, 0 nodes", Severity: pluginsv1alpha1.SeverityWarning,
			}},
		},
		{
			"ready",
			0, 0,
			&metav1.Condition{Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy},
			[]v1.VolumeStatus{volume("50Gi"), volume("50Gi"), volume("50Gi")},
			[]pluginsv1alpha1.SummaryItem{
				{Label: "Elasticsearch", Value: "Ready, 3 nodes", Severity: pluginsv1alpha1.SeverityInfo},
				{Label: "Disk", Value: "50Gi per node", Severity: pluginsv1alpha1.SeverityInfo},
			},
		},
		{
			"not ready with a reason",
			0, 0,
			&metav1.Condition{Type: v1.ConditionReady, Status: metav1.ConditionFalse, Reason: "Creating"},
			nil,
			[]pluginsv1alpha1.SummaryItem{{
				Label: "Elasticsearch", Value: "not Ready (Creating), 0 nodes", Severity: pluginsv1alpha1.SeverityWarning,
			}},
		},
		{
			"no Ready condition",
			0, 0,
			nil,
			nil,
			[]pluginsv1alpha1.SummaryItem{{
				Label: "Elasticsearch", Value: "not Ready, 0 nodes", Severity: pluginsv1alpha1.SeverityWarning,
			}},
		},
		{
			"volumes of two sizes",
			0, 0,
			nil,
			[]v1.VolumeStatus{volume("50Gi"), volume("60Gi")},
			[]pluginsv1alpha1.SummaryItem{
				{Label: "Elasticsearch", Value: "not Ready, 2 nodes", Severity: pluginsv1alpha1.SeverityWarning},
				{Label: "Disk", Value: "60Gi per node", Severity: pluginsv1alpha1.SeverityInfo},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := v1.ElasticsearchCluster{
				ObjectMeta: metav1.ObjectMeta{Generation: tt.generation},
				Status:     v1.ElasticsearchClusterStatus{ObservedGeneration: tt.statusGeneration, Volumes: tt.volumes},
			}
			if tt.ready != nil {
				cluster.Status.Conditions = []metav1.Condition{*tt.ready}
			}

			assert.Equal(t, tt.want, elasticsearchSummary(cluster, true))
		})
	}
}

func TestElasticsearchCleanupWaitsForTheClusterOnlyWithDelete(t *testing.T) {
	request := elasticsearchRequest(`{"preset":"small"}`, nil)
	tests := []struct {
		policy        pluginsv1alpha1.DeletionPolicy
		wantRemaining []string
	}{
		{pluginsv1alpha1.DeletionPolicyDelete, []string{"orders-es"}},
		{pluginsv1alpha1.DeletionPolicyOrphan, []string{}},
	}

	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			c, err := elasticsearch{}.cleanup(request, tt.policy, nil)
			require.NoError(t, err)

			require.Len(t, c.stages, 1)
			assert.Len(t, c.stages[0].components, 1)
			remaining := make([]string, 0, len(c.stages[0].remaining))
			for _, obj := range c.stages[0].remaining {
				remaining = append(remaining, obj.GetName())
			}
			assert.Equal(t, tt.wantRemaining, remaining)
		})
	}
}

func TestElasticsearchOrphanReadsTheClusterWithoutTheCache(t *testing.T) {
	stored := elasticsearchRequest(`{"preset":"small"}`, nil)
	stored.UID = "request-uid"
	stored.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan
	stored.Finalizers = []string{pluginsv1alpha1.FinalizerCleanup}
	cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{
		Name: "orders-es", Namespace: stored.Namespace,
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRequest",
			Name: stored.Name, UID: stored.UID, Controller: new(true),
		}},
	}}
	apiServer := fakeReader(t, stored, cluster)
	// The cache has not seen the cluster yet. Writes reach the API server.
	cached := uncachedReads{Client: apiServer, reader: fakeReader(t)}
	request := &pluginsv1alpha1.PluginRequest{}
	require.NoError(t, apiServer.Get(t.Context(), client.ObjectKeyFromObject(stored), request))
	r := &Reconciler{Client: cached, APIReader: apiServer}
	rec := component.ReconcileContext{
		Client: cached, Scheme: apiServer.Scheme(), EventRecorder: events.NewFakeRecorder(10),
		APIReader: apiServer, Owner: request,
	}

	_, err := r.finalize(t.Context(), rec, request, elasticsearch{})
	require.NoError(t, err)

	var got v1.ElasticsearchCluster
	require.NoError(t, apiServer.Get(t.Context(), client.ObjectKeyFromObject(cluster), &got))
	assert.Empty(t, got.OwnerReferences)
}

// elasticsearchRequest returns an elasticsearch request of the claim orders
// with params and inputs.
func elasticsearchRequest(params string, inputs map[string]string) *pluginsv1alpha1.PluginRequest {
	request := requestFor(pluginstartup.RegistrationElasticsearch, params)
	request.Spec.ClaimRef.Name = "orders"
	request.Spec.Inputs = inputs

	return request
}

// rendered returns the ElasticsearchCluster that the backing component of s
// applies.
func rendered(t *testing.T, s served) *v1.ElasticsearchCluster {
	objs, err := s.backing.Preview()
	require.NoError(t, err)
	require.Len(t, objs, 1)
	cluster, ok := objs[0].(*v1.ElasticsearchCluster)
	require.True(t, ok)

	return cluster
}

func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

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

package camundaoptimize

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

const harnessClaimNamespace = "camunda-operator-system"

// reconcileHarness is a CamundaOptimize whose every reference resolves over a
// fake client, so a Reconcile passes the pre-check and renders. Unlike envtest,
// it can fail one write and set the status of a workload.
//
// The hooks answer the matching call in place of the fake client when they
// return an error, and pass it on when they return nil. A test sets them
// between two reconciles.
type reconcileHarness struct {
	client     client.WithWatch
	reconciler *Reconciler
	recorder   *events.FakeRecorder
	optimize   *v1.CamundaOptimize
	cluster    *v1.CamundaCluster
	key        types.NamespacedName
	// lease is the storage claim that the cluster holds.
	lease *coordinationv1.Lease
	// backend is the storage claim key of the backend of the cluster.
	backend string
	// pods are the pods that a pod list returns.
	pods []metav1.PartialObjectMetadata

	// failApply answers a server-side apply: a component apply or the exporter
	// patch.
	failApply func(obj client.Object) error
	// failPatch answers every other patch, such as the patch that stops a
	// workload on a failed check.
	failPatch func(obj client.Object) error
	// failStatusUpdate answers a write of the status subresource, which is the
	// flush at the end of every reconcile.
	failStatusUpdate func(obj client.Object) error
}

// newReconcileHarness returns a harness whose CamundaOptimize passes every
// check, the storage claim gate included, and whose first Reconcile renders.
func newReconcileHarness(t *testing.T) *reconcileHarness {
	t.Helper()
	scheme := suspendScheme(t)
	namespace := "team-a"

	binding := newSecondaryStorageConfigElasticsearch(namespace)
	credentials := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      binding.Spec.Elasticsearch.CredentialsSecretRef.Name,
			Namespace: namespace,
		},
		Data: map[string][]byte{"username": []byte("camunda"), "password": []byte("es-password")},
	}
	platform := newCamundaPlatformConfigBasic()
	auth := &v1.ManagementAuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "mac"},
		Spec: v1.ManagementAuthConfigSpec{
			BaseURL:   "http://identity." + namespace + ".svc:8080",
			IssuerURL: "https://identity.example.com/realms/camunda",
			AuthURL:   "https://identity.example.com/realms/camunda/protocol/openid-connect/auth",
			TokenURL:  "https://identity.example.com/realms/camunda/protocol/openid-connect/token",
			JwksURL:   "https://identity.example.com/realms/camunda/protocol/openid-connect/certs",
			ClientID:  "optimize",
			Audience:  "optimize-api",
			ClientSecretRef: v1.SecretKeyRef{
				Name: "mac-client", Namespace: namespace, Key: "client-secret",
			},
		},
	}
	clientSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mac-client", Namespace: namespace},
		Data:       map[string][]byte{"client-secret": []byte("s3cret")},
	}
	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: namespace, UID: "uid-c"},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: platform.Name,
			Version:           "8.9.9",
			StorageRef:        binding.Name,
		},
	}
	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "co-a",
			Namespace:  namespace,
			UID:        "uid-1",
			Generation: 1,
			Finalizers: []string{Finalizer},
		},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: auth.Name,
			ClusterRef:        v1.ClusterRef{Name: cluster.Name},
		},
	}

	key, err := clustercomponents.StorageClaimKey(clustercomponents.Storage{
		Type:          binding.Spec.Type,
		Namespace:     binding.Namespace,
		Elasticsearch: binding.Spec.Elasticsearch,
	})
	require.NoError(t, err)
	lease := clustercomponents.StorageClaimSchema().NewLease(harnessClaimNamespace, key, cluster)

	h := &reconcileHarness{
		recorder: events.NewFakeRecorder(64),
		optimize: optimize,
		cluster:  cluster,
		key:      client.ObjectKeyFromObject(optimize),
		lease:    lease,
		backend:  key,
	}
	mapper := harnessRESTMapper(scheme)
	h.client = fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(binding, credentials, platform, auth, clientSecret, cluster, optimize, lease).
		WithStatusSubresource(&v1.CamundaOptimize{}, &v1.CamundaCluster{}, &appsv1.Deployment{}).
		WithInterceptorFuncs(h.interceptors()).
		Build()
	h.reconciler = &Reconciler{
		Client:          h.client,
		APIReader:       h.client,
		Scheme:          scheme,
		EventRecorder:   h.recorder,
		ClaimNamespace:  harnessClaimNamespace,
		componentClient: h.client,
		restMapper:      mapper,
	}

	return h
}

// harnessRESTMapper serves every kind of scheme with the scope the API server
// gives it. ocf reads the scope before it sets an owner reference. The scheme
// holds no ServiceMonitor, so the render leaves that kind out.
func harnessRESTMapper(scheme *runtime.Scheme) meta.RESTMapper {
	clusterScoped := map[string]bool{
		"Namespace":                  true,
		"CamundaPlatformConfig":      true,
		"CamundaRelease":             true,
		"CamundaClusterPreset":       true,
		"ManagementAuthConfig":       true,
		"DatabaseServerPreset":       true,
		"ElasticsearchClusterPreset": true,
	}

	mapper := meta.NewDefaultRESTMapper(nil)
	for gvk := range scheme.AllKnownTypes() {
		scope := meta.RESTScopeNamespace
		if clusterScoped[gvk.Kind] {
			scope = meta.RESTScopeRoot
		}
		mapper.Add(gvk, scope)
	}

	return mapper
}

// interceptors route the writes of the fake client through the hooks of h.
//
// A pod list is answered from h.pods by namespace and labels. The claim gate
// lists pods with a field selector on the phase, which the fake client cannot
// serve, and every pod of the harness runs.
func (h *reconcileHarness) interceptors() interceptor.Funcs {
	return interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			pods, ok := list.(*metav1.PartialObjectMetadataList)
			if !ok || pods.GetObjectKind().GroupVersionKind().Kind != "PodList" {
				return c.List(ctx, list, opts...)
			}

			options := (&client.ListOptions{}).ApplyOptions(opts)
			for _, pod := range h.pods {
				if options.Namespace != "" && pod.Namespace != options.Namespace {
					continue
				}
				if options.LabelSelector != nil && !options.LabelSelector.Matches(k8slabels.Set(pod.Labels)) {
					continue
				}
				pods.Items = append(pods.Items, pod)
			}

			return nil
		},
		Patch: func(
			ctx context.Context,
			c client.WithWatch,
			obj client.Object,
			patch client.Patch,
			opts ...client.PatchOption,
		) error {
			hook := h.failPatch
			if patch.Type() == types.ApplyPatchType {
				hook = h.failApply
			}
			if hook != nil {
				if err := hook(obj); err != nil {
					return err
				}
			}

			return c.Patch(ctx, obj, patch, opts...)
		},
		SubResourceUpdate: func(
			ctx context.Context,
			c client.Client,
			subResource string,
			obj client.Object,
			opts ...client.SubResourceUpdateOption,
		) error {
			if subResource == "status" && h.failStatusUpdate != nil {
				if err := h.failStatusUpdate(obj); err != nil {
					return err
				}
			}

			return c.SubResource(subResource).Update(ctx, obj, opts...)
		},
	}
}

func (h *reconcileHarness) reconcile(t *testing.T) error {
	t.Helper()
	_, err := h.reconciler.Reconcile(t.Context(), ctrl.Request{NamespacedName: h.key})

	return err
}

// latest returns the CamundaOptimize as the fake API server holds it.
func (h *reconcileHarness) latest(t *testing.T) *v1.CamundaOptimize {
	t.Helper()
	var optimize v1.CamundaOptimize
	require.NoError(t, h.client.Get(t.Context(), h.key, &optimize))

	return &optimize
}

func (h *reconcileHarness) setClusterSuspend(t *testing.T, suspend bool) {
	t.Helper()
	var cluster v1.CamundaCluster
	require.NoError(t, h.client.Get(t.Context(), client.ObjectKeyFromObject(h.cluster), &cluster))
	cluster.Spec.Suspend = suspend
	require.NoError(t, h.client.Update(t.Context(), &cluster))
}

// releaseClaim deletes the storage claim Lease, so the cluster no longer
// holds its backend.
func (h *reconcileHarness) releaseClaim(t *testing.T) {
	t.Helper()
	require.NoError(t, h.client.Delete(t.Context(), h.lease))
}

// runForeignWriter starts a pod of another cluster that carries the storage
// claim of the backend of the harness cluster.
func (h *reconcileHarness) runForeignWriter() {
	h.pods = append(h.pods, metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-cluster-zeebe-0",
			Namespace: "team-b",
			Labels: map[string]string{
				labels.StorageClaimKey: labels.OwnerName(h.lease.Name),
				labels.ClusterUIDKey:   "uid-other",
			},
		},
	})
}

// runForeignRestore registers a restore into another cluster as a writer of
// the backend of the harness cluster.
func (h *reconcileHarness) runForeignRestore(t *testing.T) {
	t.Helper()
	writer := storagewriter.Writer{
		Kind:       "LogicalRestoreElasticsearch",
		Namespace:  "team-b",
		Name:       "restore",
		UID:        "uid-restore",
		ClusterUID: "uid-other",
	}
	require.NoError(t, storagewriter.Register(
		t.Context(), h.client, h.client, harnessClaimNamespace, h.backend, h.lease.Name, writer, time.Now(),
	))
}

func (h *reconcileHarness) workloadKey(comp string) client.ObjectKey {
	return client.ObjectKey{Namespace: h.optimize.Namespace, Name: components.WorkloadName(h.optimize, comp)}
}

func (h *reconcileHarness) workloadKeys() []client.ObjectKey {
	return []client.ObjectKey{
		h.workloadKey(components.ComponentWebapp),
		h.workloadKey(components.ComponentImporter),
	}
}

// replicas returns the replicas that the Deployment at key asks for.
func (h *reconcileHarness) replicas(t *testing.T, key client.ObjectKey) int32 {
	t.Helper()
	var deployment appsv1.Deployment
	require.NoError(t, h.client.Get(t.Context(), key, &deployment))
	require.NotNil(t, deployment.Spec.Replicas)

	return *deployment.Spec.Replicas
}

// observe stands in for the kubelet: it sets the replicas that every Optimize
// workload reports to observed, all of them ready and current.
func (h *reconcileHarness) observe(t *testing.T, observed int32) {
	t.Helper()
	for _, key := range h.workloadKeys() {
		var deployment appsv1.Deployment
		require.NoError(t, h.client.Get(t.Context(), key, &deployment))
		deployment.Status = appsv1.DeploymentStatus{
			ObservedGeneration: deployment.Generation,
			Replicas:           observed,
			ReadyReplicas:      observed,
			AvailableReplicas:  observed,
			UpdatedReplicas:    observed,
		}
		require.NoError(t, h.client.Status().Update(t.Context(), &deployment))
	}
}

// events drains the recorder and returns how many events carry each reason.
func (h *reconcileHarness) events() map[string]int {
	counts := map[string]int{}
	for {
		select {
		case recorded := <-h.recorder.Events:
			// The fake recorder writes an event as its type, its reason and its note.
			if fields := strings.Fields(recorded); len(fields) > 1 {
				counts[fields[1]]++
			}
		default:
			return counts
		}
	}
}

// start renders the workloads and reports their pods ready, so the
// CamundaOptimize is running when a test begins. It drains the events of that
// start.
func (h *reconcileHarness) start(t *testing.T) {
	t.Helper()
	require.NoError(t, h.reconcile(t))
	h.observe(t, 1)
	require.NoError(t, h.reconcile(t))

	ready := meta.FindStatusCondition(h.latest(t).Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	require.Equal(t, metav1.ConditionTrue, ready.Status, ready.Message)
	h.events()
}

// expectWorkloads asserts that the condition of every Optimize workload
// carries reason.
func (h *reconcileHarness) expectWorkloads(t *testing.T, reason string) {
	t.Helper()
	latest := h.latest(t)
	for _, conditionType := range components.ConditionTypes() {
		condition := meta.FindStatusCondition(latest.Status.Conditions, conditionType)
		if assert.NotNil(t, condition, conditionType) {
			assert.Equal(t, reason, condition.Reason, conditionType)
		}
	}
}

// suspensionEvents drains the recorder and returns the counts of the events
// that report the suspension of the cluster.
func (h *reconcileHarness) suspensionEvents() map[string]int {
	counts := map[string]int{}
	for reason, count := range h.events() {
		switch reason {
		case eventReasonClusterSuspended, eventReasonClusterResumed, eventReasonStorageClaimAwaited:
			counts[reason] = count
		}
	}

	return counts
}

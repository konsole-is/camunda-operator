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

package camundacluster

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

// A cluster gives a backend back only when none of its own pods writes it any
// more. A release under running pods lets the next claimant start beside them,
// and its own gate reads the pods once.
func TestReleaseLeftBackendsKeepsOneItsPodsStillWrite(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	self := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "orders", UID: "uid-1"},
	}
	const (
		oldKey = "elasticsearch|https://old.es:9200"
		newKey = "elasticsearch|https://new.es:9200"
	)
	oldClaim := components.StorageClaimSchema().LeaseName(oldKey)
	newClaim := components.StorageClaimSchema().LeaseName(newKey)

	pod := func(name, claim string, uid types.UID) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "team-a",
			Name:      name,
			Labels:    components.StoragePodLabels("orders", uid, claim),
		}}
	}

	replicaSet := func(name, claim string, uid types.UID, replicas int32) *appsv1.ReplicaSet {
		return &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "team-a",
				Name:      name,
				Labels:    components.StoragePodLabels("orders", uid, claim),
			},
			Spec: appsv1.ReplicaSetSpec{Replicas: &replicas},
		}
	}
	statefulSet := func(observed int64, current, update string) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  "team-a",
				Name:       "orders-zeebe",
				Labels:     labels.Managed(labels.Cluster("orders"), components.ComponentZeebe),
				Generation: 2,
			},
			Status: appsv1.StatefulSetStatus{
				ObservedGeneration: observed,
				CurrentRevision:    current,
				UpdateRevision:     update,
			},
		}
	}
	deploymentOn := func(claim string, replicas int32) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "team-a",
				Name:      "orders-gateway",
				Labels:    labels.Managed(labels.Cluster("orders"), components.ComponentGateway),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: components.StoragePodLabels("orders", self.UID, claim)},
				},
			},
		}
	}
	statefulSetOn := func(claim string, replicas int32) *appsv1.StatefulSet {
		sts := statefulSet(2, "rev-2", "rev-2")
		sts.Spec.Replicas = &replicas
		sts.Spec.Template.Labels = components.StoragePodLabels("orders", self.UID, claim)

		return sts
	}

	cases := map[string]struct {
		pods     []client.Object
		heldBack []string
		released bool
	}{
		"a pod of this cluster still carries the old claim": {
			pods:     []client.Object{pod("orders-zeebe-0", oldClaim, self.UID)},
			heldBack: []string{oldClaim},
		},
		// A ReplicaSet that asks for replicas recreates a pod that goes, with
		// the claim its template carries, so the old claim stays held until
		// the rollout scales it to zero.
		"an old ReplicaSet of this cluster still asks for a pod": {
			pods:     []client.Object{replicaSet("orders-gateway-old", oldClaim, self.UID, 1)},
			heldBack: []string{oldClaim},
		},
		"an old ReplicaSet of this cluster is scaled to zero": {
			pods:     []client.Object{replicaSet("orders-gateway-old", oldClaim, self.UID, 0)},
			released: true,
		},
		"an old ReplicaSet of another cluster asks for a pod": {
			pods:     []client.Object{replicaSet("other-gateway-old", oldClaim, "uid-other", 1)},
			released: true,
		},
		// A StatefulSet mid-update recreates a pod with the revision it had,
		// which the list cannot name, so every release waits for the update.
		"a StatefulSet of this cluster is mid-update": {
			pods:     []client.Object{statefulSet(2, "rev-1", "rev-2")},
			heldBack: []string{oldClaim},
		},
		"the controller has not read the latest StatefulSet of this cluster": {
			pods:     []client.Object{statefulSet(1, "rev-1", "rev-1")},
			heldBack: []string{oldClaim},
		},
		"the StatefulSet of this cluster is up to date": {
			pods:     []client.Object{statefulSet(2, "rev-2", "rev-2")},
			released: true,
		},
		// No StatefulSet controller has read this one, so it has no earlier
		// revision to recreate a pod from.
		"the StatefulSet of this cluster was never observed": {
			pods:     []client.Object{statefulSet(0, "", "")},
			released: true,
		},
		// A pass that applied no render leaves the template on the old claim,
		// and a StatefulSet that asks for pods recreates one from it.
		"the StatefulSet template of this cluster still carries the old claim": {
			pods:     []client.Object{statefulSetOn(oldClaim, 1)},
			heldBack: []string{oldClaim},
		},
		"the StatefulSet of this cluster is at zero on the old claim": {
			pods:     []client.Object{statefulSetOn(oldClaim, 0)},
			released: true,
		},
		// A Deployment with no ReplicaSet for a moment creates the next one
		// from its template.
		"the Deployment template of this cluster still carries the old claim": {
			pods:     []client.Object{deploymentOn(oldClaim, 1)},
			heldBack: []string{oldClaim},
		},
		"the Deployment of this cluster is at zero on the old claim": {
			pods:     []client.Object{deploymentOn(oldClaim, 0)},
			released: true,
		},
		"the pods of this cluster carry the new claim": {
			pods:     []client.Object{pod("orders-zeebe-0", newClaim, self.UID)},
			released: true,
		},
		"a pod of another cluster carries the old claim": {
			pods:     []client.Object{pod("other-zeebe-0", oldClaim, "uid-other")},
			released: true,
		},
		"no pods": {released: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			objects := append(
				[]client.Object{
					components.StorageClaimSchema().NewLease("camunda-system", oldKey, self),
					components.StorageClaimSchema().NewLease("camunda-system", newKey, self),
				},
				tc.pods...,
			)
			c := storageClaimPodClient(t, scheme, objects...)

			r := &CamundaClusterReconciler{Client: c, APIReader: c, ClaimNamespace: "camunda-system"}

			heldBack, err := r.releaseLeftBackends(context.Background(), self, newClaim)

			require.NoError(t, err)
			assert.Equal(t, tc.heldBack, heldBack)
			var lease coordinationv1.Lease
			err = c.Get(
				context.Background(),
				client.ObjectKey{Namespace: "camunda-system", Name: oldClaim},
				&lease,
			)
			assert.Equal(t, tc.released, apierrors.IsNotFound(err), "the claim of the old backend")
		})
	}
}

// A refused cluster applies nothing, so the claim it took on the pass is not
// the backend it writes. It keeps the backend its pods write and gives the
// fresh one back, so a cluster that repointed into a refusal blocks no one on
// a backend it never writes. A cluster with no pod keeps what it took.
func TestReleaseRefusedWaitKeepsTheBackendThePodsWrite(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	self := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "orders", UID: "uid-1"},
	}
	const (
		oldKey = "elasticsearch|https://old.es:9200"
		newKey = "elasticsearch|https://new.es:9200"
	)
	oldClaim := components.StorageClaimSchema().LeaseName(oldKey)
	newClaim := components.StorageClaimSchema().LeaseName(newKey)
	pod := func(claim string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "team-a",
			Name:      "orders-zeebe-0",
			Labels:    components.StoragePodLabels("orders", self.UID, claim),
		}}
	}

	cases := map[string]struct {
		pods     []client.Object
		released []string
		kept     []string
	}{
		"the pods write the backend the cluster took": {
			pods:     []client.Object{pod(newClaim)},
			released: []string{oldClaim},
			kept:     []string{newClaim},
		},
		"the pods write the backend the cluster left": {
			pods:     []client.Object{pod(oldClaim)},
			released: []string{newClaim},
			kept:     []string{oldClaim},
		},
		"no pod": {
			released: []string{oldClaim},
			kept:     []string{newClaim},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			objects := append(
				[]client.Object{
					components.StorageClaimSchema().NewLease("camunda-system", oldKey, self),
					components.StorageClaimSchema().NewLease("camunda-system", newKey, self),
				},
				tc.pods...,
			)
			c := storageClaimPodClient(t, scheme, objects...)
			r := &CamundaClusterReconciler{Client: c, APIReader: c, ClaimNamespace: "camunda-system"}

			wait, err := r.releaseRefusedWait(context.Background(), self, newClaim)

			require.NoError(t, err)
			assert.Zero(t, wait, "a refused cluster holds nothing back that a pod of its does not write")
			for _, claim := range tc.released {
				var lease coordinationv1.Lease
				err := c.Get(context.Background(), client.ObjectKey{Namespace: "camunda-system", Name: claim}, &lease)
				assert.True(t, apierrors.IsNotFound(err), "%s is released", claim)
			}
			for _, claim := range tc.kept {
				var lease coordinationv1.Lease
				err := c.Get(context.Background(), client.ObjectKey{Namespace: "camunda-system", Name: claim}, &lease)
				assert.NoError(t, err, "%s is kept", claim)
			}
		})
	}
}

// The gate exists for the takeover moment. A cluster whose own pods already
// write the backend it holds is past that moment, so it does not pay for the
// list. A status write that never landed must not end a wait, so the pods of
// this cluster decide it and not the reason it last reported.
func TestHandoverPossible(t *testing.T) {
	cases := map[string]struct {
		suspended     bool
		heldAtStart   bool
		ownPodOnClaim bool
		possible      bool
	}{
		"this pass took the claim":                  {heldAtStart: false, possible: true},
		"this pass took a claim its pods carry":     {heldAtStart: false, ownPodOnClaim: true, possible: true},
		"no pod of this cluster writes the backend": {heldAtStart: true, possible: true},
		"the pods of this cluster write it":         {heldAtStart: true, ownPodOnClaim: true, possible: false},
		// A suspended cluster writes nothing, so it has no handover to wait
		// for, and its Ready keeps the reason the user asked for.
		"the user suspended the cluster":                    {suspended: true, heldAtStart: true},
		"a suspended cluster that took the claim this pass": {suspended: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(
				t, tc.possible, handoverPossible(tc.suspended, tc.heldAtStart, tc.ownPodOnClaim),
			)
		})
	}
}

// A free backend is not free while pods of another cluster write it. Without
// this rule, a parked cluster that finds the Lease gone creates it, and the
// running holder then meets a blocker and scales to zero.
//
// The cluster that waits takes no Lease and renders at zero, which is what
// keeps it off the backend those pods write.
func TestClaimStorageWaitsUnderThePodsOnAFreeBackend(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	storage := components.Storage{
		Type:          v1.SecondaryStorageTypeElasticsearch,
		Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es.data.svc:9200"},
	}
	key, err := components.StorageClaimKey(storage)
	require.NoError(t, err)
	claim := components.StorageClaimSchema().LeaseName(key)
	self := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "parked", UID: "uid-parked"},
	}
	pod := func(name string, podLabels map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "apps", Labels: podLabels}}
	}

	foreign := []client.Object{
		pod("holder-zeebe-0", components.StoragePodLabels("holder", "uid-holder", claim)),
	}

	cases := map[string]struct {
		pods      []client.Object
		suspended bool
		waits     bool
		reports   bool
	}{
		"pods of the running holder": {pods: foreign, waits: true, reports: true},
		"its own pods": {
			pods: []client.Object{pod("parked-zeebe-0", components.StoragePodLabels("parked", self.UID, claim))},
		},
		"no pods": {},
		// A suspended cluster writes nothing beside those pods, but a Lease it
		// created under them would park the running cluster they belong to.
		// It waits without a report: it is at zero already, and Ready keeps
		// the reason the user asked for.
		"a suspended cluster under the pods of another": {pods: foreign, suspended: true, waits: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := &components.Input{Storage: storage}
			in.Effective = components.NewEffective(v1.CamundaClusterSpec{Suspend: tc.suspended})
			c := storageClaimPodClient(t, scheme, tc.pods...)
			res := &resolver{
				reader:  c,
				claims:  components.StorageClaimSchema().NewClaim(c, c, "camunda-system"),
				cluster: self,
				storage: &v1.SecondaryStorageConfig{
					ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "storage"},
				},
				recorder: events.NewFakeRecorder(10),
			}

			err := res.claimStorage(context.Background(), in)

			var lease coordinationv1.Lease
			read := c.Get(
				context.Background(),
				client.ObjectKey{Namespace: "camunda-system", Name: claim},
				&lease,
			)
			require.NoError(t, err)
			if !tc.waits {
				assert.Nil(t, in.Storage.Handover)
				assert.NoError(t, read, "the cluster took the free backend")

				return
			}

			assert.True(t, apierrors.IsNotFound(read), "no Lease is written under the pods of another cluster")
			if !tc.reports {
				assert.Nil(t, in.Storage.Handover, "a suspended cluster keeps the Ready reason the user asked for")

				return
			}

			// The wait is no pre-check failure. It renders the cluster at zero,
			// the way a handover on a claim it holds does, so its workloads
			// stop writing the backend they are on.
			require.NotNil(t, in.Storage.Handover)
			assert.Equal(t, key, in.Storage.Handover.Backend)
			assert.Contains(t, in.Storage.Handover.Pods, "apps/holder-zeebe-0")
		})
	}
}

// The claim step waits for a live writer for another cluster, on a free backend
// and on one this cluster holds alike.
func TestClaimStorageWaitsForALiveWriterForAnotherCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	storage := components.Storage{
		Type:          v1.SecondaryStorageTypeElasticsearch,
		Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es.data.svc:9200"},
	}
	key, err := components.StorageClaimKey(storage)
	require.NoError(t, err)
	self := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "next", UID: "uid-next"},
	}
	restore := func(kind string, target types.UID) storagewriter.Writer {
		return storagewriter.Writer{
			Kind:       kind,
			Namespace:  "apps",
			Name:       "restore",
			UID:        "uid-restore",
			ClusterUID: target,
		}
	}

	cases := map[string]struct {
		backend string
		writer  storagewriter.Writer
		held    bool
		waitsOn string
	}{
		"an Elasticsearch restore into another cluster": {
			writer:  restore("LogicalRestoreElasticsearch", "uid-old"),
			waitsOn: "LogicalRestoreElasticsearch apps/restore",
		},
		"an RDBMS restore into another cluster": {
			writer:  restore("LogicalRestoreRDBMS", "uid-old"),
			waitsOn: "LogicalRestoreRDBMS apps/restore",
		},
		"a writer on a backend this cluster already took": {
			writer:  restore("LogicalRestoreRDBMS", "uid-old"),
			held:    true,
			waitsOn: "LogicalRestoreRDBMS apps/restore",
		},
		"a writer on another backend": {
			backend: "elasticsearch|https://other:9200",
			writer:  restore("LogicalRestoreElasticsearch", "uid-old"),
		},
		"a writer for this cluster": {
			writer: restore("LogicalRestoreElasticsearch", self.UID),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := &components.Input{Storage: storage}
			in.Effective = components.NewEffective(v1.CamundaClusterSpec{})
			var objects []client.Object
			if tc.held {
				objects = append(objects, components.StorageClaimSchema().NewLease("camunda-system", key, self))
			}
			c := storageClaimPodClient(t, scheme, objects...)
			backend := key
			if tc.backend != "" {
				backend = tc.backend
			}
			claim := components.StorageClaimSchema().LeaseName(backend)
			require.NoError(
				t,
				storagewriter.Register(
					context.Background(),
					c,
					c,
					"camunda-system",
					backend,
					claim,
					tc.writer,
				),
			)
			res := &resolver{
				reader:         c,
				claims:         components.StorageClaimSchema().NewClaim(c, c, "camunda-system"),
				claimNamespace: "camunda-system",
				cluster:        self,
				storage: &v1.SecondaryStorageConfig{
					ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "storage"},
				},
				recorder: events.NewFakeRecorder(10),
			}

			require.NoError(t, res.claimStorage(context.Background(), in))

			lease := &coordinationv1.Lease{}
			err := c.Get(
				context.Background(),
				types.NamespacedName{
					Namespace: "camunda-system",
					Name:      storagewriter.LeaseName(backend, tc.writer.UID),
				},
				lease,
			)
			assert.NoError(t, err, "a cluster never removes a writer Lease")

			if tc.waitsOn == "" {
				assert.Nil(t, in.Storage.Handover)

				return
			}
			require.NotNil(t, in.Storage.Handover)
			assert.Equal(t, []string{tc.waitsOn}, in.Storage.Handover.Writers)
			assert.Empty(t, in.Storage.Handover.Pods)
		})
	}
}

// A pre-check that fails before the claim step knows no backend of this
// cluster, so the pods name it: the claim they carry is kept, and a backend
// the cluster left goes back. A cluster whose pods are gone releases nothing,
// because a waiting cluster would take its live backend over a reference that
// says nothing about it.
func TestReportFailedPreCheckKeepsTheBackendThePodsWrite(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	const key = "elasticsearch|https://es.data.svc:9200"
	self := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "orders", UID: "uid-1"},
	}
	claim := components.StorageClaimSchema().LeaseName(key)
	pod := func(claim string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "apps",
			Name:      "orders-zeebe-0",
			Labels:    components.StoragePodLabels("orders", self.UID, claim),
		}}
	}

	cases := map[string]struct {
		claim    string
		pods     []client.Object
		released bool
	}{
		"the pre-check failed before the claim step, no pod": {claim: ""},
		"the pre-check failed before the claim step, the pods write it": {
			claim: "", pods: []client.Object{pod(claim)},
		},
		"the pre-check failed before the claim step, the pods left it": {
			claim: "", pods: []client.Object{pod("camunda-storage-other")}, released: true,
		},
		"the claim step ran and named this backend": {claim: claim},
		"the claim step ran and named another":      {claim: "camunda-storage-other", released: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := storageClaimPodClient(
				t, scheme, append(
					[]client.Object{components.StorageClaimSchema().NewLease("camunda-system", key, self)},
					tc.pods...,
				)...,
			)
			r := &CamundaClusterReconciler{Client: c, APIReader: c, ClaimNamespace: "camunda-system"}
			in := components.Input{Cluster: self, Storage: components.Storage{Claim: tc.claim}}

			_, err := r.reportFailedPreCheck(
				context.Background(),
				self.DeepCopy(),
				in,
				&conditions.PreCheckFailure{Reason: v1.ReasonInvalidReference, Message: "gone"},
			)

			require.NoError(t, err)
			read := c.Get(
				context.Background(),
				client.ObjectKey{Namespace: "camunda-system", Name: claim},
				&coordinationv1.Lease{},
			)
			assert.Equal(t, tc.released, apierrors.IsNotFound(read), "the backend this cluster writes")
		})
	}
}

// A Lease that carries the name of a storage claim and no holder annotations
// is somebody else's. No cluster holds the backend, so the failure is a report
// and not a suspension. Nothing watches that Lease for this cluster, so the
// failure must be unwatched: its deletion is picked up by the retry timer.
func TestClaimStorageReportsAForeignLeaseAsUnwatched(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	in := &components.Input{Storage: components.Storage{
		Type:          v1.SecondaryStorageTypeElasticsearch,
		Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es.data.svc:9200"},
	}}
	key, err := components.StorageClaimKey(in.Storage)
	require.NoError(t, err)
	foreign := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
		Namespace: "camunda-system",
		Name:      components.StorageClaimSchema().LeaseName(key),
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(foreign).Build()

	res := &resolver{
		reader:  c,
		claims:  components.StorageClaimSchema().NewClaim(c, c, "camunda-system"),
		cluster: &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "orders", UID: "uid-1"}},
		storage: &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "storage"},
		},
	}

	err = res.claimStorage(context.Background(), in)

	var unwatched *conditions.UnwatchedPreCheckFailure
	require.ErrorAs(t, err, &unwatched)
	assert.Equal(t, v1.ReasonInvalidReference, unwatched.Failure.Reason)
	assert.Contains(t, unwatched.Failure.Message, foreign.Name)
	assert.Nil(t, in.Storage.Holder, "no cluster holds the backend, so the cluster is not suspended")
}

// TestStorageHeld covers the Ready condition storageHeld builds for a
// suspended cluster, with and without an apply error, and that the result
// keeps CamundaCluster.Suspended true either way.
func TestStorageHeld(t *testing.T) {
	holder := &components.StorageHolder{
		Cluster: types.NamespacedName{Namespace: "ns", Name: "holder"},
		Backend: "elasticsearch|https://es:9200",
	}
	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
	}

	t.Run("without an apply error", func(t *testing.T) {
		cond := storageHeld(cluster, holder, nil)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, v1.ReasonStorageAlreadyAttached, cond.Reason)
		assert.Equal(t, cluster.Generation, cond.ObservedGeneration)
		assert.Contains(t, cond.Message, "ns/holder")
		assert.Contains(t, cond.Message, "elasticsearch|https://es:9200")
		assert.NotContains(t, cond.Message, "failed")
	})

	t.Run("with an apply error", func(t *testing.T) {
		cond := storageHeld(cluster, holder, errors.New("apply exploded"))
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, v1.ReasonStorageAlreadyAttached, cond.Reason)
		assert.Contains(t, cond.Message, "The last apply of the suspended workloads failed: apply exploded")
	})

	t.Run("the gate", func(t *testing.T) {
		for _, applyErr := range []error{nil, errors.New("apply exploded")} {
			gated := &v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Spec:       v1.CamundaClusterSpec{Suspend: false},
			}
			cond := storageHeld(gated, holder, applyErr)
			meta.SetStatusCondition(&gated.Status.Conditions, cond)
			assert.True(t, gated.Suspended())
		}
	})
}

// TestStorageHandover covers the Ready condition storageHandover builds for a
// cluster that holds the claim and waits for the pods of another cluster, with
// and without an apply error, and that the result keeps
// CamundaCluster.Suspended true either way.
func TestStorageHandover(t *testing.T) {
	handover := &components.StorageHandover{
		Backend: "elasticsearch|https://es:9200",
		Pods:    []string{"ns/old-zeebe-0", "ns/old-zeebe-1"},
	}
	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Generation: 3}}

	t.Run("without an apply error", func(t *testing.T) {
		cond := storageHandover(cluster, handover, nil)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, v1.ReasonWaitingForHandover, cond.Reason)
		assert.Equal(t, cluster.Generation, cond.ObservedGeneration)
		assert.Contains(t, cond.Message, "elasticsearch|https://es:9200")
		assert.Contains(t, cond.Message, "ns/old-zeebe-0, ns/old-zeebe-1")
		assert.NotContains(t, cond.Message, "failed")
	})

	t.Run("with a restore beside the pods", func(t *testing.T) {
		withRestore := *handover
		withRestore.Writers = []string{"LogicalRestoreRDBMS ns/restore"}
		cond := storageHandover(cluster, &withRestore, nil)
		assert.Contains(t, cond.Message, "ns/old-zeebe-0, ns/old-zeebe-1")
		assert.Contains(t, cond.Message, "Writers for another cluster still write the backend")
		assert.Contains(t, cond.Message, "LogicalRestoreRDBMS ns/restore")
	})

	t.Run("with a restore alone", func(t *testing.T) {
		alone := &components.StorageHandover{Backend: handover.Backend, Writers: []string{"LogicalRestoreRDBMS ns/r"}}
		cond := storageHandover(cluster, alone, nil)
		assert.NotContains(t, cond.Message, "Pods of another cluster")
		assert.Contains(t, cond.Message, "LogicalRestoreRDBMS ns/r. This cluster starts when they are gone")
	})

	t.Run("with an apply error", func(t *testing.T) {
		cond := storageHandover(cluster, handover, errors.New("apply exploded"))
		assert.Equal(t, v1.ReasonWaitingForHandover, cond.Reason)
		assert.Contains(t, cond.Message, "The last apply of the suspended workloads failed: apply exploded")
	})

	t.Run("the gate", func(t *testing.T) {
		for _, applyErr := range []error{nil, errors.New("apply exploded")} {
			gated := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Generation: 3}}
			cond := storageHandover(gated, handover, applyErr)
			meta.SetStatusCondition(&gated.Status.Conditions, cond)
			assert.True(t, gated.Suspended())
		}
	})
}

// storageClaimPodClient builds a fake client for the handover gate. The fake
// client refuses a "!=" field selector, which the API server serves, so the
// interceptor asserts that the phase selector reached it and then drops it. An
// envtest spec covers the filtering itself.
func storageClaimPodClient(t *testing.T, scheme *runtime.Scheme, objects ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(
				ctx context.Context,
				c client.WithWatch,
				list client.ObjectList,
				opts ...client.ListOption,
			) error {
				if _, ours := list.(*metav1.PartialObjectMetadataList); !ours {
					return c.List(ctx, list, opts...)
				}

				kept := make([]client.ListOption, 0, len(opts))
				var phases string
				for _, opt := range opts {
					if selector, ok := opt.(client.MatchingFieldsSelector); ok {
						phases = selector.String()

						continue
					}
					kept = append(kept, opt)
				}
				assert.Equal(t, "status.phase!=Failed,status.phase!=Succeeded", phases)

				return c.List(ctx, list, kept...)
			},
		}).
		Build()
}

// newNamedCluster is newCluster with a name prefix, so a spec that creates
// two clusters on one contract can tell them apart in the output.
func newNamedCluster(
	prefix, namespace string,
	cfg *v1.CamundaPlatformConfig,
	binding *v1.SecondaryStorageConfig,
) *v1.CamundaCluster {
	cluster := newCluster(namespace, cfg, binding)
	cluster.Name = prefix + utilrand.String(8)
	return cluster
}

// expectClaimedBy polls until the storage claim of the backend that binding
// names records cluster.
func expectClaimedBy(binding *v1.SecondaryStorageConfig, cluster *v1.CamundaCluster) {
	GinkgoHelper()
	key := storageKeyOf(binding)
	Eventually(func(g Gomega) {
		var lease coordinationv1.Lease
		name := client.ObjectKey{
			Namespace: testClaimNamespace,
			Name:      components.StorageClaimSchema().LeaseName(key),
		}
		g.Expect(k8sClient.Get(ctx, name, &lease)).To(Succeed())
		holder, ours := components.StorageClaimSchema().HolderOf(&lease)
		g.Expect(ours).To(BeTrue())
		g.Expect(holder.NamespacedName).To(Equal(client.ObjectKeyFromObject(cluster)))
		g.Expect(holder.UID).To(Equal(cluster.UID))
	}, timeout, interval).Should(Succeed())
}

// storageKeyOf returns the claim key of the backend that binding names. The
// test bindings are Elasticsearch ones.
func storageKeyOf(binding *v1.SecondaryStorageConfig) string {
	GinkgoHelper()
	key, err := components.StorageClaimKey(components.Storage{
		Type:          binding.Spec.Type,
		Elasticsearch: binding.Spec.Elasticsearch,
	})
	Expect(err).NotTo(HaveOccurred())

	return key
}

// expectParked polls until cluster reports StorageAlreadyAttached naming
// holder, and its broker StatefulSet exists with zero replicas.
func expectParked(cluster, holder *v1.CamundaCluster) {
	GinkgoHelper()
	expectReady(
		cluster,
		metav1.ConditionFalse,
		Equal(v1.ReasonStorageAlreadyAttached),
		ContainSubstring(holder.Namespace+"/"+holder.Name),
	)
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Eventually(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
	}, timeout, interval).Should(Succeed())
}

// createStoragePod creates a pod that carries the storage labels of cluster
// on the backend of binding, the way a rendered pod of that cluster does.
// envtest runs no kubelet, so a pod of the holder must be made by hand. It is
// deleted with the namespace.
func createStoragePod(cluster *v1.CamundaCluster, binding *v1.SecondaryStorageConfig) *corev1.Pod {
	GinkgoHelper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-zeebe-0",
			Namespace: cluster.Namespace,
			Labels: components.StoragePodLabels(
				cluster.Name,
				cluster.UID,
				components.StorageClaimSchema().LeaseName(storageKeyOf(binding)),
			),
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "camunda", Image: "camunda/camunda:8.9.0"}}},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
	return pod
}

// expectWaitingForHandover polls until cluster reports WaitingForHandover
// naming the backend of binding and pod, and checks that its broker
// StatefulSet stays at zero replicas meanwhile.
func expectWaitingForHandover(cluster *v1.CamundaCluster, binding *v1.SecondaryStorageConfig, pod *corev1.Pod) {
	GinkgoHelper()
	expectReady(
		cluster,
		metav1.ConditionFalse,
		Equal(v1.ReasonWaitingForHandover),
		And(ContainSubstring(storageKeyOf(binding)), ContainSubstring(pod.Name)),
	)
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Consistently(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
	}, "2s", interval).Should(Succeed(), "a cluster that waits for the handover renders nothing")
}

// createBindingAt creates an Elasticsearch contract in namespace that names
// endpoint, so two contracts can name one backend. createBinding gives every
// contract an endpoint of its own.
func createBindingAt(namespace, endpoint string) *v1.SecondaryStorageConfig {
	GinkgoHelper()
	binding := newSecondaryStorageConfigElasticsearch(namespace)
	binding.Spec.Elasticsearch.Endpoint = endpoint
	Expect(k8sClient.Create(ctx, binding)).To(Succeed())
	createSecret(namespace, binding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
		"username": "camunda", "password": "es-password",
	})

	return binding
}

// createStorageLeaseFor writes a storage claim Lease that records holder, the
// way a cluster that is gone leaves one behind.
func createStorageLeaseFor(key string, holder *v1.CamundaCluster) {
	GinkgoHelper()
	Expect(k8sClient.Create(
		ctx, components.StorageClaimSchema().NewLease(testClaimNamespace, key, holder),
	)).To(Succeed())
}

// expectHolds polls until cluster reports a Ready reason other than
// StorageAlreadyAttached, and its broker StatefulSet asks for one replica.
func expectHolds(cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Reason).NotTo(Equal(v1.ReasonStorageAlreadyAttached))
	}, timeout, interval).Should(Succeed())
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Eventually(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(Equal(int32(1)))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("CamundaCluster secondary storage contract", func() {
	// One CamundaCluster uses one contract, so the first cluster that claims
	// it holds it and the other is suspended, with its volumes, until the
	// holder releases it.
	It("lets the first cluster hold a contract and suspends the second", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)

		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(first)
		expectClaimedBy(binding, first)

		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(second)

		expectParked(second, first)
		expectReady(
			second,
			metav1.ConditionFalse,
			Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring(binding.Name),
		)
		expectHolds(first)
	})

	// Two contracts that name one address are one backend. The second cluster
	// parks on the claim of the first.
	It("parks the second cluster when two contracts name one address", func() {
		ns := newNamespace()
		first := newNamedCluster(
			"cc-a-", ns, createPlatformConfig(), createBindingAt(ns, "https://shared.es:9200"),
		)
		createCluster(first)
		expectHolds(first)

		binding := createBindingAt(ns, "https://SHARED.es:9200/")
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(second)
		expectParked(second, first)
		expectReady(
			second,
			metav1.ConditionFalse,
			Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring("elasticsearch|https://shared.es:9200"),
		)
	})

	It("lets two clusters on two contracts both run", func() {
		ns := newNamespace()
		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(first)
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(second)

		expectHolds(first)
		expectHolds(second)
	})

	// Nothing watches the holder for the parked cluster, so the takeover
	// happens on the retry timer.
	It("resumes the parked cluster when the holder is deleted", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		Expect(k8sClient.Create(ctx, holder)).To(Succeed())
		expectClaimedBy(binding, holder)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(holder), &v1.CamundaCluster{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		expectHolds(parked)
		expectClaimedBy(binding, parked)
	})

	// The pods of a deleted holder go through garbage collection after the
	// cluster is gone. The parked cluster waits for them, so the two never
	// write the backend at once.
	It("waits for the pods of a deleted holder before it resumes", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		Expect(k8sClient.Create(ctx, holder)).To(Succeed())
		expectClaimedBy(binding, holder)
		pod := createStoragePod(holder, binding)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
		// The deleted cluster gives the backend back before it goes, so no
		// claim of it is left for the next claimant to clear.
		claim := client.ObjectKey{
			Namespace: testClaimNamespace,
			Name:      components.StorageClaimSchema().LeaseName(storageKeyOf(binding)),
		}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, claim, &coordinationv1.Lease{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(holder), &v1.CamundaCluster{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		// The pods it left behind keep the parked cluster off the free
		// backend, so the two never write it at once.
		expectWaitingForHandover(parked, binding, pod)

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		expectHolds(parked)
		expectClaimedBy(binding, parked)
	})

	// The pods of a repointed holder write the old backend until its rollout
	// replaces them, so the holder keeps that backend until they are gone. The
	// parked cluster waits on the claim, not on the pods: the holder still
	// holds the backend it left.
	It("keeps the backend of a repointed holder while its pods still write it", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		expectClaimedBy(binding, holder)
		pod := createStoragePod(holder, binding)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		other := createBinding(ns, true)
		updateCluster(holder, func(c *v1.CamundaCluster) { c.Spec.StorageRef = other.Name })
		expectHolds(holder)
		expectClaimedBy(other, holder)
		expectParked(parked, holder)
		expectClaimedBy(binding, holder)

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		expectHolds(parked)
		expectClaimedBy(binding, parked)
	})

	// A holder that names another contract releases this one, so the parked
	// cluster takes the claim over and both clusters run.
	It("resumes the parked cluster when the holder repoints its storageRef", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		expectClaimedBy(binding, holder)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		other := createBinding(ns, true)
		updateCluster(holder, func(c *v1.CamundaCluster) { c.Spec.StorageRef = other.Name })

		expectHolds(parked)
		expectClaimedBy(binding, parked)
		expectHolds(holder)
		expectClaimedBy(other, holder)
	})

	// One edit that repoints a cluster to a held contract and lowers its
	// version parks it on the version it runs. The refusal of the downgrade
	// waits until the contract is released, so the parking never applies the
	// lower image.
	It("parks a repointed cluster whose version is also refused, and refuses it again once released", func() {
		ns := newNamespace()
		own := createBinding(ns, true)
		repointed := newNamedCluster("cc-a-", ns, createPlatformConfig(), own)
		createCluster(repointed)
		expectClaimedBy(own, repointed)
		Expect(zeebeContainer(repointed).Image).To(HaveSuffix(":" + runningVersion))

		held := createBinding(ns, true)
		holder := newNamedCluster("cc-b-", ns, createPlatformConfig(), held)
		createCluster(holder)
		expectClaimedBy(held, holder)

		By("repointing to the held contract and lowering the version in one edit")
		updateCluster(repointed, func(c *v1.CamundaCluster) {
			c.Spec.StorageRef = held.Name
			c.Spec.Version = lowerVersion
		})
		expectParked(repointed, holder)
		Consistently(func() string { return zeebeContainer(repointed).Image }, "2s", interval).Should(
			HaveSuffix(":"+runningVersion),
			"the parking keeps the running image. A lower image means the parking applied it",
		)
		Consistently(func(g Gomega) {
			g.Expect(countEvents(g, repointed, v1.ReasonVersionDowngradeRefused)).To(BeZero())
		}, "2s", interval).Should(Succeed(), "a parked cluster does not report the refusal yet")

		By("releasing the contract")
		Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
		expectReady(
			repointed, metav1.ConditionFalse,
			Equal(v1.ReasonVersionDowngradeRefused),
			ContainSubstring(lowerVersion+" is below the running version "+runningVersion),
		)
		expectEvent(repointed, v1.ReasonVersionDowngradeRefused, corev1.EventTypeWarning)
		Expect(zeebeContainer(repointed).Image).To(HaveSuffix(":" + runningVersion))

		By("setting the version forward again")
		updateCluster(repointed, func(c *v1.CamundaCluster) { c.Spec.Version = runningVersion })
		expectHolds(repointed)
		expectClaimedBy(held, repointed)
	})

	// A cluster whose contract is deleted keeps writing the backend it
	// resolved on its last pass, so it keeps its workloads and its claim on
	// that backend. Ready reports the dangling reference.
	It("keeps a running cluster on its workloads when its contract goes away", func() {
		ns := newNamespace()
		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(first)
		binding := createBinding(ns, true)
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(second)
		expectHolds(first)
		expectHolds(second)

		By("deleting the contract of the second cluster")
		Expect(k8sClient.Delete(ctx, binding)).To(Succeed())
		expectReady(
			second,
			metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference),
			ContainSubstring(binding.Name),
		)
		zeebeKey := client.ObjectKey{Namespace: ns, Name: second.Name + "-zeebe"}
		Consistently(func(g Gomega) {
			g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(
				Equal(int32(1)), "the broker StatefulSet keeps its replicas",
			)

			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(second), &latest)).To(Succeed())
			g.Expect(latest.Status.Gateway).NotTo(BeNil(), "a running cluster keeps publishing its endpoints")
			g.Expect(latest.Status.Management).NotTo(BeNil())
		}, "3s", interval).Should(Succeed())
		expectHolds(first)
		// The claim outlives the contract that named the backend. The brokers
		// of the second cluster still write it, so it stays theirs.
		expectClaimedBy(binding, second)

		By("parking a third cluster that names the same backend")
		third := newNamedCluster(
			"cc-c-", ns, createPlatformConfig(), createBindingAt(ns, binding.Spec.Elasticsearch.Endpoint),
		)
		createCluster(third)
		expectParked(third, second)

		By("recreating the contract")
		recreated := &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: binding.Name, Namespace: ns},
			Spec:       *binding.Spec.DeepCopy(),
		}
		Expect(k8sClient.Create(ctx, recreated)).To(Succeed())
		expectHolds(second)
		expectClaimedBy(recreated, second)
	})

	// A running cluster that repoints into a backend whose previous holder
	// left pods behind must stop. It holds the claim of that backend, and its
	// own workloads still write the backend it left.
	It("scales a running cluster to zero while it waits for a handover", func() {
		ns := newNamespace()
		own := createBinding(ns, true)
		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), own)
		createCluster(cluster)
		expectHolds(cluster)

		target := createBinding(ns, true)
		ghost := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"},
		}
		createStorageLeaseFor(storageKeyOf(target), ghost)
		pod := createStoragePod(ghost, target)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.StorageRef = target.Name })
		expectWaitingForHandover(cluster, target, pod)

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		expectHolds(cluster)
		expectClaimedBy(target, cluster)
	})

	// The same wait on a backend that no Lease holds. The cluster takes no
	// claim under those pods, and it must stop all the same: its own workloads
	// still write the backend it left.
	It("scales a running cluster to zero under the pods on an unclaimed backend", func() {
		ns := newNamespace()
		own := createBinding(ns, true)
		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), own)
		createCluster(cluster)
		expectHolds(cluster)

		target := createBinding(ns, true)
		ghost := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"},
		}
		pod := createStoragePod(ghost, target)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.StorageRef = target.Name })
		expectWaitingForHandover(cluster, target, pod)
		var lease coordinationv1.Lease
		name := client.ObjectKey{
			Namespace: testClaimNamespace,
			Name:      components.StorageClaimSchema().LeaseName(storageKeyOf(target)),
		}
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, name, &lease))).To(
			BeTrue(), "the backend stays unclaimed while those pods write it",
		)

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		expectHolds(cluster)
		expectClaimedBy(target, cluster)
	})

	// A repoint holds two claims for one pass. The old one must go, or the
	// next cluster on that backend parks on a claim nothing writes.
	It("releases the old backend when the holder repoints and the new one resolves", func() {
		ns := newNamespace()
		old := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), old)
		createCluster(holder)
		expectClaimedBy(old, holder)

		other := createBinding(ns, true)
		updateCluster(holder, func(c *v1.CamundaCluster) { c.Spec.StorageRef = other.Name })
		expectClaimedBy(other, holder)
		Eventually(func(g Gomega) {
			var lease coordinationv1.Lease
			name := client.ObjectKey{
				Namespace: testClaimNamespace,
				Name:      components.StorageClaimSchema().LeaseName(storageKeyOf(old)),
			}
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, name, &lease))).To(BeTrue())
		}, timeout, interval).Should(Succeed(), "the claim of the old backend is released")
	})

	// Two clusters that swap backends in one step meet each other's claim. A
	// parked cluster writes nothing, so it gives its previous backend back,
	// and the two never block each other for good.
	It("lets two clusters swap their backends in one step", func() {
		ns := newNamespace()
		first := createBinding(ns, true)
		second := createBinding(ns, true)
		a := newNamedCluster("cc-a-", ns, createPlatformConfig(), first)
		createCluster(a)
		b := newNamedCluster("cc-b-", ns, createPlatformConfig(), second)
		createCluster(b)
		expectClaimedBy(first, a)
		expectClaimedBy(second, b)

		updateCluster(a, func(c *v1.CamundaCluster) { c.Spec.StorageRef = second.Name })
		updateCluster(b, func(c *v1.CamundaCluster) { c.Spec.StorageRef = first.Name })

		expectClaimedBy(second, a)
		expectClaimedBy(first, b)
		expectHolds(a)
		expectHolds(b)
	})

	// An evicted pod of a previous holder keeps its object under a ReplicaSet
	// that nobody deleted. It writes nothing, so it must not hold the backend
	// of the next cluster for good.
	It("ignores a pod of a previous holder that ended", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		ghost := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"},
		}
		createStorageLeaseFor(storageKeyOf(binding), ghost)
		pod := createStoragePod(ghost, binding)

		By("failing the pod the way an eviction does")
		pod.Status.Phase = corev1.PodFailed
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(cluster)

		expectHolds(cluster)
		expectClaimedBy(binding, cluster)
	})

	// The render at zero deletes the real pods, so a hand-made pod stands in
	// for one that drains slowly.
	It("keeps the claim of the old backend while its own pods still write it", func() {
		ns := newNamespace()
		old := createBinding(ns, true)
		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), old)
		createCluster(cluster)
		expectClaimedBy(old, cluster)
		pod := createStoragePod(cluster, old)

		other := createBinding(ns, true)
		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.StorageRef = other.Name })
		expectClaimedBy(other, cluster)

		oldClaim := client.ObjectKey{
			Namespace: testClaimNamespace,
			Name:      components.StorageClaimSchema().LeaseName(storageKeyOf(old)),
		}
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, oldClaim, &coordinationv1.Lease{})).To(Succeed())
		}, "2s", interval).Should(Succeed(), "the pod of this cluster still writes the old backend")

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, oldClaim, &coordinationv1.Lease{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed(), "the old backend is given back once the pod is gone")
	})

	// A hand-deleted Lease must not depose the cluster that runs on the
	// backend. The pods of the holder order the claimants of the free key, so
	// the holder writes its claim again and the parked cluster keeps waiting.
	It("gives a hand-deleted claim back to the cluster whose pods write the backend", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		expectHolds(holder)
		expectClaimedBy(binding, holder)
		createStoragePod(holder, binding)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		By("deleting the claim of the backend by hand")
		claim := client.ObjectKey{
			Namespace: testClaimNamespace,
			Name:      components.StorageClaimSchema().LeaseName(storageKeyOf(binding)),
		}
		var lease coordinationv1.Lease
		Expect(k8sClient.Get(ctx, claim, &lease)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &lease)).To(Succeed())

		expectClaimedBy(binding, holder)
		expectHolds(holder)
		expectParked(parked, holder)
	})

	// A suspended cluster has no pod to hold its backend, so a release on any
	// failed check would hand it to the cluster that waits for it. The check
	// that failed says nothing about the backend, and the suspension is not
	// forever.
	It("keeps the backend of a suspended cluster whose platform config goes away", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		cfg := createPlatformConfig()
		holder := newNamedCluster("cc-a-", ns, cfg, binding)
		holder.Spec.Suspend = true
		createCluster(holder)
		expectClaimedBy(binding, holder)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		By("deleting the platform config that the suspended cluster names")
		Expect(k8sClient.Delete(ctx, cfg)).To(Succeed())
		expectReady(holder, metav1.ConditionFalse, Equal(v1.ReasonInvalidReference), ContainSubstring(cfg.Name))

		claim := client.ObjectKey{
			Namespace: testClaimNamespace,
			Name:      components.StorageClaimSchema().LeaseName(storageKeyOf(binding)),
		}
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, claim, &coordinationv1.Lease{})).To(Succeed())
		}, "3s", interval).Should(Succeed(), "the suspended cluster keeps the backend it resolved")
		expectParked(parked, holder)
	})

	// A claim whose holder never existed, or was deleted before the operator
	// ran, must not park every later cluster forever.
	It("takes over a claim whose holder does not exist", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		ghost := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"},
		}
		createStorageLeaseFor(storageKeyOf(binding), ghost)

		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(cluster)

		expectHolds(cluster)
		expectClaimedBy(binding, cluster)
	})

	// A holder deleted mid-restore gives the backend back, and the next cluster
	// must still wait for the restore to end. No restore controller runs in
	// this suite, so the spec registers the writer that a running restore
	// registers.
	for _, kind := range []string{"LogicalRestoreElasticsearch", "LogicalRestoreRDBMS"} {
		It("waits for a live "+kind+" writer for a deleted holder before it resumes", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
			createCluster(holder)
			expectClaimedBy(binding, holder)
			writer := registerWriter(kind, holder, storageKeyOf(binding), "")

			parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
			createCluster(parked)
			expectParked(parked, holder)

			deleteHolder(holder)

			expectReady(
				parked,
				metav1.ConditionFalse,
				Equal(v1.ReasonWaitingForHandover),
				And(ContainSubstring(storageKeyOf(binding)), ContainSubstring(writer.String())),
			)
			expectRendersNothing(parked)

			Expect(
				storagewriter.Release(ctx, k8sClient, testClaimNamespace, storageKeyOf(binding), writer),
			).To(Succeed())
			expectHolds(parked)
			expectClaimedBy(binding, parked)
		})
	}

	// A restore that rolls the database server back holds the database through
	// a writer that names the contract. When the contract moves to another
	// address, the writer and the next cluster react to one change, so the
	// next cluster must wait whether or not the writer followed the move yet.
	It("waits for a writer of the contract after the database of a deleted holder moves", func() {
		ns := newNamespace()
		binding, server := createDatabaseBinding(ns)
		old := databaseKeyOf(server)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		expectDatabaseClaimedBy(old, holder)
		writer := registerWriter("PointInTimeRestore", holder, old, databaseContractOf(server))

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		deleteHolder(holder)
		expectWaitingForWriter(parked, old, writer)

		By("moving the contract before the writer follows it")
		moved := moveDatabaseServer(server, "moved-postgres."+ns+".svc")
		expectWaitingForWriter(parked, moved, writer)

		By("moving the writer to the new address")
		Expect(storagewriter.Register(
			ctx,
			k8sClient,
			k8sClient,
			testClaimNamespace,
			moved,
			components.StorageClaimSchema().LeaseName(moved),
			writer,
		)).To(Succeed())
		Expect(storagewriter.Release(ctx, k8sClient, testClaimNamespace, old, writer)).To(Succeed())
		claim := client.ObjectKey{Namespace: testClaimNamespace, Name: components.StorageClaimSchema().LeaseName(moved)}
		Consistently(func(g Gomega) {
			err := k8sClient.Get(ctx, claim, &coordinationv1.Lease{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, "2s", interval).Should(Succeed(), "the release of the old key wakes the cluster, and it still waits")

		Expect(storagewriter.Release(ctx, k8sClient, testClaimNamespace, moved, writer)).To(Succeed())
		expectHolds(parked)
		expectDatabaseClaimedBy(moved, parked)
	})

	// The holder cannot take the new address while its platform config is
	// gone, so the other cluster reaches the free claim first. That is the
	// order in which a key match alone lets it start beside the writer.
	It("waits for a writer of the contract when the next cluster reaches the moved database first", func() {
		ns := newNamespace()
		binding, server := createDatabaseBinding(ns)
		old := databaseKeyOf(server)
		cfg := createPlatformConfig()
		holder := newNamedCluster("cc-a-", ns, cfg, binding)
		createCluster(holder)
		expectDatabaseClaimedBy(old, holder)
		writer := registerWriter("PointInTimeRestore", holder, old, databaseContractOf(server))

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		Expect(k8sClient.Delete(ctx, cfg)).To(Succeed())
		expectReady(holder, metav1.ConditionFalse, Equal(v1.ReasonInvalidReference), ContainSubstring(cfg.Name))

		moved := moveDatabaseServer(server, "moved-postgres."+ns+".svc")
		expectWaitingForWriter(parked, moved, writer)
		claim := client.ObjectKey{Namespace: testClaimNamespace, Name: components.StorageClaimSchema().LeaseName(moved)}
		Consistently(func(g Gomega) {
			err := k8sClient.Get(ctx, claim, &coordinationv1.Lease{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, "2s", interval).Should(Succeed(), "a cluster that waits for a writer takes no claim")

		Expect(storagewriter.Release(ctx, k8sClient, testClaimNamespace, old, writer)).To(Succeed())
		expectHolds(parked)
		expectDatabaseClaimedBy(moved, parked)
	})

	// A writer Lease that outlives its restore, because somebody removed the
	// finalizer of the restore by hand, holds the backend until somebody
	// deletes it. Its timestamps say it was renewed a year ago with a
	// one-second duration, which no reader measures.
	It("waits for a leftover writer Lease until it is deleted by hand", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		expectClaimedBy(binding, holder)
		writer := registerWriter("LogicalRestoreRDBMS", holder, storageKeyOf(binding), "")
		ageWriter(storageKeyOf(binding), writer)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		deleteHolder(holder)

		expectReady(
			parked,
			metav1.ConditionFalse,
			Equal(v1.ReasonWaitingForHandover),
			ContainSubstring(writer.String()),
		)
		Consistently(func(g Gomega) {
			var current v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(parked), &current)).To(Succeed())
			ready := meta.FindStatusCondition(current.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonWaitingForHandover))
		}, 3*time.Second, interval).Should(Succeed())

		By("deleting the leftover writer Lease by its labels, as the docs say")
		Expect(k8sClient.DeleteAllOf(
			ctx,
			&coordinationv1.Lease{},
			client.InNamespace(testClaimNamespace),
			client.MatchingLabels{
				labels.ManagedByKey: labels.ManagedBy,
				labels.ComponentKey: storagewriter.Component,
				labels.WriterUIDKey: string(writer.UID),
			},
		)).To(Succeed())
		expectHolds(parked)
		expectClaimedBy(binding, parked)
	})

})

// registerWriter registers a writer of kind for target on backend, as a
// running restore into target does. A non-empty contract is the contract that
// the writer names.
func registerWriter(kind string, target *v1.CamundaCluster, backend, contract string) storagewriter.Writer {
	GinkgoHelper()
	writer := storagewriter.Writer{
		Kind:       kind,
		Namespace:  target.Namespace,
		Name:       "restore-" + utilrand.String(6),
		UID:        types.UID(utilrand.String(12)),
		ClusterUID: target.UID,
		Contract:   contract,
	}
	claim := components.StorageClaimSchema().LeaseName(backend)
	Expect(storagewriter.Register(ctx, k8sClient, k8sClient, testClaimNamespace, backend, claim, writer)).
		To(Succeed())

	return writer
}

// ageWriter makes the registration of writer on backend read as renewed a
// year ago with a one-second duration.
func ageWriter(backend string, writer storagewriter.Writer) {
	GinkgoHelper()
	key := types.NamespacedName{Namespace: testClaimNamespace, Name: storagewriter.LeaseName(backend, writer.UID)}
	old := metav1.NewMicroTime(time.Now().AddDate(-1, 0, 0))
	Eventually(func(g Gomega) {
		var lease coordinationv1.Lease
		g.Expect(k8sClient.Get(ctx, key, &lease)).To(Succeed())
		lease.Spec.AcquireTime = &old
		lease.Spec.RenewTime = &old
		lease.Spec.LeaseDurationSeconds = new(int32(1))
		g.Expect(k8sClient.Update(ctx, &lease)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// deleteHolder deletes a cluster and waits until it is gone, so that it gives
// its backend back.
func deleteHolder(holder *v1.CamundaCluster) {
	GinkgoHelper()
	Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
	Eventually(func(g Gomega) {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(holder), &v1.CamundaCluster{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

// expectRendersNothing asserts that a waiting cluster keeps its brokers at zero.
func expectRendersNothing(cluster *v1.CamundaCluster) {
	GinkgoHelper()
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Consistently(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
	}, "2s", interval).Should(Succeed())
}

// createDatabaseBinding creates an rdbms contract in namespace: a
// DatabaseServerConfig, a DatabaseConfig for the database camunda with its
// credentials Secret, and the SecondaryStorageConfig that names them.
func createDatabaseBinding(namespace string) (*v1.SecondaryStorageConfig, *v1.DatabaseServerConfig) {
	GinkgoHelper()
	server := newDatabaseServerConfig(namespace)
	Expect(k8sClient.Create(ctx, server)).To(Succeed())
	dbConfig := newDatabaseConfig()
	dbConfig.Namespace = namespace
	dbConfig.Spec.ServerRef = server.Name
	Expect(k8sClient.Create(ctx, dbConfig)).To(Succeed())
	createSecret(namespace, dbConfig.Spec.CredentialsSecretRef.Name, map[string]string{
		"username": "camunda", "password": "db-password",
	})
	binding := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "rdbms-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: dbConfig.Name},
		},
	}
	Expect(k8sClient.Create(ctx, binding)).To(Succeed())

	return binding, server
}

// databaseKeyOf returns the claim key of the database camunda at the address
// that server names.
func databaseKeyOf(server *v1.DatabaseServerConfig) string {
	GinkgoHelper()
	key, err := components.StorageClaimKey(databaseOf(server))
	Expect(err).NotTo(HaveOccurred())

	return key
}

// databaseContractOf returns the contract of the database camunda behind
// server.
func databaseContractOf(server *v1.DatabaseServerConfig) string {
	return components.StorageContract(databaseOf(server))
}

func databaseOf(server *v1.DatabaseServerConfig) components.Storage {
	return components.Storage{
		Type:      v1.SecondaryStorageTypeRDBMS,
		Namespace: server.Namespace,
		RDBMS: &components.RDBMSStorage{
			Server:   client.ObjectKeyFromObject(server),
			Host:     server.Spec.Host,
			Port:     server.Spec.Port,
			Database: "camunda",
		},
	}
}

// moveDatabaseServer points server at host and returns the claim key of the
// database there.
func moveDatabaseServer(server *v1.DatabaseServerConfig, host string) string {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
		latest.Spec.Host = host
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	server.Spec.Host = host

	return databaseKeyOf(server)
}

// expectDatabaseClaimedBy polls until the storage claim of backend records
// cluster.
func expectDatabaseClaimedBy(backend string, cluster *v1.CamundaCluster) {
	GinkgoHelper()
	name := client.ObjectKey{Namespace: testClaimNamespace, Name: components.StorageClaimSchema().LeaseName(backend)}
	Eventually(func(g Gomega) {
		var lease coordinationv1.Lease
		g.Expect(k8sClient.Get(ctx, name, &lease)).To(Succeed())
		holder, ours := components.StorageClaimSchema().HolderOf(&lease)
		g.Expect(ours).To(BeTrue())
		g.Expect(holder.UID).To(Equal(cluster.UID))
	}, timeout, interval).Should(Succeed())
}

// expectWaitingForWriter polls until cluster reports WaitingForHandover for
// writer on backend, and checks that it renders nothing meanwhile.
func expectWaitingForWriter(cluster *v1.CamundaCluster, backend string, writer storagewriter.Writer) {
	GinkgoHelper()
	expectReady(
		cluster,
		metav1.ConditionFalse,
		Equal(v1.ReasonWaitingForHandover),
		And(ContainSubstring(backend), ContainSubstring(writer.String())),
	)
	expectRendersNothing(cluster)
}

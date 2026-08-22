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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// newNamedCluster is newCluster with a name prefix, so a pair created in one
// second still has a deterministic holder: the oldest cluster holds the
// backend, with the name breaking a tie, and "cc-a-" sorts before "cc-b-".
func newNamedCluster(
	prefix, namespace string,
	cfg *v1.CamundaPlatformConfig,
	binding *v1.SecondaryStorageConfig,
) *v1.CamundaCluster {
	cluster := newCluster(namespace, cfg, binding)
	cluster.Name = prefix + utilrand.String(8)
	return cluster
}

// createRDBMSBinding creates a DatabaseConfig on server in namespace, with its
// credentials Secret, and an rdbms binding that names it.
//
// Every SecondaryStorageConfig this returns resolves to the same backend
// identity, because fixtures.DatabaseServerConfig fixes the host and the
// port, and fixtures.DatabaseConfig fixes the database name. Two specs stay
// independent because createCluster deletes each cluster and waits for it to
// be gone before the next spec runs.
func createRDBMSBinding(namespace string, server *v1.DatabaseServerConfig) *v1.SecondaryStorageConfig {
	GinkgoHelper()
	dbConfig := fixtures.DatabaseConfig()
	dbConfig.Namespace = namespace
	dbConfig.Spec.ServerRef = server.Name
	dbConfig.Spec.CredentialsSecretRef.Namespace = namespace
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
	return binding
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

var _ = Describe("CamundaCluster secondary storage backend", func() {
	// One CamundaCluster uses one backend, so the oldest cluster holds it and
	// the other is suspended, with its volumes, until the holder releases it.
	It("suspends the newer of two clusters on one Elasticsearch contract and names the holder", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)

		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)

		expectHolds(holder)
		expectParked(parked, holder)
		expectReady(
			parked,
			metav1.ConditionFalse,
			Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring(binding.Spec.Elasticsearch.Endpoint),
		)
	})

	// The backend is the endpoint, not the contract: two contracts in two
	// namespaces that name one endpoint name one backend.
	It("suspends a cluster in another namespace whose contract names the same endpoint", func() {
		holderNS := newNamespace()
		holderBinding := createBinding(holderNS, true)
		holder := newNamedCluster("cc-a-", holderNS, createPlatformConfig(), holderBinding)
		createCluster(holder)

		parkedNS := newNamespace()
		parkedBinding := fixtures.SecondaryStorageConfigElasticsearch(parkedNS)
		parkedBinding.Spec.Elasticsearch.Endpoint = holderBinding.Spec.Elasticsearch.Endpoint
		Expect(k8sClient.Create(ctx, parkedBinding)).To(Succeed())
		createSecret(parkedNS, parkedBinding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "es-password",
		})
		parked := newNamedCluster("cc-b-", parkedNS, createPlatformConfig(), parkedBinding)
		createCluster(parked)

		expectHolds(holder)
		expectParked(parked, holder)
	})

	It("suspends the newer of two clusters on one database", func() {
		server := fixtures.DatabaseServerConfig()
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

		holderNS := newNamespace()
		holder := newNamedCluster("cc-a-", holderNS, createPlatformConfig(), createRDBMSBinding(holderNS, server))
		createCluster(holder)
		parkedNS := newNamespace()
		parked := newNamedCluster("cc-b-", parkedNS, createPlatformConfig(), createRDBMSBinding(parkedNS, server))
		createCluster(parked)

		expectHolds(holder)
		expectParked(parked, holder)
		expectReady(
			parked,
			metav1.ConditionFalse,
			Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring(server.Spec.Host+":5432/camunda"),
		)
	})

	It("lets two clusters on two endpoints both run", func() {
		ns := newNamespace()
		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(first)
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(second)

		expectHolds(first)
		expectHolds(second)
	})

	// Nothing but a watch on the clusters tells a parked cluster that its
	// holder went: its own watch reports events on itself only.
	It("resumes the parked cluster when the holder is deleted", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		Expect(k8sClient.Create(ctx, holder)).To(Succeed())
		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(holder), &v1.CamundaCluster{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		expectHolds(parked)
	})

	// A contract that names another endpoint releases the backend: the
	// parked cluster on it holds the backend on the next reconcile.
	It("resumes the parked cluster when the holder's contract names another endpoint", func() {
		ns := newNamespace()
		holderBinding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), holderBinding)
		createCluster(holder)
		parkedBinding := fixtures.SecondaryStorageConfigElasticsearch(ns)
		parkedBinding.Spec.Elasticsearch.Endpoint = holderBinding.Spec.Elasticsearch.Endpoint
		Expect(k8sClient.Create(ctx, parkedBinding)).To(Succeed())
		createSecret(ns, parkedBinding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "es-password",
		})
		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), parkedBinding)
		createCluster(parked)
		expectHolds(holder)
		expectParked(parked, holder)

		Eventually(func(g Gomega) {
			var latest v1.SecondaryStorageConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(holderBinding), &latest)).To(Succeed())
			latest.Spec.Elasticsearch.Endpoint = "https://" + holderBinding.Name + "-other." + ns + ".svc:9200"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectHolds(holder)
		expectHolds(parked)
	})

	// An older cluster whose chain does not resolve uses nothing yet. When
	// its chain resolves it holds the backend, and the newer cluster that held
	// it so far must yield. Only a watch on the chain tells the newer cluster.
	It("parks the holder when an older cluster's chain resolves onto its backend", func() {
		server := fixtures.DatabaseServerConfig()
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

		olderNS := newNamespace()
		missing := "dbc-" + utilrand.String(8)
		olderBinding := &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "rdbms-" + utilrand.String(8), Namespace: olderNS},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: missing},
			},
		}
		Expect(k8sClient.Create(ctx, olderBinding)).To(Succeed())
		older := newNamedCluster("cc-a-", olderNS, createPlatformConfig(), olderBinding)
		createCluster(older)
		expectReady(older, metav1.ConditionFalse, Equal(v1.ReasonInvalidReference), ContainSubstring(missing))

		newerNS := newNamespace()
		newer := newNamedCluster("cc-b-", newerNS, createPlatformConfig(), createRDBMSBinding(newerNS, server))
		createCluster(newer)
		expectHolds(newer)

		By("creating the DatabaseConfig of the older cluster on the same database")
		dbConfig := fixtures.DatabaseConfig()
		dbConfig.Name = missing
		dbConfig.Namespace = olderNS
		dbConfig.Spec.ServerRef = server.Name
		dbConfig.Spec.CredentialsSecretRef.Namespace = olderNS
		Expect(k8sClient.Create(ctx, dbConfig)).To(Succeed())
		createSecret(olderNS, dbConfig.Spec.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "db-password",
		})

		expectHolds(older)
		expectParked(newer, older)
	})

	// An older cluster that repoints its storageRef onto a backend a newer
	// cluster uses takes it; the newer cluster yields on the spec change.
	It("parks the holder when an older cluster repoints its storageRef onto its backend", func() {
		ns := newNamespace()
		older := newNamedCluster("cc-a-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(older)
		newerBinding := createBinding(ns, true)
		newer := newNamedCluster("cc-b-", ns, createPlatformConfig(), newerBinding)
		createCluster(newer)
		expectHolds(older)
		expectHolds(newer)

		updateCluster(older, func(c *v1.CamundaCluster) { c.Spec.StorageRef = newerBinding.Name })

		expectHolds(older)
		expectParked(newer, older)
	})
})

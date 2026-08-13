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

package controller

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// databaseConfigSecret builds a Secret at ref holding a placeholder value for
// each key.
func databaseConfigSecret(ref v1.CredentialsSecretRef, keys ...string) *corev1.Secret {
	data := make(map[string][]byte, len(keys))
	for _, key := range keys {
		data[key] = []byte("value")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace},
		Data:       data,
	}
}

// expectDatabaseConfigReady polls the DatabaseConfig at key until its Ready
// condition matches status, reason, and message.
func expectDatabaseConfigReady(key types.NamespacedName, status metav1.ConditionStatus, reason, message string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var dbConfig v1.DatabaseConfig
		g.Expect(k8sClient.Get(ctx, key, &dbConfig)).To(Succeed())
		cond := meta.FindStatusCondition(dbConfig.Status.Conditions, conditions.TypeReady)
		g.Expect(cond).NotTo(BeNil())
		g.Expect(cond.Status).To(Equal(status))
		g.Expect(cond.Reason).To(Equal(reason))
		g.Expect(cond.Message).To(Equal(message))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("DatabaseConfig controller", func() {
	var (
		ns       string
		server   *v1.DatabaseServerConfig
		dbConfig *v1.DatabaseConfig
	)

	create := func(obj client.Object) {
		GinkgoHelper()
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
	}

	BeforeEach(func() {
		ns = "dbc-" + utilrand.String(8)
		create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})

		server = validDatabaseServerConfig()
		dbConfig = validDatabaseConfig()
		dbConfig.Namespace = ns
		dbConfig.Spec.ServerRef = server.Name
		dbConfig.Spec.CredentialsSecretRef = v1.CredentialsSecretRef{
			Name: "app-creds", Namespace: ns,
			UsernameKey: "username", PasswordKey: "password",
		}
	})

	It("reports InvalidReference for a dangling serverRef", func() {
		create(dbConfig)

		expectDatabaseConfigReady(
			client.ObjectKeyFromObject(dbConfig), metav1.ConditionFalse, conditions.ReasonInvalidReference,
			fmt.Sprintf("DatabaseServerConfig %q not found", dbConfig.Spec.ServerRef),
		)
	})

	It("moves validation forward to the Secret checks when the server appears", func() {
		create(dbConfig)
		expectDatabaseConfigReady(
			client.ObjectKeyFromObject(dbConfig), metav1.ConditionFalse, conditions.ReasonInvalidReference,
			fmt.Sprintf("DatabaseServerConfig %q not found", dbConfig.Spec.ServerRef),
		)

		create(server)

		expectDatabaseConfigReady(
			client.ObjectKeyFromObject(dbConfig), metav1.ConditionFalse, conditions.ReasonMissingSecret,
			fmt.Sprintf("Secret %q not found", ns+"/app-creds"),
		)
	})

	It("flips to Healthy when the app Secret appears", func() {
		create(server)
		create(dbConfig)
		expectDatabaseConfigReady(
			client.ObjectKeyFromObject(dbConfig), metav1.ConditionFalse, conditions.ReasonMissingSecret,
			fmt.Sprintf("Secret %q not found", ns+"/app-creds"),
		)

		create(databaseConfigSecret(dbConfig.Spec.CredentialsSecretRef, "username", "password"))

		expectDatabaseConfigReady(client.ObjectKeyFromObject(dbConfig), metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed")
	})

	It("reports Healthy when the server and all Secrets exist", func() {
		dbConfig.Spec.BackupCredentialsSecretRef = &v1.CredentialsSecretRef{
			Name: "backup-creds", Namespace: ns,
			UsernameKey: "username", PasswordKey: "password",
		}
		create(server)
		create(databaseConfigSecret(dbConfig.Spec.CredentialsSecretRef, "username", "password"))
		create(databaseConfigSecret(*dbConfig.Spec.BackupCredentialsSecretRef, "username", "password"))
		create(dbConfig)

		expectDatabaseConfigReady(client.ObjectKeyFromObject(dbConfig), metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed")
	})

	It("flips a Healthy contract to InvalidReference when the server is deleted", func() {
		create(server)
		create(databaseConfigSecret(dbConfig.Spec.CredentialsSecretRef, "username", "password"))
		create(dbConfig)
		expectDatabaseConfigReady(client.ObjectKeyFromObject(dbConfig), metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed")

		Expect(k8sClient.Delete(ctx, server)).To(Succeed())

		expectDatabaseConfigReady(
			client.ObjectKeyFromObject(dbConfig), metav1.ConditionFalse, conditions.ReasonInvalidReference,
			fmt.Sprintf("DatabaseServerConfig %q not found", dbConfig.Spec.ServerRef),
		)
	})

	It("names the app Secret and key when a key is missing", func() {
		create(server)
		create(databaseConfigSecret(dbConfig.Spec.CredentialsSecretRef, "username"))
		create(dbConfig)

		expectDatabaseConfigReady(
			client.ObjectKeyFromObject(dbConfig), metav1.ConditionFalse, conditions.ReasonMissingSecret,
			fmt.Sprintf("Secret %q is missing key %q", ns+"/app-creds", "password"),
		)
	})

	It("names the backup Secret when it is missing", func() {
		dbConfig.Spec.BackupCredentialsSecretRef = &v1.CredentialsSecretRef{
			Name: "backup-creds", Namespace: ns,
			UsernameKey: "username", PasswordKey: "password",
		}
		create(server)
		create(databaseConfigSecret(dbConfig.Spec.CredentialsSecretRef, "username", "password"))
		create(dbConfig)

		expectDatabaseConfigReady(
			client.ObjectKeyFromObject(dbConfig), metav1.ConditionFalse, conditions.ReasonMissingSecret,
			fmt.Sprintf("Secret %q not found", ns+"/backup-creds"),
		)
	})

	It("stamps observedGeneration after a spec update", func() {
		create(server)
		create(databaseConfigSecret(dbConfig.Spec.CredentialsSecretRef, "username", "password"))
		create(dbConfig)
		expectDatabaseConfigReady(client.ObjectKeyFromObject(dbConfig), metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed")

		Eventually(func(g Gomega) {
			var latest v1.DatabaseConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dbConfig), &latest)).To(Succeed())
			latest.Spec.DatabaseName = "camunda-updated"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var latest v1.DatabaseConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dbConfig), &latest)).To(Succeed())
			g.Expect(latest.Generation).To(BeNumerically(">", int64(1)))
			g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
		}, timeout, interval).Should(Succeed())
	})
})

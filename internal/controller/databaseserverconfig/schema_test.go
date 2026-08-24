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

package databaseserverconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// requestUID is the identity of the request that the schema cases carry.
const requestUID = "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e"

// recoveryRequest returns a valid recovery request with mutate applied, so a
// case names the one field it is about.
func recoveryRequest(mutate func(*v1.RecoveryRequest)) *v1.RecoveryRequest {
	request := &v1.RecoveryRequest{
		RequestID:   requestUID,
		RequestedBy: "camunda/pitr-1",
		TargetTime:  "2026-08-20T14:30:00Z",
	}
	mutate(request)

	return request
}

// pitrWithOutcome returns a capability that carries a valid answer with mutate
// applied.
func pitrWithOutcome(mutate func(*v1.RecoveryOutcome)) *v1.PITRCapability {
	outcome := &v1.RecoveryOutcome{
		RequestID:   requestUID,
		RequestedBy: "camunda/pitr-1",
		TargetTime:  "2026-08-20T14:30:00Z",
		CompletedAt: metav1.Now(),
		Result:      v1.RecoveryResultCompleted,
	}
	mutate(outcome)

	return &v1.PITRCapability{
		Enabled:             true,
		RetentionPeriodDays: new(int32(7)),
		LastRecovery:        outcome,
	}
}

var _ = Describe("DatabaseServerConfig schema", func() {
	DescribeTable(
		"admission",
		func(mutate func(*v1.DatabaseServerConfig), wantErr string) {
			obj := fixtures.DatabaseServerConfig(fixtures.SchemaTestNamespace)
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example", func(*v1.DatabaseServerConfig) {}, ""),
		Entry(
			"accepts pitr with retention", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{Enabled: true, RetentionPeriodDays: new(int32(7))}
			}, "",
		),
		Entry("rejects unknown engine", func(o *v1.DatabaseServerConfig) { o.Spec.Engine = "mysql" }, "spec.engine"),
		Entry("rejects port 0", func(o *v1.DatabaseServerConfig) { o.Spec.Port = 0 }, "spec.port"),
		Entry("rejects port above 65535", func(o *v1.DatabaseServerConfig) { o.Spec.Port = 70000 }, "spec.port"),
		Entry("rejects empty host", func(o *v1.DatabaseServerConfig) { o.Spec.Host = "" }, "spec.host"),
		Entry(
			"rejects an empty admin secret name", func(o *v1.DatabaseServerConfig) {
				o.Spec.AdminCredentialsSecretRef.Name = ""
			}, "spec.adminCredentialsSecretRef.name",
		),
		Entry(
			"rejects pitr enabled without retention", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{Enabled: true}
			}, "retentionPeriodDays",
		),
		Entry(
			"rejects pitr retention 0", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{Enabled: true, RetentionPeriodDays: new(int32(0))}
			}, "retentionPeriodDays",
		),
		Entry(
			"accepts operator recovery on a server that archives", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{
					Enabled:             true,
					RetentionPeriodDays: new(int32(7)),
					Recovery:            v1.RecoveryModeOperator,
				}
			}, "",
		),
		Entry(
			"rejects operator recovery on a server that archives nothing",
			func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{Recovery: v1.RecoveryModeOperator}
			}, "recovery: operator requires enabled: true",
		),
		Entry(
			"rejects an unknown recovery mode", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{
					Enabled: true, RetentionPeriodDays: new(int32(7)), Recovery: "provider",
				}
			}, "spec.pitr.recovery",
		),
		Entry(
			"accepts a recovery request with a zone", func(o *v1.DatabaseServerConfig) {
				o.Spec.Recovery = recoveryRequest(func(*v1.RecoveryRequest) {})
			}, "",
		),
		Entry(
			"rejects a recovery target without a zone", func(o *v1.DatabaseServerConfig) {
				o.Spec.Recovery = recoveryRequest(func(r *v1.RecoveryRequest) {
					r.TargetTime = "2026-08-20T14:30:00"
				})
			}, "spec.recovery.targetTime",
		),
		Entry(
			"rejects a requester that names no namespace", func(o *v1.DatabaseServerConfig) {
				o.Spec.Recovery = recoveryRequest(func(r *v1.RecoveryRequest) {
					r.RequestedBy = "pitr-1"
				})
			}, "spec.recovery.requestedBy",
		),
		Entry(
			"rejects a request that carries no identity", func(o *v1.DatabaseServerConfig) {
				o.Spec.Recovery = recoveryRequest(func(r *v1.RecoveryRequest) { r.RequestID = "" })
			}, "spec.recovery.requestID",
		),
		Entry(
			"rejects an identity that is not a uid", func(o *v1.DatabaseServerConfig) {
				o.Spec.Recovery = recoveryRequest(func(r *v1.RecoveryRequest) { r.RequestID = "pitr-1" })
			}, "spec.recovery.requestID",
		),
		Entry(
			"accepts an outcome that answers a request", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = pitrWithOutcome(func(*v1.RecoveryOutcome) {})
			}, "",
		),
		Entry(
			"rejects an outcome target without a zone", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = pitrWithOutcome(func(out *v1.RecoveryOutcome) {
					out.TargetTime = "2026-08-20T14:30:00"
				})
			}, "spec.pitr.lastRecovery.targetTime",
		),
		Entry(
			"rejects an outcome that answers no identity", func(o *v1.DatabaseServerConfig) {
				out := pitrWithOutcome(func(out *v1.RecoveryOutcome) { out.RequestID = "" })
				o.Spec.PITR = out
			}, "spec.pitr.lastRecovery.requestID",
		),
	)

	It("defaults the recovery mode to external", func() {
		obj := fixtures.DatabaseServerConfig(fixtures.SchemaTestNamespace)
		obj.Spec.PITR = &v1.PITRCapability{Enabled: true, RetentionPeriodDays: new(int32(7))}
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		var got v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &got)).To(Succeed())
		Expect(got.Spec.PITR.Recovery).To(Equal(v1.RecoveryModeExternal))
	})

	It("defaults the admin credential keys to username and password", func() {
		obj := fixtures.DatabaseServerConfig(fixtures.SchemaTestNamespace)
		obj.Spec.AdminCredentialsSecretRef.UsernameKey = ""
		obj.Spec.AdminCredentialsSecretRef.PasswordKey = ""
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		var got v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &got)).To(Succeed())
		Expect(got.Spec.AdminCredentialsSecretRef.UsernameKey).To(Equal("username"))
		Expect(got.Spec.AdminCredentialsSecretRef.PasswordKey).To(Equal("password"))
	})
})

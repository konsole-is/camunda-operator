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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// newRequest creates a request in default for registration, with the label
// and spec.registration both set to it, and registers its deletion.
func newRequest(registration, parameters string, policy pluginsv1alpha1.DeletionPolicy) *pluginsv1alpha1.PluginRequest {
	GinkgoHelper()
	return createRequest(registration, registration, parameters, policy)
}

// createRequest creates a request in default whose registration label is
// label and whose spec.registration is registration, and registers its
// deletion. The cleanup strips the finalizer, so no spec waits on it.
func createRequest(
	label, registration, parameters string,
	policy pluginsv1alpha1.DeletionPolicy,
) *pluginsv1alpha1.PluginRequest {
	GinkgoHelper()
	section := registrationSection(registration)
	if section == "" {
		section = "backup-storage"
	}
	request := &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "req-" + utilrand.String(8),
			Namespace: "default",
			Labels: map[string]string{
				pluginsv1alpha1.LabelSection:      section,
				pluginsv1alpha1.LabelRegistration: label,
				fleetv1alpha1.LabelClaim:          "orders",
			},
		},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Section:        section,
			Registration:   registration,
			ClaimRef:       pluginsv1alpha1.ClaimRef{Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: "orders"},
			Target:         pluginsv1alpha1.TargetRef{Kind: "CamundaCluster", Name: "orders"},
			DeletionPolicy: policy,
		},
	}
	if parameters != "" {
		request.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(parameters)}
	}

	Expect(k8sClient.Create(ctx, request)).To(Succeed())
	DeferCleanup(func() {
		var live pluginsv1alpha1.PluginRequest
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(request), &live); err != nil {
			return
		}
		if controllerutil.RemoveFinalizer(&live, pluginsv1alpha1.FinalizerCleanup) {
			_ = k8sClient.Update(ctx, &live)
		}
		_ = k8sClient.Delete(ctx, &live)
	})

	return request
}

// newObjectStorageConfig returns an S3 ObjectStorageConfig with workload
// identity and a unique name in default.
func newObjectStorageConfig() *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8), Namespace: "default"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				Region:     "eu-west-1",
				Auth: v1.S3StorageAuth{
					Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
					WorkloadIdentity: &v1.S3WorkloadIdentity{},
				},
			},
		},
	}
}

// newSecondaryStorageConfig returns an RDBMS SecondaryStorageConfig with a
// unique name in default.
func newSecondaryStorageConfig() *v1.SecondaryStorageConfig {
	return &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + utilrand.String(8), Namespace: "default"},
		Spec: v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: "my-camunda-db"},
		},
	}
}

// createContract creates obj and registers its deletion.
func createContract(obj client.Object) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
}

// liveRequest reads request from the API server.
func liveRequest(g Gomega, request *pluginsv1alpha1.PluginRequest) *pluginsv1alpha1.PluginRequest {
	var live pluginsv1alpha1.PluginRequest
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(request), &live)).To(Succeed())
	return &live
}

// expectCondition waits until the request reports conditionType with status
// and reason at its current generation, and returns the live request.
func expectCondition(
	request *pluginsv1alpha1.PluginRequest,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
) *pluginsv1alpha1.PluginRequest {
	GinkgoHelper()
	var live *pluginsv1alpha1.PluginRequest
	Eventually(func(g Gomega) {
		live = liveRequest(g, request)
		cond := meta.FindStatusCondition(live.Status.Conditions, conditionType)
		g.Expect(cond).NotTo(BeNil())
		g.Expect(cond.Status).To(Equal(status))
		g.Expect(cond.Reason).To(Equal(reason))
		g.Expect(cond.ObservedGeneration).To(Equal(live.Generation))
	}, timeout, interval).Should(Succeed())
	return live
}

// expectGone waits until obj is not found.
func expectGone(obj client.Object) {
	GinkgoHelper()
	Eventually(func() bool {
		return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj))
	}, timeout, interval).Should(BeTrue())
}

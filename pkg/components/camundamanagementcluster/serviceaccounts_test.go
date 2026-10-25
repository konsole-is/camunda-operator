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

package camundamanagementcluster

import (
	"context"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/accounthash"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// fixtureWorkloads are the components that run pods, by the block whose
// serviceAccount they read.
var fixtureWorkloads = []string{
	ComponentKeycloak,
	ComponentIdentity,
	ComponentConsole,
	ComponentWebModelerRestapi,
	ComponentWebModelerWebsockets,
}

func TestServiceAccountName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, fixtureName+"-management", ServiceAccountName(newCluster(nil)))
	assert.Equal(t, "plane", ServiceAccountName(newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "plane"}
	})))
	assert.Equal(t, fixtureName+"-management", ServiceAccountName(newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Spec.ServiceAccount = &v1.ServiceAccountSpec{Annotations: map[string]string{"a": "b"}}
	})))
}

// Without a block of its own, every component runs under the account of the
// plane.
func TestPodServiceAccountNameIsThePlaneAccountByDefault(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, withKeycloakApps)
	for _, comp := range fixtureWorkloads {
		assert.Equal(t, fixtureName+"-management", PodServiceAccountName(in.Cluster, comp), comp)
	}
}

// A component block with serviceAccount gives the pods of that component an
// account of their own. Both Web Modeler processes read the Web Modeler
// block.
func TestPodServiceAccountNameOfAComponentWithItsOwnAccount(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, func(in *Input) {
		withKeycloakApps(in)
		spec := &in.Cluster.Spec
		spec.IdentityProvider.Keycloak.ServiceAccount = &v1.ServiceAccountSpec{}
		spec.Identity.ServiceAccount = &v1.ServiceAccountSpec{}
		spec.Console.ServiceAccount = &v1.ServiceAccountSpec{Name: "console-sa"}
		spec.WebModeler.ServiceAccount = &v1.ServiceAccountSpec{}
	})

	want := map[string]string{
		ComponentKeycloak:             fixtureName + "-keycloak",
		ComponentIdentity:             fixtureName + "-identity",
		ComponentConsole:              "console-sa",
		ComponentWebModelerRestapi:    fixtureName + "-web-modeler",
		ComponentWebModelerWebsockets: fixtureName + "-web-modeler",
	}
	for comp, name := range want {
		assert.Equal(t, name, PodServiceAccountName(in.Cluster, comp), comp)
	}
}

// The operator creates the account of the plane with the annotations of
// spec.serviceAccount, and the account of each component block that sets
// one with the annotations of that block.
func TestServiceAccountsRenderThePlaneAccountAndEachComponentAccount(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, func(in *Input) {
		withKeycloakApps(in)
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Annotations: map[string]string{"plane": "yes"}}
		in.Cluster.Spec.Console.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{"console": "yes"},
		}
	})

	accounts := previewedAccounts(t, in)

	assert.Equal(
		t, map[string]map[string]string{
			fixtureName + "-management": {"plane": "yes"},
			fixtureName + "-console":    {"console": "yes"},
		}, accounts,
	)
}

// An account with create false is the user's: the operator applies nothing to
// it.
func TestServiceAccountsRenderNoAccountWithCreateFalse(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "existing", Create: new(false)}
	})

	assert.Empty(t, previewedAccounts(t, in))
}

// A component block without a running component renders no account: the
// Keycloak block of another mode does not exist, and a Console that the spec
// dropped takes its block with it.
func TestServiceAccountsRenderNoAccountForAComponentThatDoesNotRun(t *testing.T) {
	t.Parallel()

	accounts := previewedAccounts(t, fixtureMinimal(t))

	assert.Equal(t, map[string]map[string]string{fixtureName + "-management": nil}, accounts)
}

// The account of a component that does not run is deleted under its derived
// name, but never when that name is the account the plane runs under.
func TestServiceAccountsKeepThePlaneAccountUnderTheNameOfAnUnusedComponentAccount(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: fixtureName + "-console"}
	})

	c := reconcileServiceAccounts(t, in)

	key := client.ObjectKey{Namespace: fixtureNamespace, Name: fixtureName + "-console"}
	require.NoError(t, c.Get(context.Background(), key, &corev1.ServiceAccount{}))
}

// Every workload names the account it runs under, and Keycloak names it
// through the pod template of the Keycloak Operator.
func TestWorkloadsRunUnderTheirAccount(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, func(in *Input) {
		withKeycloakApps(in)
		in.Cluster.Spec.Identity.ServiceAccount = &v1.ServiceAccountSpec{Name: "identity-sa"}
	})

	got := previewedPodTemplates(t, in)

	assert.Equal(
		t, map[string]string{
			ComponentKeycloak:             fixtureName + "-management",
			ComponentIdentity:             "identity-sa",
			ComponentConsole:              fixtureName + "-management",
			ComponentWebModelerRestapi:    fixtureName + "-management",
			ComponentWebModelerWebsockets: fixtureName + "-management",
		}, serviceAccountNames(got),
	)
}

// A change of the annotations that the operator renders on an account rolls
// the pods that run under it, because workload identity reads them when a pod
// starts.
func TestPodsCarryTheHashOfTheAnnotationsOfTheirAccount(t *testing.T) {
	t.Parallel()

	plane := map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/plane"}
	console := map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/console"}
	in := newKeycloakInput(t, true, func(in *Input) {
		withKeycloakApps(in)
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Annotations: plane}
		in.Cluster.Spec.Console.ServiceAccount = &v1.ServiceAccountSpec{Annotations: console}
	})

	got := previewedPodTemplates(t, in)

	for comp, template := range got {
		want := accounthash.Of(plane)
		if comp == ComponentConsole {
			want = accounthash.Of(console)
		}
		assert.Equal(t, want, template.Annotations[accounthash.Annotation], comp)
	}
}

// An account with create false, or without annotations, puts no hash on the
// pods, and a user pod annotation cannot put an old one there.
func TestPodsCarryNoAccountHashWithoutRenderedAnnotations(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, func(in *Input) {
		withKeycloakApps(in)
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Name: "existing", Create: new(false), Annotations: map[string]string{"a": "b"},
		}
		in.Cluster.Spec.Identity.PodAnnotations = map[string]string{accounthash.Annotation: "stale"}
	})

	for comp, template := range previewedPodTemplates(t, in) {
		assert.NotContains(t, template.Annotations, accounthash.Annotation, comp)
	}
}

// Azure binds a pod to the client ID of its account only when the pod carries
// the azure.workload.identity/use label.
func TestPodsOfAnAzureAccountCarryTheUseLabel(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, func(in *Input) {
		withKeycloakApps(in)
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.AzureClientIDAnnotation: "00000000-0000-0000-0000-000000000000"},
		}
		in.Cluster.Spec.Console.ServiceAccount = &v1.ServiceAccountSpec{}
	})

	for comp, template := range previewedPodTemplates(t, in) {
		if comp == ComponentConsole {
			assert.NotContains(t, template.Labels, v1.AzureWorkloadIdentityUseLabel, comp)
			continue
		}
		assert.Equal(t, "true", template.Labels[v1.AzureWorkloadIdentityUseLabel], comp)
	}
}

// ReferencedServiceAccounts lists each account that a running part of the
// plane names with create false, with the field that names it.
func TestReferencedServiceAccounts(t *testing.T) {
	t.Parallel()

	mc := newKeycloakCluster(true, func(mc *v1.CamundaManagementCluster) {
		mc.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "plane", Create: new(false)}
		mc.Spec.Identity.ServiceAccount = &v1.ServiceAccountSpec{Create: new(true)}
		mc.Spec.IdentityProvider.Keycloak.ServiceAccount = &v1.ServiceAccountSpec{Create: new(false)}
	})

	assert.Equal(
		t, []ServiceAccountReference{
			{Field: "spec.serviceAccount", Name: "plane"},
			{Field: "spec.identityProvider.keycloak.serviceAccount", Name: fixtureName + "-keycloak"},
		}, ReferencedServiceAccounts(mc),
	)
	assert.Empty(t, ReferencedServiceAccounts(newCluster(nil)))
}

// previewedAccounts returns the annotations of each ServiceAccount that the
// accounts component applies, by name.
func previewedAccounts(t *testing.T, in Input) map[string]map[string]string {
	t.Helper()

	built, err := Build(in)
	require.NoError(t, err)
	objects, err := builtComponent(t, built, ComponentServiceAccounts).Preview()
	require.NoError(t, err)

	accounts := map[string]map[string]string{}
	for _, obj := range objects {
		if account, ok := obj.(*corev1.ServiceAccount); ok {
			accounts[account.Name] = account.Annotations
		}
	}

	return accounts
}

// previewedPodTemplates returns the pod template of each workload that Build
// renders, by component. The Keycloak pod template is the one that the
// Keycloak resource gives the Keycloak Operator.
func previewedPodTemplates(t *testing.T, in Input) map[string]corev1.PodTemplateSpec {
	t.Helper()

	built, err := Build(in)
	require.NoError(t, err)

	templates := map[string]corev1.PodTemplateSpec{}
	for _, comp := range built.Components {
		objects, err := comp.Preview()
		require.NoError(t, err)
		for _, obj := range objects {
			switch workload := obj.(type) {
			case *keycloak.Keycloak:
				templates[ComponentKeycloak] = *workload.Spec.Unsupported.PodTemplate
			case *appsv1.Deployment:
				templates[workload.Labels[labels.ComponentKey]] = workload.Spec.Template
			}
		}
	}

	return templates
}

// serviceAccountNames returns the account of each pod template.
func serviceAccountNames(templates map[string]corev1.PodTemplateSpec) map[string]string {
	names := map[string]string{}
	for comp, template := range templates {
		names[comp] = template.Spec.ServiceAccountName
	}

	return names
}

// reconcileServiceAccounts reconciles the accounts component once against a
// fake API server and returns the client.
func reconcileServiceAccounts(t *testing.T, in Input) client.Client {
	t.Helper()

	built, err := Build(in)
	require.NoError(t, err)
	comp := builtComponent(t, built, ComponentServiceAccounts)

	owner := in.Cluster.DeepCopy()
	owner.UID = "plane-uid"
	owner.Status.Conditions = []metav1.Condition{}

	scheme := goldenScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme)).
		Build()
	require.NoError(t, comp.Reconcile(context.Background(), component.ReconcileContext{
		Client:        c,
		APIReader:     c,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(100),
		Owner:         owner,
	}))

	return c
}

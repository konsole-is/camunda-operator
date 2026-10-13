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

package main

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

// object returns an unstructured object for the tests of normalize. A uid
// that is not a UUID becomes one made from it, because normalize replaces
// each occurrence of a UID in the whole object, and a short uid such as "u"
// would match inside other strings.
func object(apiVersion, kind, namespace, name, uid string) unstructured.Unstructured {
	if _, err := uuid.Parse(uid); err != nil {
		uid = uuid.NewSHA1(uuid.NameSpaceURL, []byte(uid)).String()
	}

	var u unstructured.Unstructured
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	u.SetUID(types.UID(uid))
	u.SetResourceVersion("42")
	u.SetCreationTimestamp(metav1.Now())
	return u
}

// event returns an events.k8s.io Event about regarding, as the event
// recorder of a controller writes it.
func event(name string, regarding unstructured.Unstructured, reason, message string) unstructured.Unstructured {
	e := object("events.k8s.io/v1", "Event", regarding.GetNamespace(), name, "event-"+name)
	e.Object["reason"] = reason
	e.Object["note"] = message
	e.Object["eventTime"] = "2026-10-13T08:15:42.123456Z"
	e.Object["reportingController"] = "camundacluster"
	e.Object["reportingInstance"] = "camundacluster-some-laptop"
	e.Object["series"] = map[string]any{"count": int64(4), "lastObservedTime": "2026-10-13T08:16:42.123456Z"}
	e.Object["regarding"] = map[string]any{
		"apiVersion":      regarding.GetAPIVersion(),
		"kind":            regarding.GetKind(),
		"namespace":       regarding.GetNamespace(),
		"name":            regarding.GetName(),
		"uid":             string(regarding.GetUID()),
		"resourceVersion": "17",
	}
	return e
}

// find returns the object of objs with kind, namespace, and name, and fails
// the test with the objects it holds when there is none.
func find(t *testing.T, objs []unstructured.Unstructured, kind, namespace, name string) unstructured.Unstructured {
	t.Helper()

	var held []string
	for _, obj := range objs {
		if obj.GetKind() == kind && obj.GetNamespace() == namespace && obj.GetName() == name {
			return obj
		}
		held = append(held, describe(&obj))
	}
	require.Failf(t, "object not found", "%s %s/%s is not among %v", kind, namespace, name, held)
	return unstructured.Unstructured{}
}

func TestNormalizeDerivesEveryUIDFromKindNamespaceAndName(t *testing.T) {
	run := func(ownerUID, childUID string) []unstructured.Unstructured {
		owner := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", ownerUID)
		child := object("apps/v1", "StatefulSet", "team-a", "orders-zeebe", childUID)
		child.SetOwnerReferences([]metav1.OwnerReference{{
			APIVersion: "core.camunda.io/v1", Kind: "CamundaCluster", Name: "orders", UID: types.UID(ownerUID),
		}})
		child.SetManagedFields([]metav1.ManagedFieldsEntry{{
			Manager: "CamundaCluster/zeebe/" + ownerUID, Operation: metav1.ManagedFieldsOperationApply,
		}})
		out, err := normalize(newServiceIPs(), []unstructured.Unstructured{child, owner})
		require.NoError(t, err)
		return out
	}

	first := run("0b5f8c9e-1111-4c1a-9a51-000000000001", "0b5f8c9e-2222-4c1a-9a51-000000000002")
	second := run("7d0e1f2a-3333-4b2c-8d3e-000000000003", "7d0e1f2a-4444-4b2c-8d3e-000000000004")
	assert.Equal(t, first, second, "two runs with different real UIDs must write the same objects")

	ownerUID := scenarioUID("CamundaCluster", "team-a", "orders")
	owner := find(t, first, "CamundaCluster", "team-a", "orders")
	assert.Equal(t, ownerUID, string(owner.GetUID()))
	child := find(t, first, "StatefulSet", "team-a", "orders-zeebe")
	assert.Equal(t, ownerUID, string(child.GetOwnerReferences()[0].UID))
	assert.Equal(t, "CamundaCluster/zeebe/"+ownerUID, child.GetManagedFields()[0].Manager)
}

func TestNormalizeRemovesTheResourceVersion(t *testing.T) {
	out, err := normalize(newServiceIPs(), []unstructured.Unstructured{object("v1", "Namespace", "", "team-a", "u")})
	require.NoError(t, err)
	assert.Empty(t, out[0].GetResourceVersion())
	_, found, _ := unstructured.NestedFieldNoCopy(out[0].Object, "metadata", "resourceVersion")
	assert.False(t, found)
}

func TestNormalizeSetsEveryTimestampToTheScenarioTime(t *testing.T) {
	cluster := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "u1")
	cluster.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply, Time: &metav1.Time{Time: time.Now()},
	}})
	require.NoError(t, unstructured.SetNestedSlice(cluster.Object, []any{map[string]any{
		"type": "Ready", "status": "True", "lastTransitionTime": "2026-10-13T08:15:42Z",
	}}, "status", "conditions"))
	lease := object("coordination.k8s.io/v1", "Lease", "camunda-system", "claim", "u2")
	require.NoError(t, unstructured.SetNestedField(lease.Object, "2026-10-13T08:15:42.123456Z", "spec", "renewTime"))

	out, err := normalize(newServiceIPs(), []unstructured.Unstructured{cluster, lease})
	require.NoError(t, err)

	got := find(t, out, "CamundaCluster", "team-a", "orders")
	assert.Equal(t, scenarioTime.UTC(), got.GetCreationTimestamp().UTC())
	assert.Equal(t, scenarioTime.UTC(), got.GetManagedFields()[0].Time.UTC())
	conditions, _, _ := unstructured.NestedSlice(got.Object, "status", "conditions")
	assert.Equal(t, "2026-01-01T00:00:00Z", conditions[0].(map[string]any)["lastTransitionTime"])

	got = find(t, out, "Lease", "camunda-system", "claim")
	renew, _, _ := unstructured.NestedString(got.Object, "spec", "renewTime")
	assert.Equal(t, "2026-01-01T00:00:00.000000Z", renew, "a MicroTime stays in the micro format")
}

func TestNormalizeSetsEverySecretValue(t *testing.T) {
	secret := object("v1", "Secret", "team-a", "credentials", "u")
	secret.Object["data"] = map[string]any{"username": "Y2FtdW5kYQ==", "password": "c2VjcmV0"}

	out, err := normalize(newServiceIPs(), []unstructured.Unstructured{secret})
	require.NoError(t, err)

	data, _, _ := unstructured.NestedStringMap(out[0].Object, "data")
	assert.Equal(t, map[string]string{"username": "c2NlbmFyaW8=", "password": "c2NlbmFyaW8="}, data)
}

func TestNormalizeSortsManagedFields(t *testing.T) {
	obj := object("v1", "ConfigMap", "team-a", "settings", "u")
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationUpdate},
		{Manager: "CamundaCluster/zeebe", Operation: metav1.ManagedFieldsOperationApply},
		{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply, Subresource: "status"},
		{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
	})

	out, err := normalize(newServiceIPs(), []unstructured.Unstructured{obj})
	require.NoError(t, err)

	got := make([]string, 0, len(out[0].GetManagedFields()))
	for _, f := range out[0].GetManagedFields() {
		got = append(got, f.Manager+" "+string(f.Operation)+" "+f.Subresource)
	}
	assert.Equal(t, []string{
		"CamundaCluster/zeebe Apply ",
		"kubectl Apply ",
		"kubectl Apply status",
		"kubectl Update ",
	}, got)
}

func TestNormalizeSetsTheGenerationToOne(t *testing.T) {
	obj := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "u")
	obj.SetGeneration(4)
	require.NoError(t, unstructured.SetNestedField(obj.Object, int64(4), "status", "observedGeneration"))
	require.NoError(t, unstructured.SetNestedSlice(obj.Object, []any{map[string]any{
		"type": "Ready", "status": "True", "observedGeneration": int64(4),
	}}, "status", "conditions"))

	out, err := normalize(newServiceIPs(), []unstructured.Unstructured{obj})
	require.NoError(t, err)

	assert.Equal(t, int64(1), out[0].GetGeneration())
	observed, _, _ := unstructured.NestedInt64(out[0].Object, "status", "observedGeneration")
	assert.Equal(t, int64(1), observed)
	conditions, _, _ := unstructured.NestedSlice(out[0].Object, "status", "conditions")
	assert.Equal(t, int64(1), conditions[0].(map[string]any)["observedGeneration"])
}

func TestNormalizeKeepsAnObservedGenerationThatIsBehindBehind(t *testing.T) {
	for _, observedBefore := range []int64{1, 3} {
		obj := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "u")
		obj.SetGeneration(4)
		require.NoError(t, unstructured.SetNestedField(obj.Object, observedBefore, "status", "observedGeneration"))

		out, err := normalize(newServiceIPs(), []unstructured.Unstructured{obj})
		require.NoError(t, err)

		assert.Equal(t, int64(1), out[0].GetGeneration())
		observed, _, _ := unstructured.NestedInt64(out[0].Object, "status", "observedGeneration")
		assert.Equal(t, int64(0), observed, "an observed generation of %d is behind 4", observedBefore)
	}
}

func TestNormalizeGivesEachServiceAClusterIPOfTheEnvtestRange(t *testing.T) {
	service := func(name, ip string) unstructured.Unstructured {
		svc := object("v1", "Service", "team-a", name, name)
		svc.Object["spec"] = map[string]any{"clusterIP": ip, "clusterIPs": []any{ip}}
		return svc
	}
	run := func(gatewayIP, connectorsIP string) []unstructured.Unstructured {
		out, err := normalize(newServiceIPs(), []unstructured.Unstructured{
			service("orders-gateway", gatewayIP),
			service("orders-connectors", connectorsIP),
			service("orders-zeebe", "None"),
		})
		require.NoError(t, err)
		return out
	}

	first := run("10.0.0.42", "10.0.0.7")
	assert.Equal(t, first, run("10.0.0.250", "10.0.0.99"), "two runs with other IPs must write the same objects")

	ip := func(name string) string {
		svc := find(t, first, "Service", "team-a", name)
		value, _, _ := unstructured.NestedString(svc.Object, "spec", "clusterIP")
		return value
	}
	_, envtestRange, err := net.ParseCIDR("10.0.0.0/24")
	require.NoError(t, err)
	for _, name := range []string{"orders-gateway", "orders-connectors"} {
		assert.True(t, envtestRange.Contains(net.ParseIP(ip(name))), name)
	}
	assert.NotEqual(t, ip("orders-gateway"), ip("orders-connectors"))
	assert.Equal(t, "None", ip("orders-zeebe"), "a headless Service keeps None")
}

func TestNormalizeKeepsTheIPOfAServiceInALaterOutput(t *testing.T) {
	service := func(name string) unstructured.Unstructured {
		svc := object("v1", "Service", "team-a", name, name)
		svc.Object["spec"] = map[string]any{"clusterIP": "10.0.0.42"}
		return svc
	}
	ip := func(objs []unstructured.Unstructured, name string) string {
		value, _, _ := unstructured.NestedString(find(t, objs, "Service", "team-a", name).Object, "spec", "clusterIP")
		return value
	}
	ips := newServiceIPs()

	first, err := normalize(ips, []unstructured.Unstructured{service("orders-gateway")})
	require.NoError(t, err)
	later, err := normalize(ips, []unstructured.Unstructured{service("orders-gateway"), service("orders-api")})
	require.NoError(t, err)

	assert.Equal(t, ip(first, "orders-gateway"), ip(later, "orders-gateway"), "spec.clusterIP is immutable")
	assert.NotEqual(t, ip(later, "orders-gateway"), ip(later, "orders-api"))
}

func TestNormalizeSortsTheObjects(t *testing.T) {
	out, err := normalize(newServiceIPs(), []unstructured.Unstructured{
		object("v1", "Secret", "team-b", "a", "u1"),
		object("apps/v1", "StatefulSet", "team-a", "b", "u2"),
		object("v1", "Secret", "team-a", "b", "u3"),
		object("v1", "Secret", "team-a", "a", "u4"),
		object("v1", "ConfigMap", "team-a", "a", "u5"),
	})
	require.NoError(t, err)

	got := make([]string, 0, len(out))
	for i := range out {
		got = append(got, out[i].GetAPIVersion()+" "+describe(&out[i]))
	}
	assert.Equal(t, []string{
		"apps/v1 StatefulSet team-a/b",
		"v1 ConfigMap team-a/a",
		"v1 Secret team-a/a",
		"v1 Secret team-a/b",
		"v1 Secret team-b/a",
	}, got)
}

func TestNormalizeGivesAnEventANameFromWhatItIsAbout(t *testing.T) {
	run := func(eventName, clusterUID string) unstructured.Unstructured {
		cluster := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", clusterUID)
		out, err := normalize(newServiceIPs(), []unstructured.Unstructured{
			cluster, event(eventName, cluster, "Created", "Created StatefulSet orders-zeebe"),
		})
		require.NoError(t, err)
		require.Len(t, out, 2)
		return out[1]
	}

	first := run("orders.18f2a3b4c5d6e7f8", "uid-one")
	second := run("orders.28f2a3b4c5d6e7f9", "uid-two")
	assert.Equal(t, first, second, "two runs of one event must write the same Event")
	assert.Regexp(t, `^orders\.[0-9a-f]{16}$`, first.GetName())
}

func TestNormalizeGivesAnEventAboutALongNameAValidName(t *testing.T) {
	long := strings.Repeat("a", 235) + "-" + strings.Repeat("b", 17)
	cluster := object("core.camunda.io/v1", "CamundaCluster", "team-a", long, "u")
	out, err := normalize(
		newServiceIPs(),
		[]unstructured.Unstructured{cluster, event("long.a", cluster, "Created", "Created")},
	)
	require.NoError(t, err)

	for _, obj := range out {
		if isEvent(&obj) {
			assert.Empty(t, validation.IsDNS1123Subdomain(obj.GetName()), obj.GetName())
		}
	}
}

func TestNormalizeGivesTheRelatedObjectOfAnEventItsScenarioUID(t *testing.T) {
	cluster := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "u")
	e := event("orders.a", cluster, "Deleted", "Deleted StatefulSet orders-zeebe")
	e.Object["related"] = map[string]any{
		"apiVersion": "apps/v1", "kind": "StatefulSet", "namespace": "team-a", "name": "orders-zeebe",
		"uid": "5a1e3c2b-0000-4000-8000-000000000001",
	}

	out, err := normalize(newServiceIPs(), []unstructured.Unstructured{cluster, e})
	require.NoError(t, err)

	for _, obj := range out {
		if isEvent(&obj) {
			uid, _, _ := unstructured.NestedString(obj.Object, "related", "uid")
			assert.Equal(t, scenarioUID("StatefulSet", "team-a", "orders-zeebe"), uid, "the StatefulSet is gone")
		}
	}
}

func TestNormalizeSortsManagedFieldsByTheirScenarioUID(t *testing.T) {
	run := func(ordersUID, billingUID string) []string {
		orders := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", ordersUID)
		billing := object("core.camunda.io/v1", "CamundaCluster", "team-a", "billing", billingUID)
		child := object("v1", "ConfigMap", "team-a", "shared", "c")
		child.SetManagedFields([]metav1.ManagedFieldsEntry{
			{Manager: "CamundaCluster/zeebe/" + ordersUID, Operation: metav1.ManagedFieldsOperationApply},
			{Manager: "CamundaCluster/zeebe/" + billingUID, Operation: metav1.ManagedFieldsOperationApply},
		})
		out, err := normalize(newServiceIPs(), []unstructured.Unstructured{orders, billing, child})
		require.NoError(t, err)

		shared := find(t, out, "ConfigMap", "team-a", "shared")
		managers := make([]string, 0, 2)
		for _, f := range shared.GetManagedFields() {
			managers = append(managers, f.Manager)
		}
		return managers
	}

	assert.Equal(t,
		run("00000000-0000-4000-8000-000000000001", "ffffffff-0000-4000-8000-000000000002"),
		run("ffffffff-0000-4000-8000-000000000003", "00000000-0000-4000-8000-000000000004"),
		"the order of the managers depends on the scenario UIDs only",
	)
}

func TestNormalizeWritesEachEventOnce(t *testing.T) {
	cluster := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "u")
	out, err := normalize(newServiceIPs(), []unstructured.Unstructured{
		cluster,
		event("orders.a", cluster, "Created", "Created StatefulSet orders-zeebe"),
		event("orders.b", cluster, "Created", "Created StatefulSet orders-zeebe"),
		event("orders.c", cluster, "Created", "Created Service orders-zeebe"),
	})
	require.NoError(t, err)

	var events []unstructured.Unstructured
	for _, obj := range out {
		if obj.GetKind() == "Event" {
			events = append(events, obj)
		}
	}
	require.Len(t, events, 2, "one Event per object, reason, and message")
	assert.NotEqual(t, events[0].GetName(), events[1].GetName())
}

func TestNormalizeWritesNoCountAndNoHostOfAnEvent(t *testing.T) {
	cluster := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "u")
	out, err := normalize(
		newServiceIPs(),
		[]unstructured.Unstructured{cluster, event("orders.a", cluster, "Created", "Created")},
	)
	require.NoError(t, err)

	e := find(t, out, "Event", "team-a", out[1].GetName())
	_, hasSeries := e.Object["series"]
	assert.False(t, hasSeries, "the series counts the reconciles that ran")
	assert.Equal(t, "camundacluster-"+scenarioNode, e.Object["reportingInstance"], "the instance holds the host name")
	regarding, _, _ := unstructured.NestedStringMap(e.Object, "regarding")
	assert.NotContains(t, regarding, "resourceVersion")
	assert.Equal(t, scenarioUID("CamundaCluster", "team-a", "orders"), regarding["uid"])
}

func TestNormalizeLeavesOutAnEventAboutAnEarlierObjectOfTheSameName(t *testing.T) {
	earlier := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "earlier")
	current := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "current")
	out, err := normalize(
		newServiceIPs(),
		[]unstructured.Unstructured{current, event("orders.a", earlier, "Created", "Created")},
	)
	require.NoError(t, err)

	assert.Len(t, out, 1, "the Event is about an object that a state deleted")
}

func TestNormalizeLeavesOutAnEventAboutAnObjectOutsideTheScenario(t *testing.T) {
	cluster := object("core.camunda.io/v1", "CamundaCluster", "team-a", "orders", "u")
	gone := object("v1", "Pod", "team-a", "orders-zeebe-0", "u2")
	out, err := normalize(
		newServiceIPs(),
		[]unstructured.Unstructured{cluster, event("pod.a", gone, "Killing", "Stopping")},
	)
	require.NoError(t, err)

	assert.Len(t, out, 1)
}

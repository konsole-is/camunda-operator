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
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// scenarioTime is the one time of every output: each timestamp that the
// API server, a reconciler, or the simulator writes becomes this time.
var scenarioTime = metav1.NewTime(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))

// secretValue is the one value of every key of every Secret in the outputs.
const secretValue = "scenario"

// scenarioNode is the node that every Pod of a scenario runs on. The
// reporting instance of each Event names it in place of the host name.
const scenarioNode = "scenario-node"

// eventAPIVersion is the API of the Events that the event recorders of the
// reconcilers write.
const eventAPIVersion = "events.k8s.io/v1"

// The kinds that the generator treats apart from the others.
const (
	kindEvent     = "Event"
	kindNamespace = "Namespace"
)

// normalize returns objs in a form that depends only on the scenario, so a
// second run writes the same bytes:
//
//  1. An Event stays only when the object it is about is in objs. Its name
//     comes from that object, its reason, and its note, and one Event stays
//     for each of them. It has no series, and its reporting instance names
//     scenarioNode in place of the host.
//  2. The UID of each object comes from scenarioUID. Each occurrence of a
//     real UID anywhere in objs, such as an owner reference or the name of
//     a field manager, becomes the UID of the object that holds it.
//  3. metadata.resourceVersion is removed, and so is the resource version
//     of the object that an Event is about.
//  4. Each RFC 3339 string becomes scenarioTime, in the layout of the
//     original.
//  5. Each value of a v1 Secret becomes secretValue.
//  6. metadata.managedFields are sorted by manager, operation, and
//     subresource.
//  7. metadata.generation becomes 1, and so does each observedGeneration in
//     status that equals the generation. One that is behind stays.
//  8. The cluster IP of a v1 Service comes from its namespace and name. The
//     API server picks a free IP at random.
//  9. The objects are sorted by apiVersion, kind, namespace, and name.
func normalize(objs []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	objs = normalizeEvents(objs)

	uids := map[string]string{}
	for i := range objs {
		if uid := string(objs[i].GetUID()); uid != "" {
			uids[uid] = scenarioUID(objs[i].GetKind(), objs[i].GetNamespace(), objs[i].GetName())
		}
	}

	out := make([]unstructured.Unstructured, 0, len(objs))
	for i := range objs {
		obj, err := normalizeObject(objs[i].DeepCopy(), uids)
		if err != nil {
			return nil, fmt.Errorf("normalizing %s: %w", describe(&objs[i]), err)
		}
		out = append(out, *obj)
	}

	slices.SortFunc(out, func(a, b unstructured.Unstructured) int {
		return cmp.Or(
			cmp.Compare(a.GetAPIVersion(), b.GetAPIVersion()),
			cmp.Compare(a.GetKind(), b.GetKind()),
			cmp.Compare(a.GetNamespace(), b.GetNamespace()),
			cmp.Compare(a.GetName(), b.GetName()),
		)
	})

	return out, nil
}

// scenarioUID returns the UID that an object of kind, namespace, and name
// has in every output.
func scenarioUID(kind, namespace, name string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(kind+"/"+namespace+"/"+name)).String()
}

// describe returns "<Kind> <namespace>/<name>", or "<Kind> <name>" for a
// cluster-scoped object.
func describe(obj *unstructured.Unstructured) string {
	if obj.GetNamespace() == "" {
		return obj.GetKind() + " " + obj.GetName()
	}
	return obj.GetKind() + " " + obj.GetNamespace() + "/" + obj.GetName()
}

// normalizeEvents returns objs with rule 1 of normalize applied. It changes
// copies, never an object of objs.
func normalizeEvents(objs []unstructured.Unstructured) []unstructured.Unstructured {
	held := map[string]bool{}
	for i := range objs {
		if !isEvent(&objs[i]) {
			held[objs[i].GetKind()+"/"+objs[i].GetNamespace()+"/"+objs[i].GetName()] = true
		}
	}

	out := make([]unstructured.Unstructured, 0, len(objs))
	seen := map[string]bool{}
	for i := range objs {
		if !isEvent(&objs[i]) {
			out = append(out, objs[i])
			continue
		}

		e := objs[i].DeepCopy()
		kind, _, _ := unstructured.NestedString(e.Object, "regarding", "kind")
		namespace, _, _ := unstructured.NestedString(e.Object, "regarding", "namespace")
		name, _, _ := unstructured.NestedString(e.Object, "regarding", "name")
		if !held[kind+"/"+namespace+"/"+name] {
			continue
		}

		reason, _, _ := unstructured.NestedString(e.Object, "reason")
		note, _, _ := unstructured.NestedString(e.Object, "note")
		sum := sha256.Sum256([]byte(strings.Join([]string{kind, namespace, name, reason, note}, "/")))
		e.SetName(name + "." + hex.EncodeToString(sum[:8]))
		if seen[e.GetName()] {
			continue
		}
		seen[e.GetName()] = true

		delete(e.Object, "series")
		if controller, ok := e.Object["reportingController"].(string); ok {
			e.Object["reportingInstance"] = controller + "-" + scenarioNode
		}
		unstructured.RemoveNestedField(e.Object, "regarding", "resourceVersion")
		unstructured.RemoveNestedField(e.Object, "related", "resourceVersion")
		out = append(out, *e)
	}

	return out
}

// isEvent reports whether obj is an Event of the events.k8s.io API.
func isEvent(obj *unstructured.Unstructured) bool {
	return obj.GetAPIVersion() == eventAPIVersion && obj.GetKind() == kindEvent
}

// normalizeObject applies rules 2 to 8 of normalize to obj, a copy that it
// may change. uids maps each real UID to its scenario UID.
func normalizeObject(obj *unstructured.Unstructured, uids map[string]string) (*unstructured.Unstructured, error) {
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")

	if obj.GetAPIVersion() == "v1" && obj.GetKind() == "Secret" {
		if data, ok := obj.Object["data"].(map[string]any); ok {
			for key := range data {
				data[key] = base64.StdEncoding.EncodeToString([]byte(secretValue))
			}
		}
	}

	if obj.GetAPIVersion() == "v1" && obj.GetKind() == "Service" {
		setClusterIP(obj)
	}

	fields := obj.GetManagedFields()
	slices.SortFunc(fields, func(a, b metav1.ManagedFieldsEntry) int {
		return cmp.Or(
			cmp.Compare(a.Manager, b.Manager),
			cmp.Compare(a.Operation, b.Operation),
			cmp.Compare(a.Subresource, b.Subresource),
		)
	})
	obj.SetManagedFields(fields)

	if generation := obj.GetGeneration(); generation != 0 {
		if status, ok := obj.Object["status"]; ok {
			setObservedGeneration(status, generation)
		}
		obj.SetGeneration(1)
	}

	data, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}
	for real, derived := range uids {
		data = bytes.ReplaceAll(data, []byte(real), []byte(derived))
	}

	out := &unstructured.Unstructured{}
	if err := out.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	fixTimes(out.Object)

	return out, nil
}

// setClusterIP sets the cluster IP of the Service obj to an IP of
// 10.96.0.0/16 that comes from its namespace and name. A headless Service
// keeps None.
func setClusterIP(obj *unstructured.Unstructured) {
	ip, _, _ := unstructured.NestedString(obj.Object, "spec", "clusterIP")
	if ip == "" || ip == corev1.ClusterIPNone {
		return
	}

	sum := sha256.Sum256([]byte(obj.GetNamespace() + "/" + obj.GetName()))
	ip = fmt.Sprintf("10.96.%d.%d", sum[0], max(sum[1], 1))
	_ = unstructured.SetNestedField(obj.Object, ip, "spec", "clusterIP")
	_ = unstructured.SetNestedStringSlice(obj.Object, []string{ip}, "spec", "clusterIPs")
}

// setObservedGeneration sets each observedGeneration under v that equals
// generation to 1.
func setObservedGeneration(v any, generation int64) {
	switch value := v.(type) {
	case map[string]any:
		for k, e := range value {
			if observed, ok := e.(int64); ok && k == "observedGeneration" && observed == generation {
				value[k] = int64(1)
				continue
			}
			setObservedGeneration(e, generation)
		}
	case []any:
		for _, e := range value {
			setObservedGeneration(e, generation)
		}
	}
}

// fixTimes sets every RFC 3339 string under v to scenarioTime, in the layout
// of the original, so a MicroTime field stays a MicroTime.
func fixTimes(v any) any {
	switch value := v.(type) {
	case map[string]any:
		for k, e := range value {
			value[k] = fixTimes(e)
		}
	case []any:
		for i, e := range value {
			value[i] = fixTimes(e)
		}
	case string:
		if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
			if strings.Contains(value, ".") {
				return scenarioTime.UTC().Format(metav1.RFC3339Micro)
			}
			return scenarioTime.UTC().Format(time.RFC3339)
		}
	}
	return v
}

# Version Downgrade Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the `CamundaCluster` controller refuse an effective version below the one the brokers run, unless the annotation `camunda.io/allow-version-downgrade` names the target version, and make the restore set that annotation in the apply that writes `spec.version`.

**Architecture:** A pure rule in `pkg/components/camundacluster` decides whether a version pair is a downgrade. The controller reads the running version off the applied broker StatefulSet it already fetches, runs the rule after the pre-checks, and on a refusal stages `Ready: False / VersionDowngradeRefused`, records one Warning event, and applies nothing. The controller also consumes the annotation with a merge patch once the StatefulSet carries the version it names. `pkg/restore` adds the annotation to its existing single-field version apply.

**Tech Stack:** Go 1.26, controller-runtime v0.24, ocf v0.19.1, Ginkgo/Gomega envtest for the controller, testify for pure units, mkdocs for docs.

**Spec:** `docs/superpowers/specs/2026-08-22-version-downgrade-guard-design.md`

## Global Constraints

- Annotation key: `camunda.io/allow-version-downgrade`. Value: the exact target version, `x.y.z`.
- Condition reason: `VersionDowngradeRefused`, declared in `api/v1/camundacluster_types.go`.
- Event: type Warning, reason `VersionDowngradeRefused`, once per distinct refusal.
- The running version is the tag of the `camunda` container image on the applied broker StatefulSet, read without the cache. No StatefulSet, or a tag that is not `x.y.z`, means no rule.
- The restore adds the annotation to the apply under `camunda-operator/restore-version`. The #157 order is untouched. `PointInTimeRestore` is untouched.
- Prose (GoDoc, CRD descriptions, docs, messages) obeys `simple-english:simple-english`. Go obeys `how-we-write-go`. Load both before each task.
- Commit after each task with a conventional message ending in the session trailers the harness prescribes.
- Gates before the PR: `make setup-envtest`, `go test ./...`, `go -C api test ./...`, `make lint`, `make manifests generate` with a clean `git status --porcelain config api`, `go vet -tags=e2e ./test/e2e/`, `mkdocs build --strict`.

## Contracts

Single PR, sequential tasks. No cross-task contracts beyond the interfaces each task declares.

## Conventions

- Names: `components.AllowVersionDowngradeAnnotation`, `components.ImageTag`, `components.VersionDowngrade`, `v1.ReasonVersionDowngradeRefused`, controller file `internal/controller/camundacluster/downgrade.go`, controller functions `refuseDowngrade`, `recordRefusedDowngrade`, `consumeDowngradeSanction`, `brokerStorage.runningVersion`.
- The controller package alias for `pkg/components/camundacluster` is `components`, as everywhere else.
- Messages name the cause, then the remedy. No semicolons. "Camunda does not support a downgrade of a running cluster" is the one sentence the condition, the event, and the docs share.
- Tests encode intent. A refusal test asserts reason and the remedy text, not the full message.

---

### Task 1: The reason constant and the `Version` GoDoc

**Files:**
- Modify: `api/v1/camundacluster_types.go` (reasons at lines 52-68, `Version` at 365-371)
- Regenerate: `config/crd/bases/core.camunda.io_camundaclusters.yaml`, `config/crd/bases/core.camunda.io_camundaclusterpresets.yaml` (the preset embeds the spec)

**Interfaces:**
- Produces: `v1.ReasonVersionDowngradeRefused = "VersionDowngradeRefused"`.

- [ ] **Step 1: Add the reason next to the other per-CRD reasons**

After `ReasonRejected` in `api/v1/camundacluster_types.go`:

```go
// ReasonVersionDowngradeRefused on Ready means that the effective version
// of the cluster is below the version that its brokers run, and nothing
// sanctions the move. Camunda does not support a downgrade of a running
// cluster. The operator applies nothing while this stands. The message names
// the two versions and the remedies. The annotation
// camunda.io/allow-version-downgrade, with the target version as its value,
// sanctions one such move.
const ReasonVersionDowngradeRefused = "VersionDowngradeRefused"
```

- [ ] **Step 2: Extend the `Version` GoDoc**

Replace the comment on `Version`:

```go
	// Version is the Camunda version to deploy, as a full semantic version.
	// The floor of 8.9.0 is enforced by the controller on the preset-merged
	// result, the schema pins only the three-segment shape. Required unless
	// the resolved preset provides it. A value below the version that the
	// brokers run is refused with Ready VersionDowngradeRefused unless the
	// annotation camunda.io/allow-version-downgrade names it.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Version string `json:"version,omitempty"`
```

- [ ] **Step 3: Regenerate and check**

Run: `make manifests generate && git status --porcelain config api`
Expected: the two CRD yaml files and the types file are modified, nothing else.

Run: `go -C api build ./... && go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add api/v1/camundacluster_types.go config/crd/bases
git commit -m "feat(api): add VersionDowngradeRefused and name the guard on spec.version"
```

---

### Task 2: The pure rule and the shared image-tag helper

**Files:**
- Create: `pkg/components/camundacluster/downgrade.go`
- Create: `pkg/components/camundacluster/downgrade_test.go`
- Modify: `pkg/components/camundacluster/names.go:36-48` (annotation constant)
- Modify: `pkg/restore/target.go:124,240-251` (call the moved helper, delete `imageTag`)

**Interfaces:**
- Consumes: `parseVersion(string) ([3]int, error)` in `pkg/components/camundacluster/presetmerge.go:371`.
- Produces:
  - `components.AllowVersionDowngradeAnnotation = "camunda.io/allow-version-downgrade"`
  - `func ImageTag(image string) string`
  - `func VersionDowngrade(effective, running string) bool`

- [ ] **Step 1: Write the failing tests**

`pkg/components/camundacluster/downgrade_test.go`:

```go
package camundacluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionDowngrade(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		effective, running string
		want               bool
	}{
		"patch below":             {"8.9.8", "8.9.9", true},
		"minor below":             {"8.9.9", "8.10.0", true},
		"major below":             {"8.10.0", "9.0.0", true},
		"same version":            {"8.9.9", "8.9.9", false},
		"patch above":             {"8.9.10", "8.9.9", false},
		"minor above":             {"8.10.0", "8.9.9", false},
		"no running version":      {"8.9.8", "", false},
		"running tag not x.y.z":   {"8.9.8", "latest", false},
		"effective not x.y.z":     {"8.9", "8.9.9", false},
		"numeric, not lexical":    {"8.9.10", "8.9.9", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, VersionDowngrade(tc.effective, tc.running))
		})
	}
}

func TestImageTag(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ image, want string }{
		"plain":              {"camunda/camunda:8.9.9", "8.9.9"},
		"registry with port": {"registry.example.com:5000/camunda/camunda:8.9.9", "8.9.9"},
		"digest after tag":   {"camunda/camunda:8.9.9@sha256:abc", "8.9.9"},
		"no tag":             {"camunda/camunda", ""},
		"port but no tag":    {"registry.example.com:5000/camunda/camunda", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ImageTag(tc.image))
		})
	}
}
```

- [ ] **Step 2: Run the tests, see them fail**

Run: `go test ./pkg/components/camundacluster/ -run 'TestVersionDowngrade|TestImageTag'`
Expected: compile error, `VersionDowngrade` and `ImageTag` undefined.

- [ ] **Step 3: Add the annotation constant**

In `pkg/components/camundacluster/names.go`, inside the `const (` block that holds `ConfigHashAnnotation`, after `RequestedStorageSizeAnnotation`:

```go
	// AllowVersionDowngradeAnnotation is the annotation of a CamundaCluster
	// that sanctions one move of its effective version below the version its
	// brokers run. The value is the exact target version, x.y.z. The
	// controller removes the annotation once the brokers carry that version.
	AllowVersionDowngradeAnnotation = "camunda.io/allow-version-downgrade"
```

- [ ] **Step 4: Write `downgrade.go`**

```go
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

import "strings"

// VersionDowngrade reports whether effective is below running, segment by
// segment. It reports false when either value is not of the form x.y.z: a
// cluster without a running version has nothing to move back from, and an
// effective version that is not x.y.z is refused by ValidateMerged before
// this rule runs.
func VersionDowngrade(effective, running string) bool {
	want, err := parseVersion(effective)
	if err != nil {
		return false
	}
	have, err := parseVersion(running)
	if err != nil {
		return false
	}

	for i := range 3 {
		switch {
		case want[i] < have[i]:
			return true
		case want[i] > have[i]:
			return false
		}
	}

	return false
}

// ImageTag returns the tag of an image reference, or the empty string when
// it carries none. A digest follows the tag after an "@", and a registry
// host can carry a port, so only a colon after the last slash and before the
// digest starts a tag.
func ImageTag(image string) string {
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}

	colon := strings.LastIndex(image, ":")
	if colon < 0 || colon < strings.LastIndex(image, "/") {
		return ""
	}

	return image[colon+1:]
}
```

- [ ] **Step 5: Run the tests, see them pass**

Run: `go test ./pkg/components/camundacluster/ -run 'TestVersionDowngrade|TestImageTag'`
Expected: PASS.

- [ ] **Step 6: Point the restore at the shared helper**

In `pkg/restore/target.go`:
- Line 124: `version := imageTag(broker.Image)` becomes `version := components.ImageTag(broker.Image)`.
- Delete the `imageTag` function (lines 236-251) and its comment.
- If `strings` is now unused in `target.go`, remove the import.

Run: `go build ./... && go test ./pkg/restore/`
Expected: success. (`pkg/restore` already imports `pkg/components/camundacluster` as `components`.)

- [ ] **Step 7: Commit**

```bash
git add pkg/components/camundacluster/downgrade.go pkg/components/camundacluster/downgrade_test.go pkg/components/camundacluster/names.go pkg/restore/target.go
git commit -m "feat(camundacluster): add the version downgrade rule and share the image tag helper"
```

---

### Task 3: The controller guard, the event, and the consumption

**Files:**
- Create: `internal/controller/camundacluster/downgrade.go`
- Modify: `internal/controller/camundacluster/storage.go` (add `runningVersion` after `volumes()` or near `requestedSizeApplied`)
- Modify: `internal/controller/camundacluster/controller.go:195-204` (wire the two calls after `readBrokerStorage`)
- Test: `internal/controller/camundacluster/controller_test.go` (three new `It` blocks)

**Interfaces:**
- Consumes: `components.VersionDowngrade`, `components.ImageTag`, `components.AllowVersionDowngradeAnnotation`, `components.ContainerCamunda`, `v1.ReasonVersionDowngradeRefused`, `conditions.PreCheckFailure`, `conditions.Failed`, `conditions.Stage`, `brokerStorage.statefulSet`, `r.EventRecorder`, `r.Patch`.
- Produces:
  - `func (s brokerStorage) runningVersion() string`
  - `func refuseDowngrade(cluster *v1.CamundaCluster, in components.Input, storage brokerStorage) *conditions.PreCheckFailure`
  - `func (r *CamundaClusterReconciler) recordRefusedDowngrade(cluster *v1.CamundaCluster, refused metav1.Condition)`
  - `func (r *CamundaClusterReconciler) consumeDowngradeSanction(ctx context.Context, cluster *v1.CamundaCluster, storage brokerStorage) error`

- [ ] **Step 1: Write the failing envtest specs**

Append inside `var _ = Describe("CamundaCluster controller", func() { ... })` in `controller_test.go`, after the "suspends every workload" spec:

```go
	It("refuses a version below the one the brokers run, and applies a sanctioned one", func() {
		cluster := createDefaultCluster()
		Expect(zeebeContainer(cluster).Image).To(HaveSuffix(":8.9.9"))

		By("lowering spec.version")
		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Version = "8.9.8" })
		expectReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonVersionDowngradeRefused),
			And(
				ContainSubstring("8.9.8 is below the running version 8.9.9"),
				ContainSubstring(components.AllowVersionDowngradeAnnotation+"="),
			),
		)
		expectEvent(cluster, v1.ReasonVersionDowngradeRefused, corev1.EventTypeWarning)
		Consistently(func() string { return zeebeContainer(cluster).Image }, "2s", interval).
			Should(HaveSuffix(":8.9.9"), "the refusal applied the lower image")

		By("sanctioning the move with the annotation")
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			if c.Annotations == nil {
				c.Annotations = map[string]string{}
			}
			c.Annotations[components.AllowVersionDowngradeAnnotation] = "8.9.8"
		})
		Eventually(func(g Gomega) {
			g.Expect(zeebeContainer(cluster).Image).To(HaveSuffix(":8.9.8"))
		}, timeout, interval).Should(Succeed())

		By("consuming the annotation once the brokers carry the version")
		Eventually(func(g Gomega) {
			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			g.Expect(latest.Annotations).NotTo(HaveKey(components.AllowVersionDowngradeAnnotation))
		}, timeout, interval).Should(Succeed())
		expectReady(cluster, metav1.ConditionTrue, Not(Equal(v1.ReasonVersionDowngradeRefused)), Not(BeEmpty()))
	})

	It("refuses a removed spec.version when the preset carries a lower one", func() {
		ns := newNamespace()
		preset := minimalPreset()
		preset.Spec.Cluster.Version = "8.9.8"
		Expect(k8sClient.Create(ctx, preset)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.PresetRef = preset.Name
		createCluster(cluster)
		Expect(zeebeContainer(cluster).Image).To(HaveSuffix(":8.9.9"))

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Version = "" })
		expectReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonVersionDowngradeRefused),
			ContainSubstring("8.9.8 is below the running version 8.9.9"),
		)
	})

	It("refuses a preset whose version is lowered under a running cluster", func() {
		ns := newNamespace()
		preset := minimalPreset()
		preset.Spec.Cluster.Version = "8.9.9"
		Expect(k8sClient.Create(ctx, preset)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.Version = ""
		createCluster(cluster)
		Expect(zeebeContainer(cluster).Image).To(HaveSuffix(":8.9.9"))

		Eventually(func(g Gomega) {
			var latest v1.CamundaClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.Version = "8.9.8"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		expectReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonVersionDowngradeRefused),
			ContainSubstring("8.9.8 is below the running version 8.9.9"),
		)
	})
```

Check the imports of `controller_test.go`: it needs `components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"` if not already imported (`grep -n 'components "' internal/controller/camundacluster/controller_test.go`). `minimalPreset()` exists in the package tests already.

- [ ] **Step 2: Run the new specs, see them fail**

Run: `make setup-envtest` once, then
`KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path --bin-dir bin)" go test ./internal/controller/camundacluster/ -ginkgo.focus "refuses a" -count=1`
(Adjust to however `make test` sets `KUBEBUILDER_ASSETS` in this repo: `grep -n KUBEBUILDER_ASSETS Makefile`.)
Expected: the first spec fails at `expectReady` with reason `Healthy` or `AliveUpdating` rather than `VersionDowngradeRefused`.

- [ ] **Step 3: Add `runningVersion` to `brokerStorage`**

In `internal/controller/camundacluster/storage.go`, after `requestedSizeApplied`:

```go
// runningVersion returns the tag of the broker image on the applied
// StatefulSet, or the empty string before the first apply or when the
// container carries no tag. This is the version that the next broker start
// runs, whatever spec.version says.
func (s brokerStorage) runningVersion() string {
	if s.statefulSet == nil {
		return ""
	}

	for _, container := range s.statefulSet.Spec.Template.Spec.Containers {
		if container.Name == components.ContainerCamunda {
			return components.ImageTag(container.Image)
		}
	}

	return ""
}
```

- [ ] **Step 4: Write the controller file**

`internal/controller/camundacluster/downgrade.go`:

```go
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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// eventReasonVersionDowngradeRefused is recorded once per distinct refusal of
// an effective version below the running one. The Ready condition carries the
// same reason for as long as the refusal stands.
const eventReasonVersionDowngradeRefused = v1.ReasonVersionDowngradeRefused

const msgVersionDowngradeRefused = "the effective version %s is below the running version %s. " +
	"Camunda does not support a downgrade of a running cluster: a broker that starts over data of a " +
	"newer version marks itself unhealthy. Set the version to %s or later, restore a backup taken " +
	"with %s, or set the annotation %s=%q on the cluster to downgrade on purpose"

// refuseDowngrade returns the pre-check failure for an effective version
// below the version that the brokers run, or nil when the move is not a
// downgrade or the annotation sanctions it. The caller applies nothing on a
// failure, so the brokers keep the version they have.
func refuseDowngrade(
	cluster *v1.CamundaCluster,
	in components.Input,
	storage brokerStorage,
) *conditions.PreCheckFailure {
	effective := in.Effective.Version
	running := storage.runningVersion()
	if !components.VersionDowngrade(effective, running) ||
		cluster.Annotations[components.AllowVersionDowngradeAnnotation] == effective {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonVersionDowngradeRefused,
		Message: fmt.Sprintf(
			msgVersionDowngradeRefused,
			effective, running, running, effective,
			components.AllowVersionDowngradeAnnotation, effective,
		),
	}
}

// recordRefusedDowngrade records the Warning event for refused unless the
// Ready condition that the server holds already carries this exact refusal.
// The caller stages refused afterwards, so the next reconcile finds it and
// records nothing.
func (r *CamundaClusterReconciler) recordRefusedDowngrade(cluster *v1.CamundaCluster, refused metav1.Condition) {
	ready := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionReady)
	if ready != nil && ready.Reason == refused.Reason && ready.Message == refused.Message {
		return
	}

	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeWarning,
		eventReasonVersionDowngradeRefused,
		eventActionReconcile,
		"%s",
		refused.Message,
	)
}

// consumeDowngradeSanction removes the downgrade annotation from cluster once
// the brokers carry the version it names. The sanction is spent at that
// point, whoever set it. A merge patch removes the key from every manager
// that declared it, which is what a spent sanction needs.
func (r *CamundaClusterReconciler) consumeDowngradeSanction(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	storage brokerStorage,
) error {
	sanctioned, ok := cluster.Annotations[components.AllowVersionDowngradeAnnotation]
	if !ok || sanctioned != storage.runningVersion() {
		return nil
	}

	before := cluster.DeepCopy()
	delete(cluster.Annotations, components.AllowVersionDowngradeAnnotation)
	if err := r.Patch(ctx, cluster, client.MergeFrom(before)); err != nil {
		return fmt.Errorf(
			"removing the %s annotation of CamundaCluster %q: %w",
			components.AllowVersionDowngradeAnnotation, cluster.Name, err,
		)
	}

	return nil
}
```

Check that `eventActionReconcile` exists in `controller.go` (`grep -n eventActionReconcile internal/controller/camundacluster/controller.go`). It does, at the `Paused` event.

- [ ] **Step 5: Wire the reconcile**

In `internal/controller/camundacluster/controller.go`, right after

```go
	storage, err := r.readBrokerStorage(ctx, &cluster)
	if err != nil {
		return ctrl.Result{}, err
	}
```

insert:

```go
	if err := r.consumeDowngradeSanction(ctx, &cluster, storage); err != nil {
		return ctrl.Result{}, err
	}

	// A refused downgrade re-enqueues through the watches on the cluster, the
	// preset, and the owned StatefulSet, so no timer is needed.
	if failure := refuseDowngrade(&cluster, in, storage); failure != nil {
		refused := conditions.Failed(&cluster, failure)
		r.recordRefusedDowngrade(&cluster, refused)
		conditions.Stage(&cluster, refused)

		return ctrl.Result{}, nil
	}
```

Then update the `Reconcile` GoDoc (lines ~122-142) with one sentence: "An effective version below the one the brokers run is refused before anything is applied, unless the annotation `camunda.io/allow-version-downgrade` names it, and the annotation is removed once the brokers carry that version."

- [ ] **Step 6: Run the specs, see them pass**

Run the focused envtest command from Step 2.
Expected: the three new specs PASS.

Then run the whole package: `go test ./internal/controller/camundacluster/ -count=1` (with `KUBEBUILDER_ASSETS`).
Expected: PASS, no regression in the suspend or preset specs.

- [ ] **Step 7: Lint**

Run: `make lint`
Expected: 0 issues. Fix `hack/callsplit` or `golines` complaints by reshaping the calls as `how-we-write-go` shows.

- [ ] **Step 8: Commit**

```bash
git add internal/controller/camundacluster/downgrade.go internal/controller/camundacluster/storage.go internal/controller/camundacluster/controller.go internal/controller/camundacluster/controller_test.go
git commit -m "feat(camundacluster): refuse a version downgrade unless the annotation sanctions it"
```

---

### Task 4: The restore sanctions its own version write

**Files:**
- Modify: `pkg/restore/prepare.go` (`applyVersion` at lines ~413-426, `Prepare` GoDoc at ~84-107, `versionTarget` GoDoc at ~236-267)
- Test: `pkg/restore/prepare_test.go` (`TestPrepareWritesTheVersionOfTheBackup` at ~230)

**Interfaces:**
- Consumes: `components.AllowVersionDowngradeAnnotation`, `targetPatch`, `Apply`, `FieldManagerTargetVersion`.

- [ ] **Step 1: Extend the failing test**

In `TestPrepareWritesTheVersionOfTheBackup`, after the `Spec.Suspend` assertion add:

```go
	assert.Equal(
		t, "8.9.8", (*w.applies)[0].cluster.Annotations[components.AllowVersionDowngradeAnnotation],
		"the version apply sanctions the downgrade it performs",
	)
	assert.Equal(t, "8.9.8", w.live(t).Annotations[components.AllowVersionDowngradeAnnotation])
```

Import `components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"` in `prepare_test.go` if it is not imported yet.

- [ ] **Step 2: Run, see it fail**

Run: `go test ./pkg/restore/ -run TestPrepareWritesTheVersionOfTheBackup`
Expected: FAIL, the annotation is empty.

- [ ] **Step 3: Add the annotation to the apply**

Replace `applyVersion`:

```go
// applyVersion applies spec.version of the cluster under the version manager,
// together with the annotation that sanctions the downgrade it can be. The
// cluster controller refuses a version below the one the brokers run without
// that annotation, and it removes the annotation once the brokers carry the
// version. One apply carries both, so no crash leaves the version without its
// sanction.
func applyVersion(
	ctx context.Context,
	c client.Client,
	cluster types.NamespacedName,
	uid types.UID,
	version string,
) error {
	patch := targetPatch(cluster, uid, v1.CamundaClusterSpec{Version: version})
	patch.Annotations = map[string]string{components.AllowVersionDowngradeAnnotation: version}
	if err := Apply(ctx, c, patch, FieldManagerTargetVersion); err != nil {
		return fmt.Errorf("setting spec.version of CamundaCluster %s to %s: %w", cluster, version, err)
	}

	return nil
}
```

Import `components` in `prepare.go` if it is not there (`grep -n 'components "' pkg/restore/prepare.go`).

- [ ] **Step 4: Adjust the two GoDocs**

In the `Prepare` GoDoc, after the sentence "No broker performs that comparison here.", add: "The cluster controller refuses a downgrade on its own, and the version apply carries the annotation that sanctions this one."

In the `versionTarget` GoDoc, no change is needed unless a sentence now reads wrong. Read it once.

- [ ] **Step 5: Run, see it pass**

Run: `go test ./pkg/restore/ -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/restore/prepare.go pkg/restore/prepare_test.go
git commit -m "feat(restore): sanction the version write with the downgrade annotation"
```

---

### Task 5: Documentation

**Files:**
- Modify: `docs/crds/camundacluster.md` (new `## Version` section before `## Storage` at line 56; validation rules at ~404-421)
- Modify: `docs/crds/logicalrestoreelasticsearch.md:78-104`
- Modify: `docs/crds/logicalrestorerdbms.md:53-79`
- Modify: `docs/crds/pointintimerestore.md:58`
- Modify: `docs/guides/backup.md:335-342`
- Modify: `docs/guides/presets.md` ("Change a fleet")

Load `simple-english:simple-english` and `feature-dev-workflow:writing-docs` before this task. Keep each sentence under 25 words, no semicolons, conditions before commands, CAUTION first.

- [ ] **Step 1: `docs/crds/camundacluster.md`, new section**

Insert before `## Storage`:

````markdown
## Version

`spec.version`, or the version of the preset when the field is absent, is the version the operator
deploys. A new version rolls every workload.

The operator refuses a version below the one the brokers run. Camunda does not support a downgrade
of a running cluster: a broker that starts over data of a newer version marks itself unhealthy. The
cluster then reports `Ready: False` with reason `VersionDowngradeRefused`, records a Warning event of
the same reason, and applies nothing. The brokers keep the version they have. The message names the
two versions and the remedies.

The rule reads the effective version, so it covers three edits the same way: a lower `spec.version`,
a removed `spec.version` when the preset carries a lower version, and a preset whose version is
lowered. The running version is the tag of the broker image on the broker StatefulSet.

A restore sanctions its own move. The restore pages explain why that move is safe.

### Downgrade on purpose

CAUTION: A downgrade over data that a newer version wrote leaves every broker unhealthy. The
remedies are to go forward again or to restore a backup taken with the lower version.

To downgrade on purpose, set the annotation `camunda.io/allow-version-downgrade` to the exact target
version, and set the version:

```yaml
metadata:
  annotations:
    camunda.io/allow-version-downgrade: "8.9.8"
spec:
  version: "8.9.8"
```

The annotation sanctions a move to that version and to no other. The operator removes the annotation
once the brokers carry the version. An annotation that names a version the cluster never moves to
stays until you remove it.

To give the preset control again after a restore, set the annotation to the version of the preset
and remove `spec.version` in the same edit.
````

In "### Validation rules", after the `InvalidReference` list, add:

```markdown
An effective version below the one the brokers run is refused with `Ready: VersionDowngradeRefused`.
See [Version](#version).
```

- [ ] **Step 2: `docs/crds/logicalrestoreelasticsearch.md`**

In the field-manager table, change the `spec.version` row to:

```markdown
| `spec.version` and the annotation `camunda.io/allow-version-downgrade` | `camunda-operator/restore-version` | The restore keeps `spec.version`. The cluster controller removes the annotation once the brokers carry the version. |
```

After the table's paragraph "These names are published...", add:

```markdown
The annotation sanctions the move. The [CamundaCluster](camundacluster.md#version) refuses a version
below the one its brokers run unless the annotation names it, and the restore writes both in one
apply.
```

In the CAUTION about removing the field so the preset supplies it again, append one sentence before the command:

```markdown
If the version of the preset is below the one the brokers run, set the annotation
`camunda.io/allow-version-downgrade` to the version of the preset in the same edit.
```

Replace the CAUTION under "### Why the downgrade is safe here" with:

```markdown
A downgrade that you do by hand on a running cluster, outside a restore, is refused. The cluster
reports `VersionDowngradeRefused`. The [CamundaCluster page](camundacluster.md#downgrade-on-purpose)
explains how to downgrade on purpose, and what it costs.
```

In "### A GitOps tool that owns the CamundaCluster", add a bullet:

```markdown
- A tool that prunes annotations it does not declare removes the sanction before the cluster
  controller consumes it, and the cluster then refuses the version write. Exclude
  `camunda.io/allow-version-downgrade` from pruning for the time of the restore.
```

- [ ] **Step 3: `docs/crds/logicalrestorerdbms.md`**

Apply the same four edits as Step 2 at the corresponding places (table at ~55-57, removal CAUTION at ~68, safety CAUTION at ~79, GitOps section at the end).

- [ ] **Step 4: `docs/crds/pointintimerestore.md`**

After "This kind writes no version. ..." add:

```markdown
It therefore needs no sanction for a downgrade, and it sets no
`camunda.io/allow-version-downgrade` annotation.
```

- [ ] **Step 5: `docs/guides/backup.md`**

Replace the CAUTION in "What an upgrade does to the backups you hold" with:

```markdown
A downgrade that you do by hand on a running cluster, outside a restore, is refused with
`VersionDowngradeRefused`. The [CamundaCluster page](../crds/camundacluster.md#version) states the
rule.
```

In the bullet "The restore keeps `spec.version`...", after "A cluster that took its version from a preset needs the field removed by hand.", add: "If the preset is below the version the brokers run, the removal needs the downgrade annotation too."

- [ ] **Step 6: `docs/guides/presets.md`**

In "Change a fleet", after "A new version rolls the pods of every cluster.", add:

```markdown
A lower version is refused by every cluster that references the preset. Each cluster reports
`Ready: False` with reason `VersionDowngradeRefused` and keeps the version its brokers run. The
[CamundaCluster page](../crds/camundacluster.md#version) states the rule.
```

- [ ] **Step 7: Build the docs**

Run: `mkdocs build --strict`
Expected: exit 0. Anchors `#version` and `#downgrade-on-purpose` resolve.

- [ ] **Step 8: Commit**

```bash
git add docs/crds/camundacluster.md docs/crds/logicalrestoreelasticsearch.md docs/crds/logicalrestorerdbms.md docs/crds/pointintimerestore.md docs/guides/backup.md docs/guides/presets.md
git commit -m "docs: state the version downgrade rule and how to downgrade on purpose"
```

---

### Task 6: End-to-end spec for the refusal

The decision, made here so the executor does not relitigate it: one spec in the existing
`CamundaCluster` flow lowers `spec.version` on the running cluster, asserts
`Ready: False / VersionDowngradeRefused` and the Warning event, restores the version, and asserts
`Ready` recovers. It costs no roll. A sanctioned downgrade over live data would leave the flow's
brokers unhealthy, and a cross-version restore would need a second image, so neither runs end to end.
The envtest specs of Task 3 and the unit tests of Task 4 cover those paths.

**Files:**
- Modify: `test/e2e/camundacluster_test.go` (inside the first `Describe("CamundaCluster", Ordered, ...)`, after the "rotates the admin password" spec and before "suspends every workload")

**Interfaces:**
- Consumes: `apply`, `expectConditionFalse`, `expectReady`, `expectReconciledReady` from `test/e2e/helpers_test.go`; the `cluster` variable of the flow; `utils.Kubectl` or the existing `kubectl` helper used by the flow for events (`grep -n "get events\|Kubectl(" test/e2e/*.go`).

- [ ] **Step 1: Write the spec**

```go
	It("refuses a version below the one the brokers run and names the remedy", func() {
		running := cluster.Spec.Version

		By("lowering spec.version")
		lowered := cluster.DeepCopy()
		lowered.Spec.Version = "8.9.0"
		Expect(apply(lowered)).To(Succeed())
		Eventually(func(g Gomega) {
			expectConditionFalse(
				g, "camundacluster", cluster.Name, cluster.Namespace,
				v1.ConditionReady, v1.ReasonVersionDowngradeRefused,
			)
		}, timeout, interval).Should(Succeed())

		out, err := utils.Kubectl(
			"get", "events", "-n", cluster.Namespace,
			"--field-selector", "reason="+v1.ReasonVersionDowngradeRefused,
			"-o", "name",
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty(), "no VersionDowngradeRefused event was recorded")

		By("setting the version back")
		Expect(apply(cluster)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, "camundacluster", cluster.Name, cluster.Namespace, v1.ReasonHealthy)
		}, timeout, interval).Should(Succeed())
		Expect(cluster.Spec.Version).To(Equal(running))
	})
```

Adapt `utils.Kubectl` to whatever the e2e helpers expose for running kubectl (read `test/utils/kubectl.go`). If `apply` uses server-side apply with a fixed manager, the lowered apply and the restoring apply use the same manager, so the second apply takes the field back. If the flow's `cluster` carries a preset and no `spec.version`, set `lowered.Spec.Version = "8.9.0"` still works: an explicit version wins over the preset.

- [ ] **Step 2: Compile the e2e package**

Run: `go vet -tags=e2e ./test/e2e/`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add test/e2e/camundacluster_test.go
git commit -m "test(e2e): assert a hand downgrade is refused and recovers"
```

---

### Task 7: Gates and the pull request

- [ ] **Step 1: Run every gate**

```bash
make setup-envtest
go test ./...
go -C api test ./...
make lint
make manifests generate && git status --porcelain config api   # prints nothing
go vet -tags=e2e ./test/e2e/
mkdocs build --strict
```

Expected: all green. Fix and amend the relevant task commit if not.

- [ ] **Step 2: Update the orchestration state file and push**

Mark the tasks done in `docs/superpowers/states/2026-08-22-version-downgrade-guard-state.md`, commit, push `feat/version-downgrade-guard`.

- [ ] **Step 3: Open the PR**

Load `feature-dev-workflow:opening-a-pull-request`. Title: `feat(camundacluster): refuse a version downgrade unless a restore sanctioned it`. Body: problem, the controller-side decision and why not a webhook, the annotation and its consumption, the restore change, the docs corrected, the gates run, `Closes #168`. Then load `feature-dev-workflow:copilot-review-loop` and drive the review to clean.

---

## Self-review

- Spec coverage: rule and comparison basis (Tasks 2, 3), refusal with condition and event (Task 3), sanction and consumption (Task 3), restore apply (Task 4), preset and removed-field answer (Task 3 specs, Task 5 docs), CAUTION corrections (Task 5), e2e decision (Task 6), gates (Task 7). The spec's "Documentation" table names `docs/guides/operations.md` only if the page has a place for operator-consumed annotations. It has none, so it is not touched.
- Placeholders: none.
- Type consistency: `refuseDowngrade` returns `*conditions.PreCheckFailure`; `conditions.Failed` takes it and returns `metav1.Condition`; `recordRefusedDowngrade` takes that condition. `brokerStorage.runningVersion` is used by both `refuseDowngrade` and `consumeDowngradeSanction`. `components.ImageTag` is the one tag parser after Task 2.

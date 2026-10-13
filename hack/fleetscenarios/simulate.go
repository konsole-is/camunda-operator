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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The field managers of the writes of the simulator: the controllers of
// kube-controller-manager, and the kubelet for the status of a Pod.
const (
	kubeControllerManager = "kube-controller-manager"
	kubelet               = "kubelet"
)

// pvcProtectionFinalizer keeps a claim that a Pod uses from being deleted.
const pvcProtectionFinalizer = "kubernetes.io/pvc-protection"

// objectKey names the object of a workload step.
type objectKey struct {
	Kind, Namespace, Name string
}

// simulator stands in for kube-controller-manager and the kubelet, which
// envtest does not run. In the namespaces it watches it brings each
// StatefulSet, Deployment, Job, and PersistentVolumeClaim to the state that
// those controllers report in a cluster, with the workload steps applied.
// Every time it writes is scenarioTime, so a pass over objects that it has
// already brought to that state writes nothing.
type simulator struct {
	client     client.Client
	namespaces []string
	steps      map[objectKey]workloadStep
	// matched holds each step whose object missingSteps found once. A later
	// state may delete that object.
	matched map[objectKey]bool
}

// newSimulator returns a simulator that writes through c and watches no
// namespace yet.
func newSimulator(c client.Client) *simulator {
	return &simulator{client: c, steps: map[objectKey]workloadStep{}, matched: map[objectKey]bool{}}
}

// watch adds namespaces to the namespaces that a pass covers.
func (s *simulator) watch(namespaces ...string) {
	for _, ns := range namespaces {
		if !slices.Contains(s.namespaces, ns) {
			s.namespaces = append(s.namespaces, ns)
		}
	}
}

// add applies steps from the next pass on. A step replaces the step of the
// same object.
func (s *simulator) add(steps []workloadStep) {
	for _, step := range steps {
		key := objectKey{step.Kind, step.Namespace, step.Name}
		s.steps[key] = step
		delete(s.matched, key)
	}
}

// pass makes one round over the watched namespaces. It returns the number of
// writes it made, so a caller knows when the simulator has nothing to do.
func (s *simulator) pass(ctx context.Context) (int, error) {
	writes := 0
	for _, ns := range s.namespaces {
		for _, step := range []func(context.Context, string) (int, error){
			s.statefulSets, s.deployments, s.jobs, s.claims,
		} {
			n, err := step(ctx, ns)
			writes += n
			if err != nil {
				return writes, err
			}
		}
	}
	return writes, nil
}

// statefulSets brings each StatefulSet of ns, its claims, and its Pods to
// the state of its current template.
func (s *simulator) statefulSets(ctx context.Context, ns string) (int, error) {
	var list appsv1.StatefulSetList
	if err := s.client.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return 0, fmt.Errorf("listing the StatefulSets of %s: %w", ns, err)
	}

	writes := 0
	for i := range list.Items {
		n, err := s.statefulSet(ctx, &list.Items[i])
		writes += n
		if err != nil {
			return writes, fmt.Errorf("simulating StatefulSet %s/%s: %w", ns, list.Items[i].Name, err)
		}
	}
	return writes, nil
}

// statefulSet brings sts, its claims, and its Pods to the state of its
// current template.
func (s *simulator) statefulSet(ctx context.Context, sts *appsv1.StatefulSet) (int, error) {
	replicas := ptr.Deref(sts.Spec.Replicas, 1)
	ready := s.readyCount(kindStatefulSet, sts.Namespace, sts.Name, replicas)
	revision := sts.Name + "-" + templateHash(sts.Spec.Template, sts)
	writes := 0

	pods, err := s.ownedPods(ctx, sts.Namespace, "StatefulSet", sts.Name)
	if err != nil {
		return writes, err
	}
	for i := range pods {
		ordinal, err := strconv.Atoi(strings.TrimPrefix(pods[i].Name, sts.Name+"-"))
		if err == nil && int32(ordinal) < replicas {
			continue
		}
		if err := s.deletePod(ctx, &pods[i]); err != nil {
			return writes, err
		}
		writes++
	}

	for ordinal := range replicas {
		for _, template := range sts.Spec.VolumeClaimTemplates {
			created, err := s.createIfMissing(ctx, statefulSetClaim(sts, template, ordinal))
			if created {
				writes++
			}
			if err != nil {
				return writes, err
			}
		}

		n, err := s.statefulSetPod(ctx, sts, revision, ordinal, ordinal < ready)
		writes += n
		if err != nil {
			return writes, err
		}
	}

	status := *sts.Status.DeepCopy()
	status.ObservedGeneration = sts.Generation
	status.Replicas = replicas
	status.ReadyReplicas = ready
	status.AvailableReplicas = ready
	status.CurrentReplicas = replicas
	status.UpdatedReplicas = replicas
	status.CurrentRevision = revision
	status.UpdateRevision = revision
	if equality.Semantic.DeepEqual(status, sts.Status) {
		return writes, nil
	}
	sts.Status = status
	if err := s.client.Status().Update(ctx, sts, client.FieldOwner(kubeControllerManager)); err != nil {
		return writes, fmt.Errorf("writing the status: %w", err)
	}
	return writes + 1, nil
}

// readyCount returns the ready count of the workload of kind, namespace, and
// name: replicas, or the readyReplicas of its step when that is lower.
func (s *simulator) readyCount(kind, namespace, name string, replicas int32) int32 {
	step, ok := s.steps[objectKey{kind, namespace, name}]
	if !ok || step.ReadyReplicas == nil {
		return replicas
	}
	return min(*step.ReadyReplicas, replicas)
}

// statefulSetClaim returns the claim of template for the Pod of sts with
// ordinal, as the StatefulSet controller makes it.
func statefulSetClaim(
	sts *appsv1.StatefulSet,
	template corev1.PersistentVolumeClaim,
	ordinal int32,
) *corev1.PersistentVolumeClaim {
	labels := maps.Clone(template.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	if sts.Spec.Selector != nil {
		maps.Copy(labels, sts.Spec.Selector.MatchLabels)
	}

	name := template.Name + "-" + sts.Name + "-" + strconv.Itoa(int(ordinal))
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   sts.Namespace,
			Name:        name,
			Labels:      labels,
			Annotations: template.Annotations,
		},
		Spec: *template.Spec.DeepCopy(),
	}
	claim.Spec.VolumeName = volumeName(sts.Namespace, name)

	policy := sts.Spec.PersistentVolumeClaimRetentionPolicy
	if policy != nil && policy.WhenDeleted == appsv1.DeletePersistentVolumeClaimRetentionPolicyType {
		claim.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(sts, appsv1.SchemeGroupVersion.WithKind("StatefulSet")),
		}
	}
	return claim
}

// volumeName returns the name of the volume that a claim binds to. No
// PersistentVolume of that name exists.
func volumeName(namespace, name string) string {
	return "pvc-" + scenarioUID("PersistentVolume", "", namespace+"/"+name)
}

// statefulSetPod brings the Pod of sts with ordinal to revision and to
// ready. It deletes a Pod of an older revision, and the next pass makes it
// again.
func (s *simulator) statefulSetPod(
	ctx context.Context,
	sts *appsv1.StatefulSet,
	revision string,
	ordinal int32,
	ready bool,
) (int, error) {
	name := sts.Name + "-" + strconv.Itoa(int(ordinal))
	var pod corev1.Pod
	err := s.client.Get(ctx, client.ObjectKey{Namespace: sts.Namespace, Name: name}, &pod)
	switch {
	case apierrors.IsNotFound(err):
		pod = *statefulSetPodFor(sts, revision, ordinal)
		if err := s.client.Create(ctx, &pod, client.FieldOwner(kubeControllerManager)); err != nil {
			return 0, fmt.Errorf("creating Pod %s: %w", name, err)
		}
		n, err := s.runPod(ctx, &pod, ready)
		return n + 1, err
	case err != nil:
		return 0, fmt.Errorf("reading Pod %s: %w", name, err)
	case pod.Labels[appsv1.ControllerRevisionHashLabelKey] != revision:
		return 1, s.deletePod(ctx, &pod)
	default:
		return s.runPod(ctx, &pod, ready)
	}
}

// statefulSetPodFor returns the Pod of sts with ordinal, as the StatefulSet
// controller makes it from revision.
func statefulSetPodFor(sts *appsv1.StatefulSet, revision string, ordinal int32) *corev1.Pod {
	name := sts.Name + "-" + strconv.Itoa(int(ordinal))
	pod := podFromTemplate(sts.Spec.Template, sts.Namespace, name)
	pod.Labels[appsv1.ControllerRevisionHashLabelKey] = revision
	pod.Labels[appsv1.StatefulSetPodNameLabel] = name
	pod.Labels[appsv1.PodIndexLabel] = strconv.Itoa(int(ordinal))
	pod.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(sts, appsv1.SchemeGroupVersion.WithKind("StatefulSet")),
	}

	for _, template := range sts.Spec.VolumeClaimTemplates {
		claim := template.Name + "-" + name
		volume := corev1.Volume{
			Name: template.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim},
			},
		}
		index := slices.IndexFunc(pod.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == template.Name })
		if index >= 0 {
			pod.Spec.Volumes[index] = volume
		} else {
			pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
		}
	}
	return pod
}

// deployments brings each Deployment of ns, its ReplicaSet, and its Pods to
// the state of its current template.
func (s *simulator) deployments(ctx context.Context, ns string) (int, error) {
	var list appsv1.DeploymentList
	if err := s.client.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return 0, fmt.Errorf("listing the Deployments of %s: %w", ns, err)
	}

	writes := 0
	for i := range list.Items {
		n, err := s.deployment(ctx, &list.Items[i])
		writes += n
		if err != nil {
			return writes, fmt.Errorf("simulating Deployment %s/%s: %w", ns, list.Items[i].Name, err)
		}
	}
	return writes, nil
}

// deployment brings d, its ReplicaSet, and its Pods to the state of its
// current template, and retires each other ReplicaSet of d.
func (s *simulator) deployment(ctx context.Context, d *appsv1.Deployment) (int, error) {
	replicas := ptr.Deref(d.Spec.Replicas, 1)
	ready := s.readyCount(kindDeployment, d.Namespace, d.Name, replicas)
	hash := templateHash(d.Spec.Template, d)
	rsName := d.Name + "-" + hash

	var sets appsv1.ReplicaSetList
	if err := s.client.List(ctx, &sets, client.InNamespace(d.Namespace)); err != nil {
		return 0, fmt.Errorf("listing ReplicaSets: %w", err)
	}
	var old []*appsv1.ReplicaSet
	revision := 0
	for i := range sets.Items {
		rs := &sets.Items[i]
		if !controlledBy(rs, "Deployment", d.Name) {
			continue
		}
		revision = max(revision, rsRevision(rs))
		if rs.Name != rsName {
			old = append(old, rs)
		}
	}

	writes, err := s.retireReplicaSets(ctx, old, int(ptr.Deref(d.Spec.RevisionHistoryLimit, 10)))
	if err != nil {
		return writes, err
	}

	rs := replicaSetFor(d, hash, replicas)
	rs.Annotations = map[string]string{revisionAnnotation: strconv.Itoa(revision + 1)}
	created, err := s.createIfMissing(ctx, rs)
	if created {
		writes++
	}
	if err != nil {
		return writes, err
	}
	if err := s.client.Get(ctx, client.ObjectKeyFromObject(rs), rs); err != nil {
		return writes, fmt.Errorf("reading ReplicaSet %s: %w", rsName, err)
	}
	if ptr.Deref(rs.Spec.Replicas, 1) != replicas {
		rs.Spec.Replicas = new(replicas)
		if err := s.client.Update(ctx, rs, client.FieldOwner(kubeControllerManager)); err != nil {
			return writes, fmt.Errorf("scaling ReplicaSet %s: %w", rsName, err)
		}
		writes++
	}

	n, err := s.replicaSetPods(ctx, rs, replicas, ready)
	writes += n
	if err != nil {
		return writes, err
	}

	rsStatus := *rs.Status.DeepCopy()
	rsStatus.ObservedGeneration = rs.Generation
	rsStatus.Replicas = replicas
	rsStatus.FullyLabeledReplicas = replicas
	rsStatus.ReadyReplicas = ready
	rsStatus.AvailableReplicas = ready
	if !equality.Semantic.DeepEqual(rsStatus, rs.Status) {
		rs.Status = rsStatus
		if err := s.client.Status().Update(ctx, rs, client.FieldOwner(kubeControllerManager)); err != nil {
			return writes, fmt.Errorf("writing the status of ReplicaSet %s: %w", rsName, err)
		}
		writes++
	}

	status := deploymentStatus(d, rsName, replicas, ready)
	if equality.Semantic.DeepEqual(status, d.Status) {
		return writes, nil
	}
	d.Status = status
	if err := s.client.Status().Update(ctx, d, client.FieldOwner(kubeControllerManager)); err != nil {
		return writes, fmt.Errorf("writing the status: %w", err)
	}
	return writes + 1, nil
}

// revisionAnnotation holds the revision of a ReplicaSet of a Deployment.
const revisionAnnotation = "deployment.kubernetes.io/revision"

// rsRevision returns the revision of rs, or 0 when it has none.
func rsRevision(rs *appsv1.ReplicaSet) int {
	revision, _ := strconv.Atoi(rs.Annotations[revisionAnnotation])
	return revision
}

// retireReplicaSets scales each of old to 0 and deletes its Pods, as the
// Deployment controller does after a rollout, and deletes the oldest of old
// beyond limit.
func (s *simulator) retireReplicaSets(ctx context.Context, old []*appsv1.ReplicaSet, limit int) (int, error) {
	slices.SortFunc(old, func(a, b *appsv1.ReplicaSet) int {
		return cmp.Or(cmp.Compare(rsRevision(b), rsRevision(a)), cmp.Compare(a.Name, b.Name))
	})

	writes := 0
	for i, rs := range old {
		pods, err := s.ownedPods(ctx, rs.Namespace, "ReplicaSet", rs.Name)
		if err != nil {
			return writes, err
		}
		for j := range pods {
			if err := s.deletePod(ctx, &pods[j]); err != nil {
				return writes, err
			}
			writes++
		}

		if i >= limit {
			if err := s.client.Delete(ctx, rs); client.IgnoreNotFound(err) != nil {
				return writes, fmt.Errorf("deleting ReplicaSet %s: %w", rs.Name, err)
			}
			writes++
			continue
		}

		if ptr.Deref(rs.Spec.Replicas, 1) != 0 {
			rs.Spec.Replicas = new(int32(0))
			if err := s.client.Update(ctx, rs, client.FieldOwner(kubeControllerManager)); err != nil {
				return writes, fmt.Errorf("scaling ReplicaSet %s to 0: %w", rs.Name, err)
			}
			writes++
		}
		status := appsv1.ReplicaSetStatus{ObservedGeneration: rs.Generation}
		if !equality.Semantic.DeepEqual(status, rs.Status) {
			rs.Status = status
			if err := s.client.Status().Update(ctx, rs, client.FieldOwner(kubeControllerManager)); err != nil {
				return writes, fmt.Errorf("writing the status of ReplicaSet %s: %w", rs.Name, err)
			}
			writes++
		}
	}
	return writes, nil
}

// replicaSetFor returns the ReplicaSet of d for the template with hash, as
// the Deployment controller makes it.
func replicaSetFor(d *appsv1.Deployment, hash string, replicas int32) *appsv1.ReplicaSet {
	template := *d.Spec.Template.DeepCopy()
	template.Labels = maps.Clone(template.Labels)
	if template.Labels == nil {
		template.Labels = map[string]string{}
	}
	template.Labels[appsv1.DefaultDeploymentUniqueLabelKey] = hash

	selector := d.Spec.Selector.DeepCopy()
	if selector == nil {
		selector = &metav1.LabelSelector{}
	}
	selector.MatchLabels = maps.Clone(selector.MatchLabels)
	if selector.MatchLabels == nil {
		selector.MatchLabels = map[string]string{}
	}
	selector.MatchLabels[appsv1.DefaultDeploymentUniqueLabelKey] = hash

	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.Namespace,
			Name:      d.Name + "-" + hash,
			Labels:    maps.Clone(template.Labels),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(d, appsv1.SchemeGroupVersion.WithKind("Deployment")),
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: new(replicas),
			Selector: selector,
			Template: template,
		},
	}
}

// replicaSetPods makes the Pods of rs up to replicas, the first ready of
// them ready, and deletes each other Pod of rs.
func (s *simulator) replicaSetPods(ctx context.Context, rs *appsv1.ReplicaSet, replicas, ready int32) (int, error) {
	names := make([]string, replicas)
	for i := range replicas {
		names[i] = rs.Name + "-" + podSuffix(rs.Name+"/"+strconv.Itoa(int(i)))
	}

	writes := 0
	pods, err := s.ownedPods(ctx, rs.Namespace, "ReplicaSet", rs.Name)
	if err != nil {
		return writes, err
	}
	for i := range pods {
		if slices.Contains(names, pods[i].Name) {
			continue
		}
		if err := s.deletePod(ctx, &pods[i]); err != nil {
			return writes, err
		}
		writes++
	}

	for i, name := range names {
		var pod corev1.Pod
		err := s.client.Get(ctx, client.ObjectKey{Namespace: rs.Namespace, Name: name}, &pod)
		if apierrors.IsNotFound(err) {
			pod = *podFromTemplate(rs.Spec.Template, rs.Namespace, name)
			pod.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(rs, appsv1.SchemeGroupVersion.WithKind("ReplicaSet")),
			}
			if err := s.client.Create(ctx, &pod, client.FieldOwner(kubeControllerManager)); err != nil {
				return writes, fmt.Errorf("creating Pod %s: %w", name, err)
			}
			writes++
		} else if err != nil {
			return writes, fmt.Errorf("reading Pod %s: %w", name, err)
		}

		n, err := s.runPod(ctx, &pod, int32(i) < ready)
		writes += n
		if err != nil {
			return writes, err
		}
	}
	return writes, nil
}

// deploymentStatus returns the status that the Deployment controller
// reports for d when rsName holds replicas Pods and ready of them are ready.
func deploymentStatus(d *appsv1.Deployment, rsName string, replicas, ready int32) appsv1.DeploymentStatus {
	status := *d.Status.DeepCopy()
	status.ObservedGeneration = d.Generation
	status.Replicas = replicas
	status.UpdatedReplicas = replicas
	status.ReadyReplicas = ready
	status.AvailableReplicas = ready
	status.UnavailableReplicas = replicas - ready

	maxUnavailable := intstr.FromString("25%")
	if rolling := d.Spec.Strategy.RollingUpdate; rolling != nil && rolling.MaxUnavailable != nil {
		maxUnavailable = *rolling.MaxUnavailable
	}
	if d.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType {
		maxUnavailable = intstr.FromInt32(0)
	}
	unavailable, err := intstr.GetScaledValueFromIntOrPercent(&maxUnavailable, int(replicas), false)
	if err != nil {
		unavailable = 0
	}

	available := appsv1.DeploymentCondition{
		Type:    appsv1.DeploymentAvailable,
		Status:  corev1.ConditionTrue,
		Reason:  "MinimumReplicasAvailable",
		Message: "Deployment has minimum availability.",
	}
	if ready < replicas-int32(unavailable) {
		available.Status = corev1.ConditionFalse
		available.Reason = "MinimumReplicasUnavailable"
		available.Message = "Deployment does not have minimum availability."
	}
	progressing := appsv1.DeploymentCondition{
		Type:    appsv1.DeploymentProgressing,
		Status:  corev1.ConditionTrue,
		Reason:  "NewReplicaSetAvailable",
		Message: fmt.Sprintf("ReplicaSet %q has successfully progressed.", rsName),
	}
	if ready < replicas {
		progressing.Reason = "ReplicaSetUpdated"
		progressing.Message = fmt.Sprintf("ReplicaSet %q is progressing.", rsName)
	}

	status.Conditions = nil
	for _, c := range []appsv1.DeploymentCondition{available, progressing} {
		c.LastUpdateTime = scenarioTime
		c.LastTransitionTime = scenarioTime
		status.Conditions = append(status.Conditions, c)
	}
	return status
}

// jobs runs each Job of ns that has not finished: it makes the Pods of the
// Job and finishes the Job with the outcome of its step, or Complete when it
// has no step. A suspended Job does not run.
func (s *simulator) jobs(ctx context.Context, ns string) (int, error) {
	var list batchv1.JobList
	if err := s.client.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return 0, fmt.Errorf("listing the Jobs of %s: %w", ns, err)
	}

	writes := 0
	for i := range list.Items {
		job := &list.Items[i]
		outcome := jobComplete
		if step, ok := s.steps[objectKey{kindJob, job.Namespace, job.Name}]; ok {
			outcome = step.JobOutcome
		}

		if finished := finishedOutcome(job); finished != "" {
			// Kubernetes never changes the outcome of a finished Job, so a
			// later step for it would have no effect.
			if finished != outcome {
				return writes, fmt.Errorf(
					"workload step gives Job %s/%s the outcome %s, but the Job finished %s",
					ns, job.Name, outcome, finished,
				)
			}
			continue
		}
		// A Job with a parallelism of 0 runs no Pod, as a suspended one.
		if ptr.Deref(job.Spec.Suspend, false) || ptr.Deref(job.Spec.Parallelism, 1) == 0 {
			continue
		}
		n, err := s.runJob(ctx, job, outcome)
		writes += n
		if err != nil {
			return writes, fmt.Errorf("simulating Job %s/%s: %w", ns, job.Name, err)
		}
	}
	return writes, nil
}

// finishedOutcome returns jobComplete or jobFailed when job has that true
// condition, and "" when job has not finished.
func finishedOutcome(job *batchv1.Job) string {
	for _, c := range job.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		switch c.Type {
		case batchv1.JobComplete:
			return jobComplete
		case batchv1.JobFailed:
			return jobFailed
		}
	}
	return ""
}

// runJob makes the Pods of job, finishes them with outcome, and writes the
// status of job.
func (s *simulator) runJob(ctx context.Context, job *batchv1.Job, outcome string) (int, error) {
	// The defaults of the API server for spec.completions and
	// spec.backoffLimit.
	pods := ptr.Deref(job.Spec.Completions, 1)
	if outcome == jobFailed {
		pods = ptr.Deref(job.Spec.BackoffLimit, 6) + 1
	}

	writes := 0
	for i := range pods {
		name := job.Name + "-" + podSuffix(job.Name+"/"+strconv.Itoa(int(i)))
		var pod corev1.Pod
		err := s.client.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: name}, &pod)
		if apierrors.IsNotFound(err) {
			pod = *podFromTemplate(job.Spec.Template, job.Namespace, name)
			pod.Labels[batchv1.JobNameLabel] = job.Name
			pod.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job")),
			}
			if err := s.client.Create(ctx, &pod, client.FieldOwner(kubeControllerManager)); err != nil {
				return writes, fmt.Errorf("creating Pod %s: %w", name, err)
			}
			writes++
		} else if err != nil {
			return writes, fmt.Errorf("reading Pod %s: %w", name, err)
		}

		n, err := s.finishPod(ctx, &pod, outcome == jobComplete)
		writes += n
		if err != nil {
			return writes, err
		}
	}

	job.Status.StartTime = &scenarioTime
	condition := func(t batchv1.JobConditionType, reason, message string) batchv1.JobCondition {
		return batchv1.JobCondition{
			Type:               t,
			Status:             corev1.ConditionTrue,
			Reason:             reason,
			Message:            message,
			LastProbeTime:      scenarioTime,
			LastTransitionTime: scenarioTime,
		}
	}
	if outcome == jobComplete {
		const message = "Reached expected number of succeeded pods"
		job.Status.CompletionTime = &scenarioTime
		job.Status.Succeeded = pods
		job.Status.Conditions = append(
			job.Status.Conditions,
			condition(batchv1.JobSuccessCriteriaMet, batchv1.JobReasonCompletionsReached, message),
			condition(batchv1.JobComplete, batchv1.JobReasonCompletionsReached, message),
		)
	} else {
		const message = "Job has reached the specified backoff limit"
		job.Status.Failed = pods
		job.Status.Conditions = append(
			job.Status.Conditions,
			condition(batchv1.JobFailureTarget, batchv1.JobReasonBackoffLimitExceeded, message),
			condition(batchv1.JobFailed, batchv1.JobReasonBackoffLimitExceeded, message),
		)
	}
	if err := s.client.Status().Update(ctx, job, client.FieldOwner(kubeControllerManager)); err != nil {
		return writes, fmt.Errorf("writing the status: %w", err)
	}
	return writes + 1, nil
}

// claims binds each PersistentVolumeClaim of ns that is not bound.
func (s *simulator) claims(ctx context.Context, ns string) (int, error) {
	var list corev1.PersistentVolumeClaimList
	if err := s.client.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return 0, fmt.Errorf("listing the PersistentVolumeClaims of %s: %w", ns, err)
	}

	writes := 0
	for i := range list.Items {
		claim := &list.Items[i]
		if claim.DeletionTimestamp != nil {
			n, err := s.releaseClaim(ctx, claim)
			writes += n
			if err != nil {
				return writes, err
			}
			continue
		}
		if claim.Status.Phase == corev1.ClaimBound {
			continue
		}

		if claim.Spec.VolumeName == "" {
			claim.Spec.VolumeName = volumeName(claim.Namespace, claim.Name)
			if err := s.client.Update(ctx, claim, client.FieldOwner(kubeControllerManager)); err != nil {
				return writes, fmt.Errorf("binding PersistentVolumeClaim %s/%s: %w", ns, claim.Name, err)
			}
			writes++
		}

		claim.Status.Phase = corev1.ClaimBound
		claim.Status.AccessModes = claim.Spec.AccessModes
		claim.Status.Capacity = corev1.ResourceList{}
		if request, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			claim.Status.Capacity[corev1.ResourceStorage] = request
		}
		if err := s.client.Status().Update(ctx, claim, client.FieldOwner(kubeControllerManager)); err != nil {
			return writes, fmt.Errorf("writing the status of PersistentVolumeClaim %s/%s: %w", ns, claim.Name, err)
		}
		writes++
	}
	return writes, nil
}

// releaseClaim removes the protection finalizer from claim, which is being
// deleted, once no Pod uses it, as kube-controller-manager does.
func (s *simulator) releaseClaim(ctx context.Context, claim *corev1.PersistentVolumeClaim) (int, error) {
	if !slices.Contains(claim.Finalizers, pvcProtectionFinalizer) {
		return 0, nil
	}

	var pods corev1.PodList
	if err := s.client.List(ctx, &pods, client.InNamespace(claim.Namespace)); err != nil {
		return 0, fmt.Errorf("listing the Pods of %s: %w", claim.Namespace, err)
	}
	for _, pod := range pods.Items {
		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == claim.Name {
				return 0, nil
			}
		}
	}

	claim.Finalizers = slices.DeleteFunc(claim.Finalizers, func(f string) bool { return f == pvcProtectionFinalizer })
	if err := s.client.Update(ctx, claim, client.FieldOwner(kubeControllerManager)); err != nil {
		return 0, fmt.Errorf("releasing PersistentVolumeClaim %s/%s: %w", claim.Namespace, claim.Name, err)
	}
	return 1, nil
}

// createIfMissing creates obj unless an object of its name exists. It
// reports whether it created obj.
func (s *simulator) createIfMissing(ctx context.Context, obj client.Object) (bool, error) {
	err := s.client.Create(ctx, obj, client.FieldOwner(kubeControllerManager))
	if apierrors.IsAlreadyExists(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("creating %s: %w", obj.GetName(), err)
	}
	return true, nil
}

// ownedPods returns the Pods of namespace whose controller is the object of
// kind and name.
func (s *simulator) ownedPods(ctx context.Context, namespace, kind, name string) ([]corev1.Pod, error) {
	var list corev1.PodList
	if err := s.client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("listing the Pods of %s: %w", namespace, err)
	}

	var pods []corev1.Pod
	for i := range list.Items {
		if controlledBy(&list.Items[i], kind, name) {
			pods = append(pods, list.Items[i])
		}
	}
	return pods, nil
}

// controlledBy reports whether the controller of obj is the object of kind
// and name.
func controlledBy(obj metav1.Object, kind, name string) bool {
	owner := metav1.GetControllerOf(obj)
	return owner != nil && owner.Kind == kind && owner.Name == name
}

// deletePod deletes pod at once. envtest runs no kubelet, so a Pod deleted
// with a grace period would stay until the period ends.
func (s *simulator) deletePod(ctx context.Context, pod *corev1.Pod) error {
	err := s.client.Delete(ctx, pod, client.GracePeriodSeconds(0))
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("deleting Pod %s: %w", pod.Name, err)
	}
	return nil
}

// podFromTemplate returns a Pod of namespace and name from template, on
// scenarioNode. Its labels are a copy, so a caller may add to them.
func podFromTemplate(template corev1.PodTemplateSpec, namespace, name string) *corev1.Pod {
	labels := maps.Clone(template.Labels)
	if labels == nil {
		labels = map[string]string{}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      labels,
			Annotations: maps.Clone(template.Annotations),
		},
		Spec: *template.Spec.DeepCopy(),
	}
	pod.Spec.NodeName = scenarioNode
	return pod
}

// runPod writes the status that the kubelet reports for pod while its
// containers run, ready or not.
func (s *simulator) runPod(ctx context.Context, pod *corev1.Pod, ready bool) (int, error) {
	readiness := corev1.ConditionFalse
	if ready {
		readiness = corev1.ConditionTrue
	}

	status := *pod.Status.DeepCopy()
	status.Phase = corev1.PodRunning
	status.StartTime = &scenarioTime
	status.Conditions = podConditions(corev1.ConditionTrue, readiness)
	status.ContainerStatuses = nil
	for _, c := range pod.Spec.Containers {
		status.ContainerStatuses = append(status.ContainerStatuses, corev1.ContainerStatus{
			Name:    c.Name,
			Image:   c.Image,
			Ready:   ready,
			Started: new(true),
			State:   corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: scenarioTime}},
		})
	}
	return s.writePodStatus(ctx, pod, status)
}

// finishPod writes the status that the kubelet reports for pod after its
// containers ended, with exit code 0 when succeeded and 1 when not.
func (s *simulator) finishPod(ctx context.Context, pod *corev1.Pod, succeeded bool) (int, error) {
	phase, exitCode, reason := corev1.PodSucceeded, int32(0), "Completed"
	if !succeeded {
		phase, exitCode, reason = corev1.PodFailed, 1, "Error"
	}

	status := *pod.Status.DeepCopy()
	status.Phase = phase
	status.StartTime = &scenarioTime
	status.Conditions = podConditions(corev1.ConditionTrue, corev1.ConditionFalse)
	status.ContainerStatuses = nil
	for _, c := range pod.Spec.Containers {
		status.ContainerStatuses = append(status.ContainerStatuses, corev1.ContainerStatus{
			Name:    c.Name,
			Image:   c.Image,
			Started: new(false),
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode:   exitCode,
				Reason:     reason,
				StartedAt:  scenarioTime,
				FinishedAt: scenarioTime,
			}},
		})
	}
	return s.writePodStatus(ctx, pod, status)
}

// podConditions returns the conditions of a Pod that is scheduled and
// initialized, with the status of ContainersReady and Ready as ready.
func podConditions(initialized, ready corev1.ConditionStatus) []corev1.PodCondition {
	conditions := make([]corev1.PodCondition, 0, 4)
	for _, c := range []struct {
		t      corev1.PodConditionType
		status corev1.ConditionStatus
	}{
		{corev1.PodScheduled, corev1.ConditionTrue},
		{corev1.PodInitialized, initialized},
		{corev1.ContainersReady, ready},
		{corev1.PodReady, ready},
	} {
		conditions = append(conditions, corev1.PodCondition{
			Type:               c.t,
			Status:             c.status,
			LastTransitionTime: scenarioTime,
		})
	}
	return conditions
}

// writePodStatus writes status on pod as the kubelet when it differs from
// the status of pod.
func (s *simulator) writePodStatus(ctx context.Context, pod *corev1.Pod, status corev1.PodStatus) (int, error) {
	if equality.Semantic.DeepEqual(status, pod.Status) {
		return 0, nil
	}
	pod.Status = status
	if err := s.client.Status().Update(ctx, pod, client.FieldOwner(kubelet)); err != nil {
		return 0, fmt.Errorf("writing the status of Pod %s: %w", pod.Name, err)
	}
	return 1, nil
}

// templateHash returns the first 10 hex characters of the SHA-256 of the
// JSON of t, the template of workload. A template can hold the UID of an
// owner of workload, such as a label that names the owner, and the UID of
// an object differs on each run. The hash takes the scenario UID of each
// owner in its place, so it is the same on each run.
func templateHash(t corev1.PodTemplateSpec, workload metav1.Object) string {
	data, _ := json.Marshal(t)
	for _, owner := range workload.GetOwnerReferences() {
		derived := scenarioUID(owner.Kind, workload.GetNamespace(), owner.Name)
		data = bytes.ReplaceAll(data, []byte(owner.UID), []byte(derived))
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:10]
}

// podSuffix returns 5 characters from the alphabet of the generated names
// of Kubernetes, the same for each seed.
func podSuffix(seed string) string {
	const alphabet = "bcdfghjklmnpqrstvwxz2456789"
	sum := sha256.Sum256([]byte(seed))
	var b strings.Builder
	for _, c := range sum[:5] {
		b.WriteByte(alphabet[int(c)%len(alphabet)])
	}
	return b.String()
}

// missingSteps returns an error that names each workload step whose object
// no call has found since the step was added.
func (s *simulator) missingSteps(ctx context.Context) error {
	keys := slices.SortedFunc(maps.Keys(s.steps), func(a, b objectKey) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name))
	})

	var errs []error
	for _, key := range keys {
		var obj client.Object
		switch key.Kind {
		case kindStatefulSet:
			obj = &appsv1.StatefulSet{}
		case kindDeployment:
			obj = &appsv1.Deployment{}
		default:
			obj = &batchv1.Job{}
		}

		err := s.client.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Name}, obj)
		switch {
		case err == nil:
			s.matched[key] = true
		case apierrors.IsNotFound(err):
			if !s.matched[key] {
				errs = append(errs, fmt.Errorf(
					"workload step names %s %s/%s, which does not exist", key.Kind, key.Namespace, key.Name,
				))
			}
		default:
			errs = append(errs, fmt.Errorf("reading %s %s/%s: %w", key.Kind, key.Namespace, key.Name, err))
		}
	}
	return errors.Join(errs...)
}

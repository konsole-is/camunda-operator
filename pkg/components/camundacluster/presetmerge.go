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
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The problem messages of ValidateMerged. Tests and docs quote them, so each
// one exists exactly once.
const (
	// msgMissingFields names the fields that no layer (cluster, preset, or
	// release) provides.
	msgMissingFields = "missing required fields: %s"
	// msgVersionBelowFloor rejects a Camunda version below 8.9.0.
	msgVersionBelowFloor = "version %s is below the supported floor %s"
	// msgVersionNotThreeSegments rejects a version that is not x.y.z.
	msgVersionNotThreeSegments = "version %s is not of the form x.y.z"
	// msgReplicationFactorExceedsReplicas rejects more copies of a partition
	// than there are brokers.
	msgReplicationFactorExceedsReplicas = "zeebe.replicationFactor %d exceeds zeebe.replicas %d"
	// msgPartitionsBelowMinimum rejects a partition count of zero or less.
	msgPartitionsBelowMinimum = "zeebe.partitions %d is below the minimum 1"
	// msgConnectorsVersionRequired rejects enabled connectors without a
	// bundle version.
	msgConnectorsVersionRequired = "connectors.version is required when connectors are enabled"
	// msgContinuousWithoutSchedule quotes the pairing rule of the Camunda
	// backup guide: continuous mode holds the log until a backup runs.
	msgContinuousWithoutSchedule = "backup.primaryStorage.continuous is true but the schedule is none: " +
		"continuous mode holds every log segment until a backup runs, so it always needs a schedule"
)

// versionFloor is the lowest Camunda version the operator supports.
const versionFloor = "8.9.0"

// MergeSpec resolves a CamundaCluster spec against its baselines: the
// preset, then the release over it, then spec over both, under the rules of
// the CamundaClusterPreset doc. Scalars and pointers override individually:
// version, the auth fields, per-component mode, replicas, partitions,
// replicationFactor, storageClassName, storageSize,
// persistentVolumeClaimRetentionPolicy, connectors.enabled, and
// connectors.version. Resources merge per request and limit entry. ExtraEnv
// merges by variable name, lower layer first, and an entry of a higher layer
// with the same name replaces it. ExtraEnvFrom concatenates, lower layer
// first. PodLabels and podAnnotations merge by key with the higher layer
// winning. Scheduling never merges: a block set on the cluster replaces the
// block of the preset at that level (top-level, per component, or under
// backup.dump) entirely. Auth.admin and auth.basic never merge either, and
// they exist at the top level only: a block set on the cluster replaces the
// whole block of the preset. The backup block merges per field: the
// primary-storage schedule, checkpoint interval, and retention each override
// on their own, and backup.dump follows the component rules above, with its
// scratch volume replaced as a whole block. A release carries only versions,
// images, and environment, so it takes part in the version and the
// environment rules alone. The instance-bound fields (platformConfigRef,
// presetRef, releaseRef, externalUrl, serviceAccount, storageRef,
// backupStorageRef, documentStorageRef, monitoring, suspend, pause) always
// come from spec. A nil preset or release is an empty layer. The result
// shares no memory with spec, preset, or release, so callers can mutate it
// freely.
func MergeSpec(
	spec v1.CamundaClusterSpec,
	preset *v1.CamundaClusterPresetSpec,
	release *v1.CamundaReleaseSpec,
) v1.CamundaClusterSpec {
	if preset == nil && release == nil {
		return *spec.DeepCopy()
	}

	var merged v1.CamundaClusterSpec
	if preset != nil {
		merged = *preset.Cluster.DeepCopy()
	}
	if release != nil {
		merged = mergeLayer(merged, releaseLayer(release.DeepCopy()))
	}
	merged = mergeLayer(merged, spec)

	merged.PlatformConfigRef = spec.PlatformConfigRef
	merged.PresetRef = spec.PresetRef
	merged.ReleaseRef = spec.ReleaseRef
	merged.ExternalURL = spec.ExternalURL
	merged.ServiceAccount = spec.ServiceAccount
	merged.StorageRef = spec.StorageRef
	merged.BackupStorageRef = spec.BackupStorageRef
	merged.DocumentStorageRef = spec.DocumentStorageRef
	merged.Monitoring = spec.Monitoring
	merged.Suspend = spec.Suspend
	merged.Pause = spec.Pause

	return *merged.DeepCopy()
}

// releaseLayer expresses a release in the shape of a cluster spec, so the
// same merge rules layer it. A component block appears only when the
// release sets something under it, so a nil block of the preset stays nil.
func releaseLayer(release *v1.CamundaReleaseSpec) v1.CamundaClusterSpec {
	layer := v1.CamundaClusterSpec{
		Version:      release.Version,
		ExtraEnv:     release.ExtraEnv,
		ExtraEnvFrom: release.ExtraEnvFrom,
	}

	if env := releaseWorkload(release.Zeebe); env != nil {
		layer.Zeebe = &v1.ZeebeSpec{WorkloadSpec: *env}
	}
	if env := releaseWorkload(release.Gateway); env != nil {
		layer.Gateway = &v1.GatewaySpec{WorkloadSpec: *env}
	}
	if env := releaseWorkload(release.Operate); env != nil {
		layer.Operate = &v1.WebAppSpec{WorkloadSpec: *env}
	}
	if env := releaseWorkload(release.Tasklist); env != nil {
		layer.Tasklist = &v1.WebAppSpec{WorkloadSpec: *env}
	}
	if env := releaseWorkload(release.Admin); env != nil {
		layer.Admin = &v1.WebAppSpec{WorkloadSpec: *env}
	}
	if c := release.Connectors; c != nil {
		env := releaseWorkload(&c.ReleaseEnvSpec)
		if c.Version != "" || env != nil {
			layer.Connectors = &v1.ConnectorsSpec{Version: c.Version}
			if env != nil {
				layer.Connectors.WorkloadSpec = *env
			}
		}
	}

	return layer
}

// releaseWorkload treats an empty block as unset, so `zeebe: {}` on a
// release introduces no component block into the merged spec.
func releaseWorkload(env *v1.ReleaseEnvSpec) *v1.WorkloadSpec {
	if env == nil || (len(env.ExtraEnv) == 0 && len(env.ExtraEnvFrom) == 0) {
		return nil
	}

	return &v1.WorkloadSpec{ExtraEnv: env.ExtraEnv, ExtraEnvFrom: env.ExtraEnvFrom}
}

// mergeLayer applies over on top of base under the field rules of MergeSpec,
// the instance-bound fields aside. The result can share memory with either
// argument.
func mergeLayer(base, over v1.CamundaClusterSpec) v1.CamundaClusterSpec {
	merged := base

	if over.Version != "" {
		merged.Version = over.Version
	}

	merged.Auth = mergeAuth(merged.Auth, over.Auth)

	merged.ExtraEnv = mergeEnv(merged.ExtraEnv, over.ExtraEnv)
	merged.ExtraEnvFrom = append(merged.ExtraEnvFrom, over.ExtraEnvFrom...)
	merged.PodLabels = mergeMap(merged.PodLabels, over.PodLabels)
	merged.PodAnnotations = mergeMap(merged.PodAnnotations, over.PodAnnotations)
	if over.Scheduling != nil {
		merged.Scheduling = over.Scheduling
	}

	merged.Backup = mergeBackup(merged.Backup, over.Backup)

	merged.Zeebe = mergeZeebe(merged.Zeebe, over.Zeebe)
	merged.Gateway = mergeGateway(merged.Gateway, over.Gateway)
	merged.Operate = mergeWebApp(merged.Operate, over.Operate)
	merged.Tasklist = mergeWebApp(merged.Tasklist, over.Tasklist)
	merged.Admin = mergeWebApp(merged.Admin, over.Admin)
	merged.Connectors = mergeConnectors(merged.Connectors, over.Connectors)

	return merged
}

func mergeAuth(base, over *v1.ClusterAuthSpec) *v1.ClusterAuthSpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	if over.ClientID != "" {
		base.ClientID = over.ClientID
	}
	if over.Audience != "" {
		base.Audience = over.Audience
	}
	if over.ClientSecretRef != nil {
		base.ClientSecretRef = over.ClientSecretRef
	}
	if over.Admin != nil {
		base.Admin = over.Admin
	}
	if over.Basic != nil {
		base.Basic = over.Basic
	}

	return base
}

// mergeEnv keeps the order of base, replaces an entry by name, and appends
// the entries of over that base does not have, in their order. A later entry
// of over with the same name replaces the earlier one, so the result carries
// each name once.
func mergeEnv(base, over []corev1.EnvVar) []corev1.EnvVar {
	if len(over) == 0 {
		return base
	}

	index := make(map[string]int, len(base))
	for i, env := range base {
		index[env.Name] = i
	}

	merged := slices.Clone(base)
	for _, env := range over {
		if i, ok := index[env.Name]; ok {
			merged[i] = env
			continue
		}
		merged = append(merged, env)
		index[env.Name] = len(merged) - 1
	}

	return merged
}

// mergeMap returns base with the entries of over applied over it. It returns
// base itself when over is empty, and over itself when base is nil, so an
// unset field stays unset.
func mergeMap[M ~map[K]V, K comparable, V any](base, over M) M {
	if len(over) == 0 {
		return base
	}
	if base == nil {
		return over
	}

	maps.Copy(base, over)
	return base
}

// mergeBackup merges the backup policy: the primary-storage fields merge one
// by one so that a cluster can change the schedule and keep the retention of
// the preset, and the dump block follows the workload rules. Scheduling and
// the scratch volume replace as whole blocks, like every other scheduling
// block of the spec.
func mergeBackup(base, over *v1.ClusterBackupSpec) *v1.ClusterBackupSpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	base.PrimaryStorage = mergePrimaryStorageBackup(base.PrimaryStorage, over.PrimaryStorage)
	base.Dump = mergeBackupDump(base.Dump, over.Dump)

	return base
}

func mergePrimaryStorageBackup(base, over *v1.PrimaryStorageBackupSpec) *v1.PrimaryStorageBackupSpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	if over.Continuous != nil {
		base.Continuous = over.Continuous
	}
	if over.Schedule != "" {
		base.Schedule = over.Schedule
	}
	if over.CheckpointInterval != "" {
		base.CheckpointInterval = over.CheckpointInterval
	}
	base.Retention = mergePrimaryStorageRetention(base.Retention, over.Retention)

	return base
}

func mergePrimaryStorageRetention(base, over *v1.PrimaryStorageRetentionSpec) *v1.PrimaryStorageRetentionSpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	if over.Window != "" {
		base.Window = over.Window
	}
	if over.CleanupSchedule != "" {
		base.CleanupSchedule = over.CleanupSchedule
	}

	return base
}

func mergeBackupDump(base, over *v1.BackupDumpSpec) *v1.BackupDumpSpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	base.Resources = mergeResources(base.Resources, over.Resources)
	base.ExtraEnv = mergeEnv(base.ExtraEnv, over.ExtraEnv)
	base.ExtraEnvFrom = append(base.ExtraEnvFrom, over.ExtraEnvFrom...)
	base.PodLabels = mergeMap(base.PodLabels, over.PodLabels)
	base.PodAnnotations = mergeMap(base.PodAnnotations, over.PodAnnotations)
	if over.Scheduling != nil {
		base.Scheduling = over.Scheduling
	}
	if over.ScratchVolume != nil {
		base.ScratchVolume = over.ScratchVolume
	}
	if over.ActiveDeadlineSeconds != nil {
		base.ActiveDeadlineSeconds = over.ActiveDeadlineSeconds
	}
	if over.PostgresImage != "" {
		base.PostgresImage = over.PostgresImage
	}

	return base
}

func mergeResources(base, over *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	base.Requests = mergeMap(base.Requests, over.Requests)
	base.Limits = mergeMap(base.Limits, over.Limits)
	if over.Claims != nil {
		base.Claims = over.Claims
	}

	return base
}

func mergeZeebe(base, over *v1.ZeebeSpec) *v1.ZeebeSpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	base.WorkloadSpec = mergeWorkload(base.WorkloadSpec, over.WorkloadSpec)
	if over.Partitions != nil {
		base.Partitions = over.Partitions
	}
	if over.ReplicationFactor != nil {
		base.ReplicationFactor = over.ReplicationFactor
	}
	if over.StorageClassName != nil {
		base.StorageClassName = over.StorageClassName
	}
	if over.StorageSize != nil {
		base.StorageSize = over.StorageSize
	}
	if over.PersistentVolumeClaimRetentionPolicy != nil {
		base.PersistentVolumeClaimRetentionPolicy = over.PersistentVolumeClaimRetentionPolicy
	}

	return base
}

// mergeWorkload applies the shared merge rules of one component block:
// replicas by pointer, resources per entry, extraEnv by name, extraEnvFrom
// concatenated, the maps by key, and scheduling wholesale.
func mergeWorkload(base, over v1.WorkloadSpec) v1.WorkloadSpec {
	if over.Replicas != nil {
		base.Replicas = over.Replicas
	}
	base.Resources = mergeResources(base.Resources, over.Resources)
	base.ExtraEnv = mergeEnv(base.ExtraEnv, over.ExtraEnv)
	base.ExtraEnvFrom = append(base.ExtraEnvFrom, over.ExtraEnvFrom...)
	base.PodLabels = mergeMap(base.PodLabels, over.PodLabels)
	base.PodAnnotations = mergeMap(base.PodAnnotations, over.PodAnnotations)
	if over.Scheduling != nil {
		base.Scheduling = over.Scheduling
	}

	return base
}

func mergeGateway(base, over *v1.GatewaySpec) *v1.GatewaySpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	base.WorkloadSpec = mergeWorkload(base.WorkloadSpec, over.WorkloadSpec)
	if over.Mode != "" {
		base.Mode = over.Mode
	}

	return base
}

func mergeWebApp(base, over *v1.WebAppSpec) *v1.WebAppSpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	base.WorkloadSpec = mergeWorkload(base.WorkloadSpec, over.WorkloadSpec)
	if over.Mode != "" {
		base.Mode = over.Mode
	}

	return base
}

func mergeConnectors(base, over *v1.ConnectorsSpec) *v1.ConnectorsSpec {
	if over == nil {
		return base
	}
	if base == nil {
		return over
	}

	base.WorkloadSpec = mergeWorkload(base.WorkloadSpec, over.WorkloadSpec)
	if over.Enabled != nil {
		base.Enabled = over.Enabled
	}
	if over.Version != "" {
		base.Version = over.Version
	}

	return base
}

// ReleaseImages returns the image references that the release pins for the
// merged spec, or nil when none applies. A pin belongs to the version of the
// release, so it applies only while that version is the effective one: a
// cluster that sets its own version, or a restore that forces one, pulls the
// normal repository at that version instead of the pin. The camunda pin
// follows spec.version, the connectors pin follows connectors.version, each
// on its own. The result shares no memory with release.
func ReleaseImages(merged v1.CamundaClusterSpec, release *v1.CamundaReleaseSpec) *v1.ReleaseImagesSpec {
	if release == nil || release.Images == nil {
		return nil
	}

	images := *release.Images

	if merged.Version != release.Version {
		images.Camunda = ""
	}

	effectiveConnectors := ""
	if merged.Connectors != nil {
		effectiveConnectors = merged.Connectors.Version
	}
	if release.Connectors == nil || release.Connectors.Version == "" ||
		effectiveConnectors != release.Connectors.Version {
		images.Connectors = ""
	}

	if images == (v1.ReleaseImagesSpec{}) {
		return nil
	}

	return &images
}

// ValidateMerged checks the merged spec for the rules that admission cannot
// enforce, because a preset or a release can provide the values. The version must
// be present, three segments, and 8.9.0 or later. The effective
// replicationFactor must not exceed the effective replicas, the effective
// partitions must be at least 1, and connectors.version must be present when
// connectors are enabled. The error joins every problem with "; ".
func ValidateMerged(spec v1.CamundaClusterSpec) error {
	var problems []string
	effective := NewEffective(spec)

	if spec.Version == "" {
		problems = append(problems, fmt.Sprintf(msgMissingFields, "version"))
	} else if err := checkVersionFloor(spec.Version); err != nil {
		problems = append(problems, err.Error())
	}

	if effective.ReplicationFactor() > effective.ZeebeReplicas() {
		problems = append(problems, fmt.Sprintf(
			msgReplicationFactorExceedsReplicas, effective.ReplicationFactor(), effective.ZeebeReplicas(),
		))
	}

	if effective.Partitions() < 1 {
		problems = append(problems, fmt.Sprintf(msgPartitionsBelowMinimum, effective.Partitions()))
	}

	if effective.ConnectorsEnabled() && spec.Connectors.Version == "" {
		problems = append(problems, msgConnectorsVersionRequired)
	}

	if b := spec.Backup; b != nil && b.PrimaryStorage != nil &&
		b.PrimaryStorage.Continuous != nil && *b.PrimaryStorage.Continuous &&
		b.PrimaryStorage.Schedule == ScheduleNone {
		problems = append(problems, msgContinuousWithoutSchedule)
	}

	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}

	return nil
}

// checkVersionFloor rejects a Camunda version below versionFloor. A later
// minor or major passes.
func checkVersionFloor(version string) error {
	segments, err := parseVersion(version)
	if err != nil {
		return err
	}
	floor, _ := parseVersion(versionFloor)

	for i := range 3 {
		switch {
		case segments[i] < floor[i]:
			return fmt.Errorf(msgVersionBelowFloor, version, versionFloor)
		case segments[i] > floor[i]:
			return nil
		}
	}

	return nil
}

func parseVersion(version string) ([3]int, error) {
	var parsed [3]int

	segments := strings.Split(version, ".")
	if len(segments) != 3 {
		return parsed, fmt.Errorf(msgVersionNotThreeSegments, version)
	}

	for i, segment := range segments {
		n, err := strconv.Atoi(segment)
		if err != nil {
			return parsed, fmt.Errorf(msgVersionNotThreeSegments, version)
		}
		parsed[i] = n
	}

	return parsed, nil
}

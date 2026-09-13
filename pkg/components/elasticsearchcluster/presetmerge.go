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

package elasticsearchcluster

import (
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// MergeSpec resolves an ElasticsearchCluster spec against its baselines: the
// preset, then the release over it, then spec over both. A field set by a
// higher layer overrides the value of the lower one for that field wholesale.
// A field left unset inherits. An empty list or map (extraEnv, extraEnvFrom,
// podLabels, podAnnotations) counts as unset: the API drops an empty value on
// the way in, so the merge cannot see it. To drop a list that the preset
// provides, override it with the list you want, or reference a preset without
// it. The scheduling block is replaced entirely, never merged field by field,
// and so is the monitoring block, and so is the secureSettings list. A preset
// can carry snapshotStorageRef: one bucket serves a fleet, because every
// cluster writes under its own base path. A release carries the version alone,
// so it takes part in the version rule alone. The instance-bound fields
// (presetRef, releaseRef, secondaryStorageConfig, suspend) always come from
// spec, and a preset cannot set them. A nil preset or release is an empty
// layer. The result shares no memory with preset or release, so callers can
// mutate it freely.
func MergeSpec(
	spec v1.ElasticsearchClusterSpec,
	preset *v1.ElasticsearchClusterPresetSpec,
	release *v1.CamundaReleaseSpec,
) v1.ElasticsearchClusterSpec {
	if preset == nil && release == nil {
		return spec
	}

	var merged v1.ElasticsearchClusterSpec
	if preset != nil {
		merged = *preset.Cluster.DeepCopy()
	}
	if release != nil && release.Elasticsearch != nil && release.Elasticsearch.Version != "" {
		merged.Version = release.Elasticsearch.Version
	}

	if spec.Version != "" {
		merged.Version = spec.Version
	}
	if spec.Replicas != nil {
		merged.Replicas = spec.Replicas
	}
	if spec.Resources != nil {
		merged.Resources = spec.Resources
	}
	if spec.StorageSize != nil {
		merged.StorageSize = spec.StorageSize
	}
	if spec.StorageClassName != nil {
		merged.StorageClassName = spec.StorageClassName
	}
	if spec.ServiceAccount != nil {
		merged.ServiceAccount = spec.ServiceAccount
	}
	if spec.SnapshotStorageRef != "" {
		merged.SnapshotStorageRef = spec.SnapshotStorageRef
	}
	if len(spec.SecureSettings) > 0 {
		merged.SecureSettings = spec.SecureSettings
	}
	if len(spec.ExtraEnv) > 0 {
		merged.ExtraEnv = spec.ExtraEnv
	}
	if len(spec.ExtraEnvFrom) > 0 {
		merged.ExtraEnvFrom = spec.ExtraEnvFrom
	}
	if len(spec.PodLabels) > 0 {
		merged.PodLabels = spec.PodLabels
	}
	if len(spec.PodAnnotations) > 0 {
		merged.PodAnnotations = spec.PodAnnotations
	}
	if spec.Scheduling != nil {
		merged.Scheduling = spec.Scheduling
	}
	if spec.Monitoring != nil {
		merged.Monitoring = spec.Monitoring
	}
	if spec.PersistentVolumeClaimRetentionPolicy != nil {
		merged.PersistentVolumeClaimRetentionPolicy = spec.PersistentVolumeClaimRetentionPolicy
	}

	merged.PresetRef = spec.PresetRef
	merged.ReleaseRef = spec.ReleaseRef
	merged.SecondaryStorageConfig = spec.SecondaryStorageConfig
	merged.Suspend = spec.Suspend

	return merged
}

// ValidateMerged checks the merged spec for the cross-resource rules that
// admission cannot enforce, because a preset or a release can provide the
// values. Version, replicas, and storageSize must be present. The version must
// meet the Camunda 8.9 floor of Elasticsearch 8.19 or 9.2, and any later major
// is above the floor. The error names every missing field and, when set, the
// rejected version.
func ValidateMerged(spec v1.ElasticsearchClusterSpec) error {
	var problems []string

	var missing []string
	if spec.Version == "" {
		missing = append(missing, "version")
	}
	if spec.Replicas == nil {
		missing = append(missing, "replicas")
	}
	if spec.StorageSize == nil {
		missing = append(missing, "storageSize")
	}
	if len(missing) > 0 {
		problems = append(problems, "missing required fields: "+strings.Join(missing, ", "))
	}

	if spec.Version != "" {
		if err := checkVersionFloor(spec.Version); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}

	return nil
}

// checkVersionFloor rejects an Elasticsearch version below the Camunda 8.9
// floor. 8.x needs 8.19 or later, 9.x needs 9.2 or later, earlier majors are
// unsupported, and later majors pass.
func checkVersionFloor(version string) error {
	unsupported := fmt.Errorf("version %q is not supported: Camunda 8.9 requires Elasticsearch 8.19+ or 9.2+", version)

	segments := strings.Split(version, ".")
	if len(segments) != 3 {
		return unsupported
	}

	major, majorErr := strconv.Atoi(segments[0])
	minor, minorErr := strconv.Atoi(segments[1])
	if majorErr != nil || minorErr != nil {
		return unsupported
	}

	switch {
	case major < 8:
		return unsupported
	case major == 8 && minor < 19:
		return unsupported
	case major == 9 && minor < 2:
		return unsupported
	}

	return nil
}

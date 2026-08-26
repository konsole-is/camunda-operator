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

package databaseserver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/robfig/cron/v3"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// MinimumPostgresMajor is the oldest PostgreSQL major that Camunda 8.9
// supports as secondary storage. Camunda calls it deprecated and supports it
// until the vendor ends support
// (https://docs.camunda.io/docs/self-managed/concepts/databases/relational-db/rdbms-support-policy/).
const MinimumPostgresMajor = 14

// baseBackupScheduleParser reads a base backup schedule with the field set
// that CloudNativePG gives its own parser: six fields with the seconds first,
// a day of the week that may be left out, and the @ descriptors.
var baseBackupScheduleParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.DowOptional |
		cron.Descriptor,
)

// MergePreset resolves a DatabaseServer spec against its preset baseline. A
// field set inline overrides the value of the preset for that field
// wholesale. A field left unset inherits from the preset. An empty map
// (podLabels, podAnnotations) counts as unset: the API drops an empty value on
// the way in, so the merge cannot see it. To drop a map that the preset
// provides, override it with the one you want, or reference a preset without
// it. The scheduling block is replaced entirely, never merged field by field,
// and so are the monitoring block and the archive block. A preset may carry
// archive: one bucket serves a fleet, because every server writes under a
// prefix of its own. The instance-bound fields (presetRef, databaseServerConfig,
// suspend) always come from spec, and a preset cannot set them. A nil preset
// returns spec unchanged. The result shares no memory with preset, so callers
// can mutate it freely.
func MergePreset(spec v1.DatabaseServerSpec, preset *v1.DatabaseServerPresetSpec) v1.DatabaseServerSpec {
	if preset == nil {
		return spec
	}

	merged := *preset.Server.DeepCopy()

	if spec.PlatformConfigRef != "" {
		merged.PlatformConfigRef = spec.PlatformConfigRef
	}
	if spec.Version != "" {
		merged.Version = spec.Version
	}
	if spec.Instances != nil {
		merged.Instances = spec.Instances
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
	if spec.WALStorageSize != nil {
		merged.WALStorageSize = spec.WALStorageSize
	}
	if spec.ServiceAccount != nil {
		merged.ServiceAccount = spec.ServiceAccount
	}
	if spec.Scheduling != nil {
		merged.Scheduling = spec.Scheduling
	}
	if len(spec.PodLabels) > 0 {
		merged.PodLabels = spec.PodLabels
	}
	if len(spec.PodAnnotations) > 0 {
		merged.PodAnnotations = spec.PodAnnotations
	}
	if spec.Monitoring != nil {
		merged.Monitoring = spec.Monitoring
	}
	if spec.Archive != nil {
		merged.Archive = spec.Archive
	}

	merged.PresetRef = spec.PresetRef
	merged.DatabaseServerConfig = spec.DatabaseServerConfig
	merged.Suspend = spec.Suspend

	return merged
}

// ValidateMerged checks the preset-merged spec for the rules that admission
// cannot enforce. Version, storageSize, and databaseServerConfig must be
// present, the version must meet the Camunda 8.9 floor, and the base backup
// schedule must be one CloudNativePG can read. Admission cannot see version
// and storageSize because a preset may supply them, it cannot range-check a
// version because the pattern only fixes the shape, and it cannot compare the
// two ends of a range in a schedule. The error names every missing field and,
// when set, the rejected version and the rejected schedule.
func ValidateMerged(spec v1.DatabaseServerSpec) error {
	var problems []string

	var missing []string
	if spec.Version == "" {
		missing = append(missing, "version")
	}
	if spec.StorageSize == nil {
		missing = append(missing, "storageSize")
	}
	if spec.DatabaseServerConfig == "" {
		missing = append(missing, "databaseServerConfig")
	}
	if len(missing) > 0 {
		problems = append(problems, "missing required fields after preset merge: "+strings.Join(missing, ", "))
	}

	if spec.Version != "" {
		if err := checkVersionFloor(spec.Version); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if spec.Archive != nil && spec.Archive.BaseBackupSchedule != "" {
		if err := checkBaseBackupSchedule(spec.Archive.BaseBackupSchedule); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}

	return nil
}

// checkBaseBackupSchedule rejects a schedule that CloudNativePG cannot read.
//
// The pattern on the field bounds every field of the schedule, and a pattern
// cannot compare the two ends of a range. A range that reads downward, such as
// FRI-MON or 23-1, therefore passes admission. CloudNativePG refuses it when
// it reads the ScheduledBackup, so base backups stop while the server goes on
// offering point-in-time recovery. Reading it here puts the refusal on Ready,
// before anything is applied.
func checkBaseBackupSchedule(schedule string) error {
	if _, err := baseBackupScheduleParser.Parse(schedule); err != nil {
		return fmt.Errorf(
			"archive.baseBackupSchedule %q is not a schedule CloudNativePG can read: %w",
			schedule, err,
		)
	}

	return nil
}

// checkVersionFloor rejects a PostgreSQL major below the Camunda 8.9 floor.
// Any later major passes: Camunda adds a major to its policy before the
// operator hears about it, and a server the user runs on purpose must not be
// held back by a number in this file.
func checkVersionFloor(version string) error {
	unsupported := fmt.Errorf(
		"version %q is not supported: Camunda 8.9 requires PostgreSQL %d or later",
		version, MinimumPostgresMajor,
	)

	major, err := strconv.Atoi(version)
	if err != nil || major < MinimumPostgresMajor {
		return unsupported
	}

	return nil
}

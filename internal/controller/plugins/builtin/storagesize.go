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
	"k8s.io/apimachinery/pkg/api/resource"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// keepLiveStorageSize returns the storage size to apply for asked. The live
// size is the larger of spec and the largest of volumes, the spec storage
// size and the volumes of the live object. When asked is smaller than the
// live size, it returns the live size and the summary item that says so.
// Otherwise it returns asked and no item. A nil asked stays nil.
func keepLiveStorageSize(
	asked, spec *resource.Quantity,
	volumes []v1.VolumeStatus,
) (*resource.Quantity, *pluginsv1alpha1.SummaryItem) {
	live := spec
	for _, volume := range volumes {
		if live == nil || volume.Capacity.Cmp(*live) > 0 {
			live = &volume.Capacity
		}
	}
	if asked == nil || live == nil || asked.Cmp(*live) >= 0 {
		return asked, nil
	}

	kept := live.DeepCopy()
	return &kept, &pluginsv1alpha1.SummaryItem{
		Label:    "Storage size",
		Value:    "Storage size " + asked.String() + " is smaller than " + kept.String() + ", kept " + kept.String(),
		Severity: pluginsv1alpha1.SeverityWarning,
	}
}

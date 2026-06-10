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

package controllers

import (
	leaderworkerset "sigs.k8s.io/lws/api/leaderworkerset/v1"
	"sigs.k8s.io/lws/pkg/schedulerprovider"
)

// addGangSchedulerProviderPodLabel propagates the per-LWS opt-in label to pod
// templates only when the value is valid. Invalid values are intentionally not
// propagated so cleanup of invalid historical labels cannot affect pod templates.
func addGangSchedulerProviderPodLabel(podLabels, lwsLabels map[string]string) {
	provider, found := lwsLabels[leaderworkerset.GangSchedulerProviderLabelKey]
	if found && schedulerprovider.IsValidPerLWSProvider(provider) {
		podLabels[leaderworkerset.GangSchedulerProviderLabelKey] = provider
	}
}

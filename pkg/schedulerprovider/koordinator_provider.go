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

package schedulerprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	leaderworkerset "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

const (
	Koordinator ProviderType = "koordinator"

	// ACKPodGroupNameLabelKey is the ACK documented label-only gang scheduling
	// key that identifies which logical PodGroup a pod belongs to. It matches
	// the scheduler-plugins/Koordinator lightweight coscheduling label key.
	ACKPodGroupNameLabelKey = "pod-group.scheduling.sigs.k8s.io/name"

	// ACKPodGroupMinAvailableLabelKey is the ACK documented label-only gang
	// scheduling key that declares the minimum number of pods required before
	// the gang is admitted.
	ACKPodGroupMinAvailableLabelKey = "pod-group.scheduling.sigs.k8s.io/min-available"

	koordinatorGangNamePrefix     = "kg-"
	koordinatorGangNameHashLength = 16
)

// KoordinatorProvider implements ACK Koordinator label-only gang scheduling.
type KoordinatorProvider struct{}

var _ SchedulerProvider = (*KoordinatorProvider)(nil)

// NewKoordinatorProvider creates a provider for ACK Koordinator label-only gang
// scheduling. It does not need a Kubernetes client because it creates no CRs.
func NewKoordinatorProvider() *KoordinatorProvider {
	return &KoordinatorProvider{}
}

// IsValidPerLWSProvider returns whether provider is valid for the fork-private
// per-LWS opt-in label. This is intentionally narrower than the global
// SupportedSchedulerProviders set.
func IsValidPerLWSProvider(provider string) bool {
	return provider == string(Koordinator)
}

// CreatePodGroupIfNotExists is a no-op for ACK label-only gang scheduling.
// ACK kube-scheduler derives and manages PodGroup state from pod labels.
func (k *KoordinatorProvider) CreatePodGroupIfNotExists(ctx context.Context, lws *leaderworkerset.LeaderWorkerSet, leaderPod *corev1.Pod) error {
	return nil
}

// InjectPodGroupMetadata adds ACK gang scheduling labels to opt-in pods.
func (k *KoordinatorProvider) InjectPodGroupMetadata(pod *corev1.Pod) error {
	provider, found := pod.Labels[leaderworkerset.GangSchedulerProviderLabelKey]
	if !found || !IsValidPerLWSProvider(provider) {
		return nil
	}

	lwsName, ok := requiredPodLabel(pod, leaderworkerset.SetNameLabelKey)
	if !ok {
		return fmt.Errorf("missing required pod label %q for Koordinator gang scheduling", leaderworkerset.SetNameLabelKey)
	}
	groupIndex, ok := requiredPodLabel(pod, leaderworkerset.GroupIndexLabelKey)
	if !ok {
		return fmt.Errorf("missing required pod label %q for Koordinator gang scheduling", leaderworkerset.GroupIndexLabelKey)
	}
	revision, ok := requiredPodLabel(pod, leaderworkerset.RevisionKey)
	if !ok {
		return fmt.Errorf("missing required pod label %q for Koordinator gang scheduling", leaderworkerset.RevisionKey)
	}
	size, ok := requiredPodAnnotation(pod, leaderworkerset.SizeAnnotationKey)
	if !ok {
		return fmt.Errorf("missing required pod annotation %q for Koordinator gang scheduling", leaderworkerset.SizeAnnotationKey)
	}
	if value, err := strconv.Atoi(size); err != nil || value <= 0 {
		return fmt.Errorf("invalid pod annotation %q value %q for Koordinator gang scheduling: must be a positive integer", leaderworkerset.SizeAnnotationKey, size)
	}

	pod.Labels[ACKPodGroupNameLabelKey] = GetKoordinatorGangName(pod.Namespace, lwsName, groupIndex, revision)
	pod.Labels[ACKPodGroupMinAvailableLabelKey] = size
	return nil
}

func requiredPodLabel(pod *corev1.Pod, key string) (string, bool) {
	value, found := pod.Labels[key]
	return value, found && value != ""
}

func requiredPodAnnotation(pod *corev1.Pod, key string) (string, bool) {
	value, found := pod.Annotations[key]
	return value, found && value != ""
}

// GetKoordinatorGangName returns a label-safe ACK gang name. ACK label-only gang
// scheduling uses Kubernetes label values, so this intentionally does not reuse
// the Volcano-style "<lws>-<groupIndex>-<revision>" name.
func GetKoordinatorGangName(namespace, lwsName, groupIndex, revision string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%s", namespace, lwsName, groupIndex, revision)))
	return koordinatorGangNamePrefix + hex.EncodeToString(sum[:])[:koordinatorGangNameHashLength]
}

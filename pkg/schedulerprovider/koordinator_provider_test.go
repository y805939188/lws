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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	leaderworkerset "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

func TestGetKoordinatorGangName(t *testing.T) {
	longLWSName := strings.Repeat("a", 63)

	// These golden values intentionally lock the hash algorithm because the
	// result is written to pod labels. Changing it would silently move existing
	// opt-in workloads to a different gang name.
	tests := []struct {
		name       string
		namespace  string
		lwsName    string
		groupIndex string
		revision   string
		want       string
	}{
		{
			name:       "stable name",
			namespace:  "default",
			lwsName:    "test-lws",
			groupIndex: "0",
			revision:   "abc123",
			want:       "kg-c7ad253e09e11917",
		},
		{
			name:       "long lws name and multi digit group index",
			namespace:  "canary-test",
			lwsName:    longLWSName,
			groupIndex: "1234567890",
			revision:   "1234567890",
			want:       "kg-c7d0a166ab99de1f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetKoordinatorGangName(tt.namespace, tt.lwsName, tt.groupIndex, tt.revision)
			assert.Equal(t, tt.want, got)
			assert.LessOrEqual(t, len(got), 63)
			assert.Empty(t, validation.IsValidLabelValue(got))
		})
	}
}

func TestGetKoordinatorGangNameUsesAllInputs(t *testing.T) {
	base := GetKoordinatorGangName("default", "test-lws", "0", "abc123")

	assert.NotEqual(t, base, GetKoordinatorGangName("other", "test-lws", "0", "abc123"))
	assert.NotEqual(t, base, GetKoordinatorGangName("default", "other-lws", "0", "abc123"))
	assert.NotEqual(t, base, GetKoordinatorGangName("default", "test-lws", "1", "abc123"))
	assert.NotEqual(t, base, GetKoordinatorGangName("default", "test-lws", "0", "def456"))
	assert.Equal(t, base, GetKoordinatorGangName("default", "test-lws", "0", "abc123"))
}

func TestKoordinatorProvider_InjectPodGroupMetadata(t *testing.T) {
	tests := []struct {
		name       string
		pod        *corev1.Pod
		wantInject bool
	}{
		{
			name: "no opt-in",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Labels: map[string]string{
						leaderworkerset.SetNameLabelKey:    "test-lws",
						leaderworkerset.GroupIndexLabelKey: "0",
						leaderworkerset.RevisionKey:        "abc123",
					},
					Annotations: map[string]string{
						leaderworkerset.SizeAnnotationKey: "4",
					},
				},
			},
		},
		{
			name: "unsupported opt-in value",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Labels: map[string]string{
						leaderworkerset.GangSchedulerProviderLabelKey: "volcano",
						leaderworkerset.SetNameLabelKey:               "test-lws",
						leaderworkerset.GroupIndexLabelKey:            "0",
						leaderworkerset.RevisionKey:                   "abc123",
					},
					Annotations: map[string]string{
						leaderworkerset.SizeAnnotationKey: "4",
					},
				},
			},
		},
		{
			name:       "koordinator opt-in",
			pod:        koordinatorTestPod("test-lws-0", "0"),
			wantInject: true,
		},
	}

	provider := NewKoordinatorProvider()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.InjectPodGroupMetadata(tt.pod)
			require.NoError(t, err)

			gangName, foundName := tt.pod.Labels[ACKPodGroupNameLabelKey]
			minAvailable, foundMinAvailable := tt.pod.Labels[ACKPodGroupMinAvailableLabelKey]
			if !tt.wantInject {
				assert.False(t, foundName)
				assert.False(t, foundMinAvailable)
				return
			}

			assert.True(t, foundName)
			assert.True(t, foundMinAvailable)
			assert.Equal(t, GetKoordinatorGangName("default", "test-lws", "0", "abc123"), gangName)
			assert.Equal(t, "4", minAvailable)
			assert.Empty(t, validation.IsValidLabelValue(gangName))
		})
	}
}

func TestKoordinatorProvider_InjectPodGroupMetadataLeaderAndWorkerUseSameGangName(t *testing.T) {
	provider := NewKoordinatorProvider()
	leaderPod := koordinatorTestPod("test-lws-0", "0")
	workerPod := koordinatorTestPod("test-lws-0-1", "1")

	require.NoError(t, provider.InjectPodGroupMetadata(leaderPod))
	require.NoError(t, provider.InjectPodGroupMetadata(workerPod))

	assert.Equal(t, leaderPod.Labels[ACKPodGroupNameLabelKey], workerPod.Labels[ACKPodGroupNameLabelKey])
	assert.Equal(t, leaderPod.Labels[ACKPodGroupMinAvailableLabelKey], workerPod.Labels[ACKPodGroupMinAvailableLabelKey])
}

func TestKoordinatorProvider_InjectPodGroupMetadataMissingRequiredInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*corev1.Pod)
	}{
		{
			name: "missing lws name",
			mutate: func(pod *corev1.Pod) {
				delete(pod.Labels, leaderworkerset.SetNameLabelKey)
			},
		},
		{
			name: "empty group index",
			mutate: func(pod *corev1.Pod) {
				pod.Labels[leaderworkerset.GroupIndexLabelKey] = ""
			},
		},
		{
			name: "missing revision",
			mutate: func(pod *corev1.Pod) {
				delete(pod.Labels, leaderworkerset.RevisionKey)
			},
		},
		{
			name: "missing size annotation",
			mutate: func(pod *corev1.Pod) {
				delete(pod.Annotations, leaderworkerset.SizeAnnotationKey)
			},
		},
		{
			name: "non-numeric size annotation",
			mutate: func(pod *corev1.Pod) {
				pod.Annotations[leaderworkerset.SizeAnnotationKey] = "four"
			},
		},
		{
			name: "zero size annotation",
			mutate: func(pod *corev1.Pod) {
				pod.Annotations[leaderworkerset.SizeAnnotationKey] = "0"
			},
		},
	}

	provider := NewKoordinatorProvider()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := koordinatorTestPod("test-lws-0", "0")
			tt.mutate(pod)

			err := provider.InjectPodGroupMetadata(pod)
			require.Error(t, err)
			assert.NotContains(t, pod.Labels, ACKPodGroupNameLabelKey)
			assert.NotContains(t, pod.Labels, ACKPodGroupMinAvailableLabelKey)
		})
	}
}

func TestKoordinatorProvider_CreatePodGroupIfNotExists(t *testing.T) {
	provider := NewKoordinatorProvider()
	assert.NoError(t, provider.CreatePodGroupIfNotExists(context.TODO(), nil, nil))
}

func TestIsValidPerLWSProvider(t *testing.T) {
	assert.True(t, IsValidPerLWSProvider(string(Koordinator)))
	assert.False(t, IsValidPerLWSProvider(""))
	assert.False(t, IsValidPerLWSProvider("Koordinator"))
	assert.False(t, IsValidPerLWSProvider(string(Volcano)))
}

func koordinatorTestPod(name, workerIndex string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				leaderworkerset.GangSchedulerProviderLabelKey: string(Koordinator),
				leaderworkerset.SetNameLabelKey:               "test-lws",
				leaderworkerset.GroupIndexLabelKey:            "0",
				leaderworkerset.WorkerIndexLabelKey:           workerIndex,
				leaderworkerset.RevisionKey:                   "abc123",
			},
			Annotations: map[string]string{
				leaderworkerset.SizeAnnotationKey: "4",
			},
		},
	}
}

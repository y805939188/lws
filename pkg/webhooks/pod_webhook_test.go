/*
Copyright 2023.

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

package webhooks

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	leaderworkerset "sigs.k8s.io/lws/api/leaderworkerset/v1"
	"sigs.k8s.io/lws/pkg/schedulerprovider"
	"sigs.k8s.io/lws/test/wrappers"
)

func TestGenGroupUniqueKey(t *testing.T) {
	tests := []struct {
		name        string
		podName     string
		namespace   string
		expectedKey string
	}{
		{
			name:        "same namespace, pod name",
			podName:     "test-sample",
			namespace:   "default",
			expectedKey: "95e88034e460983f51a9952fe128729fbc0663b5",
		},
		{
			name:        "same namespace, different pod name",
			podName:     "podName",
			namespace:   "default",
			expectedKey: "390b34ab671d29e9997d7d4252b8bbf8da02f5b7",
		},
		{
			name:        "different namespace, same pod name",
			podName:     "test-sample",
			namespace:   "leaderworkerset",
			expectedKey: "39f5d7e9122b9d94d3932e3720b43fd3b56347e8",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := genGroupUniqueKey(tc.namespace, tc.podName)
			if diff := cmp.Diff(tc.expectedKey, key); diff != "" {
				t.Errorf("unexpected key %s", diff)
			}
		})
	}
}

func TestSetExclusiveAffinities(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		groupUniqueKey string
		topologyKey    string
		podAffinityKey string
		expectedPod    *corev1.Pod
	}{
		{
			name: "Pod with only Exclusive Topology Annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"leaderworkerset.sigs.k8s.io/exclusive-topology": "topologyKey"},
				},
			},
			groupUniqueKey: "test-key",
			topologyKey:    "topologyKey",
			podAffinityKey: leaderworkerset.GroupUniqueHashLabelKey,
			expectedPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"leaderworkerset.sigs.k8s.io/exclusive-topology": "topologyKey"},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
								TopologyKey: "topologyKey",
								LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "leaderworkerset.sigs.k8s.io/group-key",
										Operator: "In",
										Values:   []string{"test-key"},
									},
								}},
							}},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
								TopologyKey: "topologyKey",
								LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "leaderworkerset.sigs.k8s.io/group-key",
										Operator: "Exists",
									},
									{
										Key:      "leaderworkerset.sigs.k8s.io/group-key",
										Operator: "NotIn",
										Values:   []string{"test-key"},
									},
								}},
							}},
						},
					},
				},
			},
		},
		{
			name: "Pod with Exclusive Annotation, Affinity, and AntiAffinity",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"leaderworkerset.sigs.k8s.io/exclusive-topology": "topologyKey"},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
					},
				},
			},
			groupUniqueKey: "test-key",
			topologyKey:    "topologyKey",
			podAffinityKey: leaderworkerset.GroupUniqueHashLabelKey,
			expectedPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"leaderworkerset.sigs.k8s.io/exclusive-topology": "topologyKey"},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetExclusiveAffinities(tc.pod, tc.groupUniqueKey, tc.topologyKey, tc.podAffinityKey)
			if diff := cmp.Diff(tc.pod, tc.expectedPod); diff != "" {
				t.Errorf("unexpected set exclusive affinities operation: %s", diff)
			}
		})
	}
}

func TestExclusiveAffinityApplied(t *testing.T) {
	tests := []struct {
		name                              string
		pod                               corev1.Pod
		expectedAppliedExclusivePlacement bool
		topologyKey                       string
	}{
		{
			name: "Has annotiation, Pod Affinity and Pod AntiAffinity",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"leaderworkerset.sigs.k8s.io/exclusive-topology": "topologyKey",
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
					},
				},
			},
			expectedAppliedExclusivePlacement: true,
			topologyKey:                       "topologyKey",
		},
		{
			name: "Has annotiation, Pod Affinity, doesn't have Pod AntiAffinity",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"leaderworkerset.sigs.k8s.io/exclusive-topology": "topologyKey",
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
					},
				},
			},
			expectedAppliedExclusivePlacement: false,
			topologyKey:                       "topologyKey",
		},
		{
			name: "Has annotiation, Pod AntiAffinity, doesn't have Pod Affinity",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"leaderworkerset.sigs.k8s.io/exclusive-topology": "topologyKey",
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
					},
				},
			},
			expectedAppliedExclusivePlacement: false,
			topologyKey:                       "topologyKey",
		},
		{
			name: "Has annotiation, Pod Affinity and Pod AntiAffinity, Topology Key doesn't match",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"leaderworkerset.sigs.k8s.io/exclusive-topology": "topologyKey",
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey1"}},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topologyKey"}},
						},
					},
				},
			},
			expectedAppliedExclusivePlacement: false,
			topologyKey:                       "topologyKey",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			appliedExclusivePlacement := exclusiveAffinityApplied(tc.pod, tc.topologyKey)
			if appliedExclusivePlacement != tc.expectedAppliedExclusivePlacement {
				t.Errorf("Expected value %t, got %t", tc.expectedAppliedExclusivePlacement, appliedExclusivePlacement)
			}
		})
	}
}

func TestGetSubGroupIndex(t *testing.T) {
	tests := []struct {
		name                  string
		podCount              int
		subGroupSize          int
		workerIndex           int
		leaderOnly            bool
		expectedSubGroupIndex string
	}{
		{
			name:                  "Even number of pods",
			podCount:              4,
			subGroupSize:          2,
			workerIndex:           2,
			expectedSubGroupIndex: "1",
		},
		{
			name:                  "Odd number of pods, first subgroup has an extra pod",
			podCount:              5,
			subGroupSize:          2,
			workerIndex:           2,
			expectedSubGroupIndex: "0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subGroupIndex := getSubGroupIndex(tc.podCount, tc.subGroupSize, tc.workerIndex)
			if tc.expectedSubGroupIndex != subGroupIndex {
				t.Errorf("Expected subGroupIndex to be %s, got %s", tc.expectedSubGroupIndex, subGroupIndex)
			}
		})
	}
}

func TestPodWebhookDefaultKoordinatorGangScheduling(t *testing.T) {
	schedulerProvider, err := schedulerprovider.NewSchedulerProvider(schedulerprovider.Koordinator, nil)
	require.NoError(t, err)
	volcanoProvider, err := schedulerprovider.NewSchedulerProvider(schedulerprovider.Volcano, nil)
	require.NoError(t, err)

	tests := []struct {
		name              string
		webhook           *PodWebhook
		pod               *corev1.Pod
		wantInject        bool
		wantVolcanoInject bool
		wantGroupIndex    string
		wantWorkerIndex   string
		wantMinAvailable  string
		wantGangNameInput koordinatorGangNameInput
	}{
		{
			name:    "global koordinator provider with opt-in leader pod",
			webhook: NewPodWebhook(schedulerProvider),
			pod:     koordinatorWebhookLeaderPod(true),
			wantGangNameInput: koordinatorGangNameInput{
				namespace:  "default",
				lwsName:    "test-lws",
				groupIndex: "0",
				revision:   "abc123",
			},
			wantGroupIndex:   "0",
			wantWorkerIndex:  "0",
			wantMinAvailable: "4",
			wantInject:       true,
		},
		{
			name:    "global koordinator provider with opt-in worker pod",
			webhook: NewPodWebhook(schedulerProvider),
			pod:     koordinatorWebhookWorkerPod(true),
			wantGangNameInput: koordinatorGangNameInput{
				namespace:  "default",
				lwsName:    "test-lws",
				groupIndex: "0",
				revision:   "abc123",
			},
			wantGroupIndex:   "0",
			wantWorkerIndex:  "1",
			wantMinAvailable: "4",
			wantInject:       true,
		},
		{
			name:             "global koordinator provider without opt-in",
			webhook:          NewPodWebhook(schedulerProvider),
			pod:              koordinatorWebhookLeaderPod(false),
			wantGroupIndex:   "0",
			wantWorkerIndex:  "0",
			wantMinAvailable: "4",
		},
		{
			name:             "opt-in without global scheduler provider",
			webhook:          NewPodWebhook(nil),
			pod:              koordinatorWebhookLeaderPod(true),
			wantGroupIndex:   "0",
			wantWorkerIndex:  "0",
			wantMinAvailable: "4",
		},
		{
			name:              "global volcano provider ignores koordinator opt-in label",
			webhook:           NewPodWebhook(volcanoProvider),
			pod:               koordinatorWebhookLeaderPod(true),
			wantGroupIndex:    "0",
			wantWorkerIndex:   "0",
			wantMinAvailable:  "4",
			wantVolcanoInject: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.webhook.Default(context.Background(), tt.pod))

			assert.Equal(t, tt.wantGroupIndex, tt.pod.Labels[leaderworkerset.GroupIndexLabelKey])
			assert.Equal(t, tt.wantWorkerIndex, tt.pod.Labels[leaderworkerset.WorkerIndexLabelKey])

			gangName, foundName := tt.pod.Labels[schedulerprovider.ACKPodGroupNameLabelKey]
			minAvailable, foundMinAvailable := tt.pod.Labels[schedulerprovider.ACKPodGroupMinAvailableLabelKey]
			if !tt.wantInject {
				assert.False(t, foundName)
				assert.False(t, foundMinAvailable)
				if tt.wantVolcanoInject {
					assert.Equal(t, schedulerprovider.GetPodGroupName("test-lws", "0", "abc123"), tt.pod.Annotations[volcanov1beta1.KubeGroupNameAnnotationKey])
				} else {
					assert.NotContains(t, tt.pod.Annotations, volcanov1beta1.KubeGroupNameAnnotationKey)
				}
				return
			}

			assert.True(t, foundName)
			assert.True(t, foundMinAvailable)
			assert.NotContains(t, tt.pod.Annotations, volcanov1beta1.KubeGroupNameAnnotationKey)
			assert.Equal(t, schedulerprovider.GetKoordinatorGangName(
				tt.wantGangNameInput.namespace,
				tt.wantGangNameInput.lwsName,
				tt.wantGangNameInput.groupIndex,
				tt.wantGangNameInput.revision,
			), gangName)
			assert.Equal(t, tt.wantMinAvailable, minAvailable)
		})
	}
}

type koordinatorGangNameInput struct {
	namespace  string
	lwsName    string
	groupIndex string
	revision   string
}

func koordinatorWebhookLeaderPod(optIn bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lws-0",
			Namespace: "default",
			Labels: map[string]string{
				leaderworkerset.SetNameLabelKey:     "test-lws",
				leaderworkerset.WorkerIndexLabelKey: "0",
				leaderworkerset.RevisionKey:         "abc123",
			},
			Annotations: map[string]string{
				leaderworkerset.SizeAnnotationKey: "4",
			},
		},
		Spec: wrappers.MakeLeaderPodSpec(),
	}
	if optIn {
		pod.Labels[leaderworkerset.GangSchedulerProviderLabelKey] = string(schedulerprovider.Koordinator)
	}
	return pod
}

func koordinatorWebhookWorkerPod(optIn bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lws-0-1",
			Namespace: "default",
			Labels: map[string]string{
				leaderworkerset.SetNameLabelKey:    "test-lws",
				leaderworkerset.GroupIndexLabelKey: "0",
				leaderworkerset.RevisionKey:        "abc123",
			},
			Annotations: map[string]string{
				leaderworkerset.SizeAnnotationKey:          "4",
				leaderworkerset.LeaderPodNameAnnotationKey: "test-lws-0",
			},
		},
		Spec: wrappers.MakeWorkerPodSpec(),
	}
	if optIn {
		pod.Labels[leaderworkerset.GangSchedulerProviderLabelKey] = string(schedulerprovider.Koordinator)
	}
	return pod
}

/*
Copyright 2024.

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
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	leaderworkerset "sigs.k8s.io/lws/api/leaderworkerset/v1"
	"sigs.k8s.io/lws/pkg/schedulerprovider"
)

func TestGetPercentValue(t *testing.T) {
	tests := []struct {
		name           string
		input          intstr.IntOrString
		wantOutputVal  int
		wantOutputBool bool
	}{
		{
			name: "input type int",
			input: intstr.IntOrString{
				Type:   0,
				IntVal: 1,
			},
			wantOutputVal:  0,
			wantOutputBool: false,
		},
		{
			name: "input type string - invalid format",
			input: intstr.IntOrString{
				Type:   1,
				StrVal: "1",
			},
			wantOutputVal:  0,
			wantOutputBool: false,
		},
		{
			name: "input type string - valid format",
			input: intstr.IntOrString{
				Type:   1,
				StrVal: "1%",
			},
			wantOutputVal:  1,
			wantOutputBool: true,
		},
		{
			name: "input type string - valid format",
			input: intstr.IntOrString{
				Type:   1,
				StrVal: "101%",
			},
			wantOutputVal:  101,
			wantOutputBool: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outputVal, outputBool := getPercentValue(tc.input)
			if diff := cmp.Diff(tc.wantOutputVal, outputVal); diff != "" {
				t.Errorf("unexpected result: (-want, +got) %s", diff)
			}
			if diff := cmp.Diff(tc.wantOutputBool, outputBool); diff != "" {
				t.Errorf("unexpected result: (-want, +got) %s", diff)
			}
		})
	}
}

func TestValidateNonnegativeOrZeroField(t *testing.T) {
	tests := []struct {
		name       string
		input      int64
		wantOutput field.ErrorList
	}{
		{
			name:  "input less than 0",
			input: -1,
			wantOutput: []*field.Error{
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: int64(-1),
					Detail:   "must be greater than or equal to 0",
				},
			},
		},
		{
			name:       "input equal to 0",
			input:      0,
			wantOutput: []*field.Error{},
		},
		{
			name:       "input greater than 0",
			input:      1,
			wantOutput: []*field.Error{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testPath := field.NewPath("test")
			output := validateNonnegativeField(tc.input, testPath)
			if diff := cmp.Diff(tc.wantOutput, output); diff != "" {
				t.Errorf("unexpected result: (-want, +got) %s", diff)
			}
		})
	}
}

func TestIsNotMoreThan100Percent(t *testing.T) {
	tests := []struct {
		name       string
		input      intstr.IntOrString
		wantErr    string
		wantOutput field.ErrorList
	}{
		{
			name: "invalid input",
			input: intstr.IntOrString{
				Type:   0,
				IntVal: 1,
			},
			wantOutput: nil,
		},
		{
			name: "valid input - greater than 100",
			input: intstr.IntOrString{
				Type:   1,
				StrVal: "101%",
			},
			wantOutput: []*field.Error{
				{
					Type:  field.ErrorTypeInvalid,
					Field: "test",
					BadValue: intstr.IntOrString{
						Type:   1,
						StrVal: "101%",
					},
					Detail: "must not be greater than 100%",
				},
			},
		},
		{
			name: "valid input - less than 100",
			input: intstr.IntOrString{
				Type:   1,
				StrVal: "99%",
			},
			wantOutput: nil,
		},
		{
			name: "valid input - equal to 100",
			input: intstr.IntOrString{
				Type:   1,
				StrVal: "100%",
			},
			wantOutput: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testPath := field.NewPath("test")
			output := isNotMoreThan100Percent(tc.input, testPath)
			if diff := cmp.Diff(tc.wantOutput, output); diff != "" {
				t.Errorf("unexpected result: (-want, +got) %s", diff)
			}
		})
	}
}

func TestLeaderWorkerSetWebhookValidateCreateGangSchedulerProvider(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*leaderworkerset.LeaderWorkerSet)
		wantErr bool
	}{
		{
			name: "no opt-in",
		},
		{
			name: "valid koordinator opt-in",
			mutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
			},
		},
		{
			name: "unsupported provider value",
			mutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: "volcano",
				}
			},
			wantErr: true,
		},
		{
			name: "empty provider value",
			mutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: "",
				}
			},
			wantErr: true,
		},
		{
			name: "case sensitive provider value",
			mutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: "Koordinator",
				}
			},
			wantErr: true,
		},
		{
			name: "koordinator opt-in with LeaderReady",
			mutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
				lws.Spec.StartupPolicy = leaderworkerset.LeaderReadyStartupPolicy
			},
			wantErr: true,
		},
		{
			name: "reserved label in leader template",
			mutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Spec.LeaderWorkerTemplate.LeaderTemplate = &corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
						},
					},
				}
			},
			wantErr: true,
		},
		{
			name: "reserved label in worker template",
			mutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Spec.LeaderWorkerTemplate.WorkerTemplate.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := &LeaderWorkerSetWebhook{}
			lws := validLeaderWorkerSetForWebhookTest()
			if tt.mutate != nil {
				tt.mutate(lws)
			}
			require.NoError(t, webhook.Default(context.Background(), lws))

			_, err := webhook.ValidateCreate(context.Background(), lws)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLeaderWorkerSetWebhookValidateUpdateGangSchedulerProvider(t *testing.T) {
	tests := []struct {
		name      string
		oldMutate func(*leaderworkerset.LeaderWorkerSet)
		newMutate func(*leaderworkerset.LeaderWorkerSet)
		wantErr   bool
	}{
		{
			name: "unchanged without opt-in",
		},
		{
			name: "unchanged koordinator opt-in",
			oldMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
			},
		},
		{
			name: "adding opt-in is rejected",
			newMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
			},
			wantErr: true,
		},
		{
			name: "removing opt-in is rejected",
			oldMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
			},
			newMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				delete(lws.Labels, leaderworkerset.GangSchedulerProviderLabelKey)
			},
			wantErr: true,
		},
		{
			name: "changing opt-in is rejected",
			oldMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
			},
			newMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels[leaderworkerset.GangSchedulerProviderLabelKey] = "volcano"
			},
			wantErr: true,
		},
		{
			name: "removing unsupported provider value is allowed for cleanup",
			oldMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: "volcano",
				}
			},
			newMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				delete(lws.Labels, leaderworkerset.GangSchedulerProviderLabelKey)
			},
		},
		{
			name: "unrelated update with unsupported provider value is rejected",
			oldMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: "volcano",
				}
			},
			newMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Spec.Replicas = ptr.To[int32](2)
			},
			wantErr: true,
		},
		{
			name: "reserved label added to worker template is rejected",
			newMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Spec.LeaderWorkerTemplate.WorkerTemplate.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
			},
			wantErr: true,
		},
		{
			name: "reserved label added to leader template is rejected",
			newMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Spec.LeaderWorkerTemplate.LeaderTemplate = &corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
						},
					},
				}
			},
			wantErr: true,
		},
		{
			name: "unrelated update with unchanged opt-in succeeds",
			oldMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Labels = map[string]string{
					leaderworkerset.GangSchedulerProviderLabelKey: string(schedulerprovider.Koordinator),
				}
			},
			newMutate: func(lws *leaderworkerset.LeaderWorkerSet) {
				lws.Spec.Replicas = ptr.To[int32](2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := &LeaderWorkerSetWebhook{}
			oldLWS := validLeaderWorkerSetForWebhookTest()
			if tt.oldMutate != nil {
				tt.oldMutate(oldLWS)
			}
			require.NoError(t, webhook.Default(context.Background(), oldLWS))

			newLWS := oldLWS.DeepCopy()
			if tt.newMutate != nil {
				tt.newMutate(newLWS)
			}
			require.NoError(t, webhook.Default(context.Background(), newLWS))

			_, err := webhook.ValidateUpdate(context.Background(), oldLWS, newLWS)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func validLeaderWorkerSetForWebhookTest() *leaderworkerset.LeaderWorkerSet {
	return &leaderworkerset.LeaderWorkerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lws",
			Namespace: "default",
		},
		Spec: leaderworkerset.LeaderWorkerSetSpec{
			Replicas:      ptr.To[int32](1),
			StartupPolicy: leaderworkerset.LeaderCreatedStartupPolicy,
			LeaderWorkerTemplate: leaderworkerset.LeaderWorkerTemplate{
				Size: ptr.To[int32](2),
				WorkerTemplate: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "worker",
								Image: "busybox",
							},
						},
					},
				},
			},
		},
	}
}

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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSchedulerProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerType ProviderType
		wantProvider SchedulerProvider
		wantErr      bool
	}{
		{
			name:         "volcano",
			providerType: Volcano,
			wantProvider: &VolcanoProvider{},
		},
		{
			name:         "koordinator",
			providerType: Koordinator,
			wantProvider: &KoordinatorProvider{},
		},
		{
			name:         "unsupported",
			providerType: ProviderType("unsupported"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSchedulerProvider(tt.providerType, nil)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.IsType(t, tt.wantProvider, got)
		})
	}
}

func TestSupportedSchedulerProviders(t *testing.T) {
	assert.True(t, SupportedSchedulerProviders.Has(string(Volcano)))
	assert.True(t, SupportedSchedulerProviders.Has(string(Koordinator)))
}

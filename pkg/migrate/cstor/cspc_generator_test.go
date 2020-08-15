/*
Copyright 2020 The OpenEBS Authors

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

package migrate

import (
	"reflect"
	"testing"

	cstor "github.com/openebs/api/pkg/apis/cstor/v1"
	apis "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
)

func Test_getDataRaidGroups(t *testing.T) {
	type args struct {
		cspObj apis.CStorPool
	}
	tests := []struct {
		name string
		args args
		want []cstor.RaidGroup
	}{
		{
			name: "striped different raid groups",
			args: args{
				cspObj: apis.CStorPool{
					Spec: apis.CStorPoolSpec{
						PoolSpec: apis.CStorPoolAttr{
							PoolType: "striped",
						},
						Group: []apis.BlockDeviceGroup{
							{
								Item: []apis.CspBlockDevice{
									{
										Name: "sparse-37a7de580322f43a13338bf2467343f5",
									},
								},
							},
							{
								Item: []apis.CspBlockDevice{
									{
										Name: "sparse-5a92ced3e2ee21eac7b930f670b5eab5",
									},
								},
							},
							{
								Item: []apis.CspBlockDevice{
									{
										Name: "sparse-5e508018b4dd2c8e2530fbdae8e44bb6",
									},
								},
							},
						},
					},
				},
			},
			want: []cstor.RaidGroup{
				{
					CStorPoolInstanceBlockDevices: []cstor.CStorPoolInstanceBlockDevice{
						{
							BlockDeviceName: "sparse-37a7de580322f43a13338bf2467343f5",
						},
						{
							BlockDeviceName: "sparse-5a92ced3e2ee21eac7b930f670b5eab5",
						},
						{
							BlockDeviceName: "sparse-5e508018b4dd2c8e2530fbdae8e44bb6",
						},
					},
				},
			},
		},
		{
			name: "striped same raid group",
			args: args{
				cspObj: apis.CStorPool{
					Spec: apis.CStorPoolSpec{
						PoolSpec: apis.CStorPoolAttr{
							PoolType: "striped",
						},
						Group: []apis.BlockDeviceGroup{
							{
								Item: []apis.CspBlockDevice{
									{
										Name: "sparse-37a7de580322f43a13338bf2467343f5",
									},
									{
										Name: "sparse-5a92ced3e2ee21eac7b930f670b5eab5",
									},
									{
										Name: "sparse-5e508018b4dd2c8e2530fbdae8e44bb6",
									},
								},
							},
						},
					},
				},
			},
			want: []cstor.RaidGroup{
				{
					CStorPoolInstanceBlockDevices: []cstor.CStorPoolInstanceBlockDevice{
						{
							BlockDeviceName: "sparse-37a7de580322f43a13338bf2467343f5",
						},
						{
							BlockDeviceName: "sparse-5a92ced3e2ee21eac7b930f670b5eab5",
						},
						{
							BlockDeviceName: "sparse-5e508018b4dd2c8e2530fbdae8e44bb6",
						},
					},
				},
			},
		},
		{
			name: "mirrored",
			args: args{
				cspObj: apis.CStorPool{
					Spec: apis.CStorPoolSpec{
						PoolSpec: apis.CStorPoolAttr{
							PoolType: "mirrored",
						},
						Group: []apis.BlockDeviceGroup{
							{
								Item: []apis.CspBlockDevice{
									{
										Name: "sparse-37a7de580322f43a13338bf2467343f5",
									},
									{
										Name: "sparse-5a92ced3e2ee21eac7b930f670b5eab5",
									},
								},
							},
							{
								Item: []apis.CspBlockDevice{
									{
										Name: "sparse-5e508018b4dd2c8e2530fbdae8e44bb6",
									},
									{
										Name: "sparse-b6ffc62c1e15edd30c1b4150d897d5cb",
									},
								},
							},
						},
					},
				},
			},
			want: []cstor.RaidGroup{
				{
					CStorPoolInstanceBlockDevices: []cstor.CStorPoolInstanceBlockDevice{
						{
							BlockDeviceName: "sparse-37a7de580322f43a13338bf2467343f5",
						},
						{
							BlockDeviceName: "sparse-5a92ced3e2ee21eac7b930f670b5eab5",
						},
					},
				},
				{
					CStorPoolInstanceBlockDevices: []cstor.CStorPoolInstanceBlockDevice{
						{
							BlockDeviceName: "sparse-5e508018b4dd2c8e2530fbdae8e44bb6",
						},
						{
							BlockDeviceName: "sparse-b6ffc62c1e15edd30c1b4150d897d5cb",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDataRaidGroups(tt.args.cspObj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDataRaidGroups() = %v, want %v", got, tt.want)
			}
		})
	}
}

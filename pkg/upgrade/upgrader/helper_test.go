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

package upgrader

import "testing"

func Test_removeSuffixFromEnd(t *testing.T) {
	type args struct {
		str    string
		suffix string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "with suffix",
			args: args{
				str:    "openebs/cstor-operator-amd64",
				suffix: "-amd64",
			},
			want: "openebs/cstor-operator",
		},
		{
			name: "without suffix",
			args: args{
				str:    "openebs/cstor-operator",
				suffix: "-amd64",
			},
			want: "openebs/cstor-operator",
		},
		{
			name: "airgap with suffix",
			args: args{
				str:    "air-gap-having-amd64/openebs/cstor-operator-amd64",
				suffix: "-amd64",
			},
			want: "air-gap-having-amd64/openebs/cstor-operator",
		},
		{
			name: "airgap without suffix",
			args: args{
				str:    "air-gap-having-amd64/openebs/cstor-operator",
				suffix: "-amd64",
			},
			want: "air-gap-having-amd64/openebs/cstor-operator",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeSuffixFromEnd(tt.args.str, tt.args.suffix); got != tt.want {
				t.Errorf("removeSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}

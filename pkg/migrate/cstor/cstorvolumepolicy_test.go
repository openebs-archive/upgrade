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
	"testing"

	"github.com/google/go-cmp/cmp"
	cstor "github.com/openebs/api/v2/pkg/apis/cstor/v1"
	openebsFakeClientset "github.com/openebs/api/v2/pkg/client/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	casConfig = `
- name: StoragePoolClaim
  value: "sparse-claim"
- name: ReplicaCount
  value: "1"
- name: TargetResourceLimits
  value: |-
    memory: 1Gi
    cpu: 200m
- name: TargetResourceRequests
  value: |-
    memory: 500Mi
    cpu: 100m
- name: AuxResourceLimits
  value: |-
    memory: 1Gi
    cpu: 200m
- name: AuxResourceRequests
  value: |-
    memory: 500Mi
    cpu: 100m
- name: TargetTolerations
  value: |-
    t1:
      key: "key1"
      operator: "Equal"
      value: "value1"
      effect: "NoSchedule"
    t2:
      key: "key1"
      operator: "Equal"
      value: "value1"
      effect: "NoExecute"
- name: TargetNodeSelector
  value: |-
    nodetype: storage
- name: QueueDepth
  value: "32"
- name: Luworkers
  value: "6"
- name: ZvolWorkers
  value: "1"
`
)

// fixture encapsulates fake client sets and client-go testing objects.
// This is useful in mocking a controller.
type fixture struct {
	v              *VolumeMigrator
	openebsObjects []runtime.Object
}

// newFixture returns a new fixture
func newFixture() *fixture {
	f := &fixture{
		v: &VolumeMigrator{
			KubeClientset:    fake.NewSimpleClientset(),
			OpenebsClientset: openebsFakeClientset.NewSimpleClientset(),
			OpenebsNamespace: "openebs",
		},
	}
	return f
}

func TestVolumeMigrator_createCVPforConfig(t *testing.T) {
	f := newFixture()
	type args struct {
		sc *storagev1.StorageClass
	}
	tests := []struct {
		name    string
		args    args
		expect  *cstor.CStorVolumePolicy
		wantErr bool
	}{
		{
			name: "when cas config annotation is present on the storageclass",
			args: args{
				sc: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cstor-sparse",
						Annotations: map[string]string{
							"cas.openebs.io/config": casConfig,
						},
					},
					Parameters: map[string]string{},
				},
			},
			expect: &cstor.CStorVolumePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cstor-sparse",
					Namespace: "openebs",
				},
				Spec: cstor.CStorVolumePolicySpec{
					Target: cstor.TargetSpec{
						Resources: &v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("200m"),
								v1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("100m"),
								v1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						AuxResources: &v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("200m"),
								v1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("100m"),
								v1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						Tolerations: []v1.Toleration{
							{
								Key:      "key1",
								Operator: v1.TolerationOpEqual,
								Value:    "value1",
								Effect:   v1.TaintEffectNoSchedule,
							},
							{
								Key:      "key1",
								Operator: v1.TolerationOpEqual,
								Value:    "value1",
								Effect:   v1.TaintEffectNoExecute,
							},
						},
						NodeSelector: map[string]string{
							"nodetype": "storage",
						},
						QueueDepth: "32",
						IOWorkers:  6,
					},
					Replica: cstor.ReplicaSpec{
						IOWorkers: "1",
					},
				},
			},
		},
		{
			name: "when no cas config annotation is present on the storageclass",
			args: args{
				sc: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openebs-cstor-default",
					},
					Parameters: map[string]string{},
				},
			},
			wantErr: false,
			expect: &cstor.CStorVolumePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openebs-cstor-default",
					Namespace: "openebs",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := f.v.createCVPforConfig(tt.args.sc); (err != nil) != tt.wantErr {
				t.Errorf("VolumeMigrator.createCVPforConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			cvp, err := f.v.OpenebsClientset.CstorV1().CStorVolumePolicies(f.v.OpenebsNamespace).Get(tt.args.sc.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("VolumeMigrator.createCVPforConfig() failed to get CVP = %v, wantErr %v", err, tt.wantErr)
			}
			if !cmp.Equal(cvp, tt.expect) {
				t.Errorf("VolumeMigrator.createCVPforConfig() translation failed \nexpected : %+v\ngot : %+v", tt.expect, cvp)
			}

		})
	}
}

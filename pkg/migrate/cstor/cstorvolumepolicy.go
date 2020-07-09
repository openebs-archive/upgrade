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
	"strconv"
	"strings"

	cstor "github.com/openebs/api/pkg/apis/cstor/v1"
	"gopkg.in/yaml.v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type config struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type quantity struct {
	Memory string `yaml:"memory,omitempty"`
	CPU    string `yaml:"cpu,omitempty"`
}

// createCVPforConfig creates an equivalent CVP
// for the cas config annotation set on old SC
func (v *VolumeMigrator) createCVPforConfig(sc *storagev1.StorageClass) error {
	_, err := v.OpenebsClientset.CstorV1().
		CStorVolumePolicies(v.OpenebsNamespace).
		Get(sc.Name, metav1.GetOptions{})
	found := false
	if err == nil {
		found = true
	}
	if !k8serrors.IsNotFound(err) {
		return err
	}
	scConfig := sc.Annotations["cas.openebs.io/config"]
	scConfig = strings.TrimSpace(scConfig)
	configs := []config{}
	err = yaml.Unmarshal([]byte(scConfig), &configs)
	if err != nil {
		return err
	}
	cvp := &cstor.CStorVolumePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sc.Name,
			Namespace: v.OpenebsNamespace,
		},
	}
	for _, config := range configs {
		switch config.Name {
		case "TargetResourceRequests":
			res, err := parseResource(config.Value)
			if err != nil {
				return err
			}
			if cvp.Spec.Target.Resources == nil {
				cvp.Spec.Target.Resources = &corev1.ResourceRequirements{}
			}
			cvp.Spec.Target.Resources.Requests = res
		case "TargetResourceLimits":
			res, err := parseResource(config.Value)
			if err != nil {
				return err
			}
			if cvp.Spec.Target.Resources == nil {
				cvp.Spec.Target.Resources = &corev1.ResourceRequirements{}
			}
			cvp.Spec.Target.Resources.Limits = res
		case "AuxResourceRequests":
			res, err := parseResource(config.Value)
			if err != nil {
				return err
			}
			if cvp.Spec.Target.AuxResources == nil {
				cvp.Spec.Target.AuxResources = &corev1.ResourceRequirements{}
			}
			cvp.Spec.Target.AuxResources.Requests = res
		case "AuxResourceLimits":
			res, err := parseResource(config.Value)
			if err != nil {
				return err
			}
			if cvp.Spec.Target.AuxResources == nil {
				cvp.Spec.Target.AuxResources = &corev1.ResourceRequirements{}
			}
			cvp.Spec.Target.AuxResources.Limits = res
		case "TargetNodeSelector":
			x := map[string]string{}
			err = yaml.Unmarshal([]byte(config.Value), &x)
			if err != nil {
				panic(err.Error())
			}
			cvp.Spec.Target.NodeSelector = x
		case "TargetTolerations":
			tMap := map[string]corev1.Toleration{}
			err = yaml.Unmarshal([]byte(config.Value), &tMap)
			if err != nil {
				panic(err.Error())
			}
			t := []corev1.Toleration{}
			for _, y := range tMap {
				t = append(t, y)
			}
			cvp.Spec.Target.Tolerations = t
		case "Luworkers":
			cvp.Spec.Target.IOWorkers, err = strconv.ParseInt(config.Value, 10, 64)
			if err != nil {
				return err
			}
		case "QueueDepth":
			cvp.Spec.Target.QueueDepth = config.Value
		case "ZvolWorkers":
			cvp.Spec.Replica.IOWorkers = config.Value
		case "FSType":
			sc.Parameters["fsType"] = config.Value
		}
	}
	if !found {
		_, err = v.OpenebsClientset.CstorV1().
			CStorVolumePolicies(v.OpenebsNamespace).
			Create(cvp)
		if err != nil {
			return err
		}
	}
	sc.Parameters["cstorVolumePolicy"] = cvp.Name
	return nil
}

func parseResource(str string) (corev1.ResourceList, error) {
	q := quantity{}
	res := corev1.ResourceList{}
	err := yaml.Unmarshal([]byte(str), &q)
	if err != nil {
		return res, err
	}
	if q.Memory != "" {
		res[corev1.ResourceMemory] = resource.MustParse(q.Memory)

	}
	if q.CPU != "" {
		res[corev1.ResourceCPU] = resource.MustParse(q.CPU)
	}
	return res, nil
}

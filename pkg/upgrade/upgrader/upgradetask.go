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

import (
	"os"

	v1Alpha1API "github.com/openebs/api/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func updateUpgradeDetailedStatus(utaskObj *v1Alpha1API.UpgradeTask,
	uStatusObj v1Alpha1API.UpgradeDetailedStatuses,
	openebsNamespace string, client *Client,
) (*v1Alpha1API.UpgradeTask, error) {
	var err error
	if !isValidStatus(uStatusObj) {
		return nil, errors.Errorf(
			"failed to update upgradetask status: invalid status %v",
			uStatusObj,
		)
	}
	uStatusObj.LastUpdatedTime = metav1.Now()
	if uStatusObj.Phase == v1Alpha1API.StepWaiting {
		uStatusObj.StartTime = uStatusObj.LastUpdatedTime
		utaskObj.Status.UpgradeDetailedStatuses = append(
			utaskObj.Status.UpgradeDetailedStatuses,
			uStatusObj,
		)
	} else {
		l := len(utaskObj.Status.UpgradeDetailedStatuses)
		uStatusObj.StartTime = utaskObj.Status.UpgradeDetailedStatuses[l-1].StartTime
		utaskObj.Status.UpgradeDetailedStatuses[l-1] = uStatusObj
	}
	utaskObj, err = client.OpenebsClientset.OpenebsV1alpha1().
		UpgradeTasks(openebsNamespace).Update(utaskObj)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update upgradetask ")
	}
	return utaskObj, nil
}

// isValidStatus is used to validate IsValidStatus
func isValidStatus(o v1Alpha1API.UpgradeDetailedStatuses) bool {
	if o.Step == "" {
		return false
	}
	if o.Phase == "" {
		return false
	}
	if o.Message == "" && o.Phase != v1Alpha1API.StepWaiting {
		return false
	}
	if o.Reason == "" && o.Phase == v1Alpha1API.StepErrored {
		return false
	}
	return true
}

// getOrCreateUpgradeTask fetches upgrade task if provided or creates a new upgradetask CR
func getOrCreateUpgradeTask(kind string, r *ResourcePatch, client *Client) (*v1Alpha1API.UpgradeTask, error) {
	var utaskObj *v1Alpha1API.UpgradeTask
	var err error
	if r.OpenebsNamespace == "" {
		return nil, errors.Errorf("missing openebsNamespace")
	}
	if kind == "" {
		return nil, errors.Errorf("missing kind for upgradeTask")
	}
	if r.Name == "" {
		return nil, errors.Errorf("missing name for upgradeTask")
	}
	utaskObj = buildUpgradeTask(kind, r)
	// the below logic first tries to fetch the CR if not found
	// then creates a new CR
	utaskObj1, err1 := client.OpenebsClientset.OpenebsV1alpha1().
		UpgradeTasks(r.OpenebsNamespace).
		Get(utaskObj.Name, metav1.GetOptions{})
	if err1 != nil {
		if k8serror.IsNotFound(err1) {
			utaskObj, err = client.OpenebsClientset.OpenebsV1alpha1().
				UpgradeTasks(r.OpenebsNamespace).Create(utaskObj)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err1
		}
	} else {
		utaskObj = utaskObj1
	}

	if utaskObj.Status.StartTime.IsZero() {
		utaskObj.Status.Phase = v1Alpha1API.UpgradeStarted
		utaskObj.Status.StartTime = metav1.Now()
	}

	utaskObj.Status.UpgradeDetailedStatuses = []v1Alpha1API.UpgradeDetailedStatuses{}
	utaskObj, err = client.OpenebsClientset.OpenebsV1alpha1().
		UpgradeTasks(r.OpenebsNamespace).
		Update(utaskObj)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update upgradetask")
	}
	return utaskObj, nil
}

func buildUpgradeTask(kind string, r *ResourcePatch) *v1Alpha1API.UpgradeTask {
	// TODO builder
	utaskObj := &v1Alpha1API.UpgradeTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.OpenebsNamespace,
		},
		Spec: v1Alpha1API.UpgradeTaskSpec{
			FromVersion: r.From,
			ToVersion:   r.To,
			ImageTag:    r.ImageTag,
			ImagePrefix: r.BaseURL,
		},
		Status: v1Alpha1API.UpgradeTaskStatus{
			Phase:     v1Alpha1API.UpgradeStarted,
			StartTime: metav1.Now(),
		},
	}
	switch kind {
	case "cstorPoolInstance":
		utaskObj.Name = "upgrade-cstor-cspi-" + r.Name
		utaskObj.Spec.ResourceSpec = v1Alpha1API.ResourceSpec{
			CStorPoolInstance: &v1Alpha1API.CStorPoolInstance{
				CSPIName: r.Name,
			},
		}
	case "cstorVolume":
		utaskObj.Name = "upgrade-cstor-csi-volume-" + r.Name
		utaskObj.Spec.ResourceSpec = v1Alpha1API.ResourceSpec{
			CStorVolume: &v1Alpha1API.CStorVolume{
				PVName: r.Name,
			},
		}
	}
	return utaskObj
}

func getBackoffLimit(openebsNamespace string, client *Client) (int, error) {
	podName := os.Getenv("POD_NAME")
	podObj, err := client.KubeClientset.CoreV1().Pods(openebsNamespace).
		Get(podName, metav1.GetOptions{})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get backoff limit")
	}
	jobObj, err := client.KubeClientset.BatchV1().Jobs(openebsNamespace).
		Get(podObj.OwnerReferences[0].Name, metav1.GetOptions{})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get backoff limit")
	}
	// if backoffLimit not present it returns the default as 6
	if jobObj.Spec.BackoffLimit == nil {
		return 6, nil
	}
	backoffLimit := int(*jobObj.Spec.BackoffLimit)
	return backoffLimit, nil
}

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
	v1Alpha1API "github.com/openebs/api/v2/pkg/apis/openebs.io/v1alpha1"
	openebsclientset "github.com/openebs/api/v2/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func updateMigrationDetailedStatus(mtaskObj *v1Alpha1API.MigrationTask,
	mStatusObj v1Alpha1API.MigrationDetailedStatuses,
	openebsNamespace string, client openebsclientset.Interface,
) (*v1Alpha1API.MigrationTask, error) {
	var err error
	if !isValidStatus(mStatusObj) {
		return nil, errors.Errorf(
			"failed to update migratetask status: invalid status %v",
			mStatusObj,
		)
	}
	mStatusObj.LastUpdatedTime = metav1.Now()
	if mStatusObj.Phase == v1Alpha1API.StepWaiting {
		mStatusObj.StartTime = mStatusObj.LastUpdatedTime
		mtaskObj.Status.MigrationDetailedStatuses = append(
			mtaskObj.Status.MigrationDetailedStatuses,
			mStatusObj,
		)
	} else {
		l := len(mtaskObj.Status.MigrationDetailedStatuses)
		mStatusObj.StartTime = mtaskObj.Status.MigrationDetailedStatuses[l-1].StartTime
		mtaskObj.Status.MigrationDetailedStatuses[l-1] = mStatusObj
	}
	mtaskObj, err = client.OpenebsV1alpha1().
		MigrationTasks(openebsNamespace).Update(mtaskObj)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update migratetask ")
	}
	return mtaskObj, nil
}

// isValidStatus is used to validate IsValidStatus
func isValidStatus(o v1Alpha1API.MigrationDetailedStatuses) bool {
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

// getOrCreateMigrationTask fetches migrate task if provided or creates a new migrationtask CR
func getOrCreateMigrationTask(kind, name, openebsNamespace string, r Migrator,
	client openebsclientset.Interface) (*v1Alpha1API.MigrationTask, error) {
	var mtaskObj *v1Alpha1API.MigrationTask
	var err error
	mtaskObj = buildMigrationTask(kind, name, r)
	// the below logic first tries to fetch the CR if not found
	// then creates a new CR
	mtaskObj1, err1 := client.OpenebsV1alpha1().
		MigrationTasks(openebsNamespace).
		Get(mtaskObj.Name, metav1.GetOptions{})
	if err1 != nil {
		if k8serror.IsNotFound(err1) {
			mtaskObj, err = client.OpenebsV1alpha1().
				MigrationTasks(openebsNamespace).Create(mtaskObj)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err1
		}
	} else {
		mtaskObj = mtaskObj1
	}

	if mtaskObj.Status.StartTime.IsZero() {
		mtaskObj.Status.Phase = v1Alpha1API.MigrateStarted
		mtaskObj.Status.StartTime = metav1.Now()
	}

	mtaskObj.Status.MigrationDetailedStatuses = []v1Alpha1API.MigrationDetailedStatuses{}
	mtaskObj, err = client.OpenebsV1alpha1().
		MigrationTasks(openebsNamespace).
		Update(mtaskObj)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update migratetask")
	}
	return mtaskObj, nil
}

func buildMigrationTask(kind, name string, m Migrator) *v1Alpha1API.MigrationTask {
	// TODO builder
	mtaskObj := &v1Alpha1API.MigrationTask{
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       v1Alpha1API.MigrationTaskSpec{},
		Status: v1Alpha1API.MigrationTaskStatus{
			Phase:     v1Alpha1API.MigrateStarted,
			StartTime: metav1.Now(),
		},
	}
	switch kind {
	case "cstorPool":
		r := m.(*CSPCMigrator)
		mtaskObj.Name = "migrate-cstor-pool-" + name
		mtaskObj.Spec.MigrateResource = v1Alpha1API.MigrateResource{
			MigrateCStorPool: &v1Alpha1API.MigrateCStorPool{
				SPCName: name,
				Rename:  r.CSPCName,
			},
		}
	case "cstorVolume":
		r := m.(*VolumeMigrator)
		mtaskObj.Name = "migrate-cstor-volume-" + name
		mtaskObj.Spec.MigrateResource = v1Alpha1API.MigrateResource{
			MigrateCStorVolume: &v1Alpha1API.MigrateCStorVolume{
				PVName: r.PVName,
			},
		}
	}
	return mtaskObj
}

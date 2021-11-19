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
	"context"

	cstor "github.com/openebs/api/v3/pkg/apis/cstor/v1"
	openebsio "github.com/openebs/api/v3/pkg/apis/openebs.io/v1alpha1"
	"github.com/openebs/api/v3/pkg/apis/types"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TranslateBackupToV1 translates v1alpha resources to v1
func TranslateBackupToV1(oldBackup openebsio.CStorBackup) *cstor.CStorBackup {
	newBackup := &cstor.CStorBackup{}
	newBackup.ObjectMeta = oldBackup.ObjectMeta
	newBackup.Spec = cstor.CStorBackupSpec{
		BackupName:   oldBackup.Spec.BackupName,
		VolumeName:   oldBackup.Spec.VolumeName,
		SnapName:     oldBackup.Spec.SnapName,
		PrevSnapName: oldBackup.Spec.PrevSnapName,
		BackupDest:   oldBackup.Spec.BackupDest,
		LocalSnap:    oldBackup.Spec.LocalSnap,
	}
	newBackup.Status = cstor.CStorBackupStatus(oldBackup.Status)
	return newBackup
}

// TranslateRestoreToV1 translates v1alpha resources to v1
func TranslateRestoreToV1(oldRestore openebsio.CStorRestore) *cstor.CStorRestore {
	newRestore := &cstor.CStorRestore{}
	newRestore.ObjectMeta = oldRestore.ObjectMeta
	newRestore.Spec = cstor.CStorRestoreSpec{
		RestoreName:   oldRestore.Spec.RestoreName,
		VolumeName:    oldRestore.Spec.VolumeName,
		RestoreSrc:    oldRestore.Spec.RestoreSrc,
		MaxRetryCount: oldRestore.Spec.MaxRetryCount,
		RetryCount:    oldRestore.Spec.RetryCount,
		StorageClass:  oldRestore.Spec.StorageClass,
		Size:          oldRestore.Spec.Size,
		Local:         oldRestore.Spec.Local,
	}
	newRestore.Status = cstor.CStorRestoreStatus(oldRestore.Status)
	return newRestore
}

// TranslateCompletedBackupToV1 translates v1alpha resources to v1
func TranslateCompletedBackupToV1(oldCompletedBackup openebsio.CStorCompletedBackup) *cstor.CStorCompletedBackup {
	newCompletedBackup := &cstor.CStorCompletedBackup{}
	newCompletedBackup.ObjectMeta = oldCompletedBackup.ObjectMeta
	newCompletedBackup.Spec = cstor.CStorCompletedBackupSpec{
		BackupName:         oldCompletedBackup.Spec.BackupName,
		VolumeName:         oldCompletedBackup.Spec.VolumeName,
		LastSnapName:       oldCompletedBackup.Spec.PrevSnapName,
		SecondLastSnapName: oldCompletedBackup.Spec.SnapName,
	}
	return newCompletedBackup
}

func (c *CSPCMigrator) upgradeBackupRestore(cspUID string, cspiObj *cstor.CStorPoolInstance) error {
	// Migrate backup to v1 version
	oldBackupList, err := c.OpenebsClientset.OpenebsV1alpha1().
		CStorBackups(c.OpenebsNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: cspUIDLabel + "=" + cspUID,
	})
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to list v1alpha1 cstorbackups")
	}
	for _, oldBackup := range oldBackupList.Items {
		newBackup := TranslateBackupToV1(oldBackup)
		newBackup.Labels[types.CStorPoolInstanceNameLabelKey] = cspiObj.Name
		newBackup.Labels[types.CStorPoolInstanceUIDLabelKey] = string(cspiObj.UID)
		delete(newBackup.Labels, cspUIDLabel)
		_, err := c.OpenebsClientset.CstorV1().
			CStorBackups(c.OpenebsNamespace).Create(context.TODO(), newBackup, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create v1 cstorbackup for %s", oldBackup.Name)
		}

	}
	if len(oldBackupList.Items) != 0 {
		err = c.OpenebsClientset.OpenebsV1alpha1().
			CStorBackups(c.OpenebsNamespace).
			DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: cspUIDLabel + "=" + cspUID,
			})
		if err != nil && !k8serrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to delete v1alpha1 cstorbackups")
		}
	}

	// Migrate restore to v1 version
	oldRestoreList, err := c.OpenebsClientset.OpenebsV1alpha1().
		CStorRestores(c.OpenebsNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: cspUIDLabel + "=" + cspUID,
	})
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to list v1alpha1 cstorrestores")
	}
	for _, oldRestore := range oldRestoreList.Items {
		newRestore := TranslateRestoreToV1(oldRestore)
		newRestore.Labels[types.CStorPoolInstanceNameLabelKey] = cspiObj.Name
		newRestore.Labels[types.CStorPoolInstanceUIDLabelKey] = string(cspiObj.UID)
		delete(newRestore.Labels, cspUIDLabel)
		_, err := c.OpenebsClientset.CstorV1().
			CStorRestores(c.OpenebsNamespace).Create(context.TODO(), newRestore, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create v1 cstorrestore for %s", oldRestore.Name)
		}

	}
	if len(oldRestoreList.Items) != 0 {
		err = c.OpenebsClientset.OpenebsV1alpha1().
			CStorRestores(c.OpenebsNamespace).
			DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: cspUIDLabel + "=" + cspUID,
			})
		if err != nil && !k8serrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to delete v1alpha1 cstorrestores")
		}
	}

	// Migrate completedbackup to v1 version
	oldCompletedBackupList, err := c.OpenebsClientset.OpenebsV1alpha1().
		CStorCompletedBackups(c.OpenebsNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: cspUIDLabel + "=" + cspUID,
	})
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to list v1alpha1 cstorcompletedbackups")
	}
	for _, oldCompletedBackup := range oldCompletedBackupList.Items {
		newCompletedBackup := TranslateCompletedBackupToV1(oldCompletedBackup)
		newCompletedBackup.Labels[types.CStorPoolInstanceNameLabelKey] = cspiObj.Name
		newCompletedBackup.Labels[types.CStorPoolInstanceUIDLabelKey] = string(cspiObj.UID)
		delete(newCompletedBackup.Labels, cspUIDLabel)
		_, err := c.OpenebsClientset.CstorV1().
			CStorCompletedBackups(c.OpenebsNamespace).Create(context.TODO(), newCompletedBackup, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create v1 cstorcompletedbackup for %s", oldCompletedBackup.Name)
		}

	}
	if len(oldBackupList.Items) != 0 {
		err = c.OpenebsClientset.OpenebsV1alpha1().
			CStorCompletedBackups(c.OpenebsNamespace).
			DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: cspUIDLabel + "=" + cspUID,
			})
		if err != nil && !k8serrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to delete v1alpha1 cstorcompletedbackups")
		}
	}
	return nil
}

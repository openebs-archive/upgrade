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
	"time"

	errors "github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

// IsVolumeMounted checks if the volume is mounted into any pod.
// This check is required as if mounted the pod will not allow
// deleting the pvc for recreation into csi volume.
func (v *VolumeMigrator) IsVolumeMounted(pvName string) (*corev1.PersistentVolume, error) {
	pvObj, err := v.KubeClientset.CoreV1().
		PersistentVolumes().
		Get(context.TODO(), pvName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	pvcName := pvObj.Spec.ClaimRef.Name
	pvcNamespace := pvObj.Spec.ClaimRef.Namespace
	podList, err := v.KubeClientset.CoreV1().Pods(pvcNamespace).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, podObj := range podList.Items {
		for _, volume := range podObj.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				if volume.PersistentVolumeClaim.ClaimName == pvcName {
					return nil, errors.Errorf(
						"the volume %s is mounted on %s, please scale down all apps before migrating",
						pvName,
						podObj.Name,
					)
				}
			}
		}
	}
	return pvObj, nil
}

// RetainPV sets the Retain policy on the PV.
// This operation is performed to prevent deletion of the OpenEBS
// resources while deleting the pvc to recreate with migrated spec.
func (v *VolumeMigrator) RetainPV(pvObj *corev1.PersistentVolume) error {
	pvObj.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
	_, err := v.KubeClientset.CoreV1().
		PersistentVolumes().
		Update(context.TODO(), pvObj, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// RecreatePV recreates PV for the given PV object by first deleting
// the old PV with same name and creating a new PV having claimRef same
// as previous PV except for the uid to avoid any other PVC to claim it.
func (v *VolumeMigrator) RecreatePV(pvObj *corev1.PersistentVolume) (*corev1.PersistentVolume, error) {
	err := v.KubeClientset.CoreV1().
		PersistentVolumes().
		Delete(context.TODO(), pvObj.Name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	err = v.isPVDeletedEventually(pvObj)
	if err != nil {
		return nil, err
	}
	pvObj, err = v.KubeClientset.CoreV1().
		PersistentVolumes().
		Create(context.TODO(), pvObj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return pvObj, nil
}

// RecreatePVC recreates PVC for the given PVC object by first deleting
// the old PVC with same name and creating a new PVC.
func (v *VolumeMigrator) RecreatePVC(pvcObj *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	err := v.KubeClientset.CoreV1().
		PersistentVolumeClaims(pvcObj.Namespace).
		Delete(context.TODO(), pvcObj.Name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	err = v.isPVCDeletedEventually(pvcObj)
	if err != nil {
		return nil, err
	}
	pvcObj, err = v.KubeClientset.CoreV1().
		PersistentVolumeClaims(pvcObj.Namespace).
		Create(context.TODO(), pvcObj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return pvcObj, nil
}

// IsPVCDeletedEventually tries to get the deleted pvc
// and returns true if pvc is not found
// else returns false
func (v *VolumeMigrator) isPVCDeletedEventually(pvcObj *corev1.PersistentVolumeClaim) error {
	for i := 1; i < 60; i++ {
		_, err := v.KubeClientset.CoreV1().
			PersistentVolumeClaims(pvcObj.Namespace).
			Get(context.TODO(), pvcObj.Name, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			return nil
		}
		klog.Infof("Waiting for pvc %s to go away", pvcObj.Name)
		time.Sleep(5 * time.Second)
	}
	return errors.Errorf("PVC %s still present", pvcObj.Name)
}

// IsPVDeletedEventually tries to get the deleted pv
// and returns true if pv is not found
// else returns false
func (v *VolumeMigrator) isPVDeletedEventually(pvObj *corev1.PersistentVolume) error {
	for i := 1; i < 60; i++ {
		_, err := v.KubeClientset.CoreV1().
			PersistentVolumes().
			Get(context.TODO(), pvObj.Name, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			return nil
		}
		klog.Infof("Waiting for pv %s to go away", pvObj.Name)
		time.Sleep(5 * time.Second)
	}
	return errors.Errorf("PV %s still present", pvObj.Name)
}

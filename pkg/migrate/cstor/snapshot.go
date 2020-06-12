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
	snapv1beta1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/clientset/versioned"
	snapv1 "github.com/openebs/maya/pkg/apis/openebs.io/snapshot/v1"
	snap "github.com/openebs/maya/pkg/kubernetes/snapshot/v1alpha1"
	snapData "github.com/openebs/maya/pkg/kubernetes/snapshotdata/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

// SnapshotMigrator ...
type SnapshotMigrator struct {
	pvName     string
	snapClient *snapclientset.Clientset
}

var (
	snapClass = "csi-cstor-snapshotclass"
)

func (s *SnapshotMigrator) migrate(pvName string) error {
	s.pvName = pvName
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(err, "error building kubeconfig")
	}
	// snapclientset.NewForConfig creates a new Clientset for VolumesnapshotV1beta1Client
	s.snapClient, err = snapclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Failed to create snapshot client: %v", err)
	}
	return s.migrateSnapshots()
}

func (s *SnapshotMigrator) migrateSnapshots() error {
	_, err := s.snapClient.SnapshotV1beta1().VolumeSnapshotClasses().
		Get(snapClass, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get snapshotclass %s", snapClass)
	}
	snapshotList, err := snap.NewKubeClient().
		WithNamespace("").
		List(metav1.ListOptions{
			LabelSelector: "SnapshotMetadata-PVName=" + s.pvName,
		})
	if err != nil {
		return err
	}
	for _, snapshot := range snapshotList.Items {
		snapshot := snapshot // pin it
		err = s.migrateSnapshot(&snapshot)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SnapshotMigrator) migrateSnapshot(oldSnap *snapv1.VolumeSnapshot) error {
	snapshotData, err := snapData.NewKubeClient().
		Get(oldSnap.Spec.SnapshotDataName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get volumesnapshotdata %s for %s", oldSnap.Spec.SnapshotDataName, oldSnap.Name)
	}
	klog.Infof("Creating equivalent volumesnapshotcontent for volumesnapshotdata %s", snapshotData.Name)
	snapContent, err := s.createSnapContent(snapshotData, oldSnap)
	if err != nil {
		return errors.Wrapf(err, "failed to create equivalent volumesnapshotcontent for volumesnapshotdata %s", snapshotData.Name)
	}
	klog.Infof("Creating equivalent new csi volumesnapshot for old volumesnapshot %s", oldSnap.Name)
	newSnap, err := s.createNewSnapShot(snapContent, oldSnap)
	if err != nil {
		return errors.Wrapf(err, "failed to create equivalent new csi volumesnapshot for old volumesnapshot %s", oldSnap.Name)
	}
	klog.Infof("Validating new csi volumesnapshot %s is bound to volumesnapshotcontent %s", newSnap.Name, snapContent.Name)
	err = s.validateMigration(snapContent, newSnap)
	if err != nil {
		return errors.Wrapf(err, "failed to validate new volumesnapshot %")
	}
	klog.Infof("Cleaing up old volumesnapshot %s", oldSnap.Name)
	err = snap.NewKubeClient().WithNamespace(oldSnap.Namespace).Delete(oldSnap.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to delete old volumesnapshot %s", oldSnap.Name)
	}
	return nil
}

func (s *SnapshotMigrator) createSnapContent(snapshotData *snapv1.VolumeSnapshotData, oldSnap *snapv1.VolumeSnapshot) (
	*snapv1beta1.VolumeSnapshotContent, error) {
	snapContent, err := s.snapClient.SnapshotV1beta1().VolumeSnapshotContents().
		Get(snapshotData.Name, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	if err == nil {
		return snapContent, nil
	}
	snapHandle := snapshotData.Spec.PersistentVolumeRef.Name + "@" + snapshotData.Spec.OpenEBSSnapshot.SnapshotID
	snapContent = &snapv1beta1.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: snapshotData.Name,
		},
		Spec: snapv1beta1.VolumeSnapshotContentSpec{
			DeletionPolicy:          snapv1beta1.VolumeSnapshotContentDelete,
			Driver:                  cstorCSIDriver,
			VolumeSnapshotClassName: &snapClass,
			Source: snapv1beta1.VolumeSnapshotContentSource{
				SnapshotHandle: &snapHandle,
			},
			VolumeSnapshotRef: corev1.ObjectReference{
				APIVersion: "snapshot.storage.k8s.io/v1beta1",
				Kind:       "VolumeSnapshot",
				Name:       oldSnap.Name,
				Namespace:  oldSnap.Namespace,
			},
		},
	}
	return s.snapClient.SnapshotV1beta1().VolumeSnapshotContents().Create(snapContent)
}

func (s *SnapshotMigrator) createNewSnapShot(snapContent *snapv1beta1.VolumeSnapshotContent, oldSnap *snapv1.VolumeSnapshot) (
	*snapv1beta1.VolumeSnapshot, error) {
	newSnap, err := s.snapClient.SnapshotV1beta1().VolumeSnapshots(oldSnap.Namespace).
		Get(oldSnap.Name, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	if err == nil {
		return newSnap, nil
	}
	newSnap = &snapv1beta1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oldSnap.Name,
			Namespace: oldSnap.Namespace,
		},
		Spec: snapv1beta1.VolumeSnapshotSpec{
			Source: snapv1beta1.VolumeSnapshotSource{
				VolumeSnapshotContentName: &snapContent.Name,
			},
			VolumeSnapshotClassName: &snapClass,
		},
	}
	return s.snapClient.SnapshotV1beta1().VolumeSnapshots(oldSnap.Namespace).
		Create(newSnap)
}

func (s *SnapshotMigrator) validateMigration(snapContent *snapv1beta1.VolumeSnapshotContent, newSnap *snapv1beta1.VolumeSnapshot) error {
retry:
	newSnap, err := s.snapClient.SnapshotV1beta1().
		VolumeSnapshots(newSnap.Namespace).
		Get(newSnap.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if newSnap.Status == nil || newSnap.Status.BoundVolumeSnapshotContentName == nil {
		goto retry
	}
	if *newSnap.Status.BoundVolumeSnapshotContentName != snapContent.Name {
		return errors.Errorf("volumesnapshot %s is bound to incorrect volumesnapshotcontent: expected %s got %s",
			newSnap.Name, snapContent.Name, *newSnap.Status.BoundVolumeSnapshotContentName,
		)
	}
	return nil
}

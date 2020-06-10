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
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	goversion "github.com/hashicorp/go-version"
	cstor "github.com/openebs/api/pkg/apis/cstor/v1"
	"github.com/openebs/api/pkg/apis/types"
	openebsclientset "github.com/openebs/api/pkg/client/clientset/versioned"
	apis "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	cv "github.com/openebs/maya/pkg/cstor/volume/v1alpha1"
	cvr "github.com/openebs/maya/pkg/cstor/volumereplica/v1alpha1"
	"github.com/openebs/upgrade/pkg/version"
	errors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

var (
	cvcKind                = "CStorVolumeConfig"
	trueBool               = true
	cstorCSIDriver         = "cstor.csi.openebs.io"
	timeStamp              = time.Now().UnixNano() / int64(time.Millisecond)
	csiProvisionerIdentity = strconv.FormatInt(timeStamp, 10) + "-" + strconv.Itoa(rand.Intn(10000)) + "-" + "cstor.csi.openebs.io"
)

// VolumeMigrator ...
type VolumeMigrator struct {
	// kubeclientset is a standard kubernetes clientset
	KubeClientset kubernetes.Interface
	// openebsclientset is a openebs custom resource package generated for custom API group.
	OpenebsClientset openebsclientset.Interface
	PVName           string
	OpenebsNamespace string
	CVNamespace      string
	StorageClass     *storagev1.StorageClass
}

// Migrate is the interface implementation for
func (v *VolumeMigrator) Migrate(pvName, openebsNamespace string) error {
	v.PVName = pvName
	v.OpenebsNamespace = openebsNamespace
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(err, "error building kubeconfig")
	}
	v.KubeClientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "error building kubernetes clientset")
	}
	v.OpenebsClientset, err = openebsclientset.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "error building openebs clientset")
	}
	err = v.validateCVCOperator()
	if err != nil {
		return err
	}
	shouldMigrate, err := v.isMigrationRequired()
	if err != nil {
		return err
	}
	if shouldMigrate {
		err = v.migrate()
		if err != nil {
			return err
		}
	} else {
		klog.Infof("Volume %s already migrated to csi spec", pvName)
	}
	return v.deleteTempPolicy()
}

func (v *VolumeMigrator) isMigrationRequired() (bool, error) {
	cvList, err := cv.NewKubeclient().WithNamespace("").
		List(metav1.ListOptions{
			LabelSelector: "openebs.io/persistent-volume=" + v.PVName,
		})
	if err != nil && !k8serrors.IsNotFound(err) {
		return false, err
	}
	if err == nil && len(cvList.Items) != 0 {
		return true, nil
	}
	_, err = v.OpenebsClientset.CstorV1().
		CStorVolumes(v.OpenebsNamespace).Get(v.PVName, metav1.GetOptions{})
	if err == nil {
		return false, nil
	}
	return false, err
}

func (v *VolumeMigrator) deleteTempPolicy() error {
	err := v.OpenebsClientset.CstorV1().
		CStorVolumePolicies(v.OpenebsNamespace).
		Delete(v.PVName, &metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (v *VolumeMigrator) validateCVCOperator() error {
	v1110, _ := goversion.NewVersion("1.11.0")
	operatorPods, err := v.KubeClientset.CoreV1().
		Pods(v.OpenebsNamespace).
		List(metav1.ListOptions{
			LabelSelector: "openebs.io/component-name=cvc-operator",
		})
	if err != nil {
		return err
	}
	if len(operatorPods.Items) == 0 {
		return fmt.Errorf("cvc operator pod missing")
	}
	for _, pod := range operatorPods.Items {
		operatorVersion := strings.Split(pod.Labels["openebs.io/version"], "-")[0]
		vOperator, err := goversion.NewVersion(operatorVersion)
		if err != nil {
			return errors.Wrap(err, "failed to get cvc operator version")
		}
		if vOperator.LessThan(v1110) {
			return fmt.Errorf("cvc operator is in %s version, please upgrade it to 1.11.0 or above version",
				pod.Labels["openebs.io/version"])
		}
	}
	return nil
}

// migrate migrates the volume from non-CSI schema to CSI schema
func (v *VolumeMigrator) migrate() error {
	var pvObj *corev1.PersistentVolume
	pvcObj, pvPresent, err := v.validatePVName()
	if err != nil {
		return errors.Wrapf(err, "failed to validate pvname")
	}
	err = v.populateCVNamespace(v.PVName)
	if err != nil {
		return errors.Wrapf(err, "failed to cv namespace")
	}
	err = v.createTempPolicy()
	if err != nil {
		return errors.Wrapf(err, "failed to create temporary policy")
	}
	if pvPresent {
		klog.Infof("Checking volume is not mounted on any application")
		pvObj, err = v.IsVolumeMounted(v.PVName)
		if err != nil {
			return errors.Wrapf(err, "failed to verify mount status for pv {%s}", v.PVName)
		}
		if pvObj.Spec.PersistentVolumeSource.CSI == nil {
			klog.Infof("Retaining PV to migrate into csi volume")
			err = v.RetainPV(pvObj)
			if err != nil {
				return errors.Wrapf(err, "failed to retain pv {%s}", v.PVName)
			}
		}
		err = v.updateStorageClass(pvObj.Name, pvObj.Spec.StorageClassName)
		if err != nil {
			return errors.Wrapf(err, "failed to update storageclass {%s}", pvObj.Spec.StorageClassName)
		}
		pvcObj, err = v.migratePVC(pvObj)
		if err != nil {
			return errors.Wrapf(err, "failed to migrate pvc to csi spec")
		}
	} else {
		klog.Infof("PVC and storageclass already migrated to csi format")
	}
	v.StorageClass, err = v.KubeClientset.StorageV1().
		StorageClasses().Get(*pvcObj.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	_, err = v.migratePV(pvcObj)
	if err != nil {
		return errors.Wrapf(err, "failed to migrate pv to csi spec")
	}
	err = v.removeOldTarget()
	if err != nil {
		return errors.Wrapf(err, "failed to remove old target deployment")
	}
	klog.Infof("Creating CVC to bound the volume and trigger CSI driver")
	err = v.createCVC(v.PVName)
	if err != nil {
		return errors.Wrapf(err, "failed to create cvc")
	}
	err = v.validateMigratedVolume()
	if err != nil {
		return errors.Wrapf(err, "failed to validate migrated volume")
	}
	err = v.patchTargetPodAffinity()
	if err != nil {
		return errors.Wrap(err, "failed to patch target affinity")
	}
	err = v.cleanupOldResources()
	if err != nil {
		return errors.Wrapf(err, "failed to cleanup old volume resources")
	}
	return nil
}

func (v *VolumeMigrator) migratePVC(pvObj *corev1.PersistentVolume) (*corev1.PersistentVolumeClaim, error) {
	pvcObj, recreateRequired, err := v.generateCSIPVC(pvObj.Name)
	if err != nil {
		return nil, err
	}
	if recreateRequired {
		err := v.addSkipAnnotationToPVC(pvcObj)
		if err != nil {
			return nil, errors.Wrap(err, "failed to add skip-validations annotation")
		}
		klog.Infof("Recreating equivalent CSI PVC")
		pvcObj, err = v.RecreatePVC(pvcObj)
		if err != nil {
			return nil, err
		}
	}
	return pvcObj, nil
}

func (v *VolumeMigrator) addSkipAnnotationToPVC(pvcObj *corev1.PersistentVolumeClaim) error {
	oldPVC, err := v.KubeClientset.CoreV1().
		PersistentVolumeClaims(pvcObj.Namespace).Get(pvcObj.Name, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if oldPVC.Annotations["openebs.io/skip-validations"] != "true" {
		newPVC := oldPVC.DeepCopy()
		newPVC.Annotations["openebs.io/skip-validations"] = "true"
		data, err := GetPatchData(oldPVC, newPVC)
		if err != nil {
			return err
		}
		_, err = v.KubeClientset.CoreV1().PersistentVolumeClaims(oldPVC.Namespace).Patch(
			oldPVC.Name,
			k8stypes.StrategicMergePatchType,
			data,
		)
	}
	return err
}

func (v *VolumeMigrator) migratePV(pvcObj *corev1.PersistentVolumeClaim) (*corev1.PersistentVolume, error) {
	pvObj, recreateRequired, err := v.generateCSIPV(pvcObj.Spec.VolumeName, pvcObj)
	if err != nil {
		return nil, err
	}
	if recreateRequired {
		klog.Infof("Recreating equivalent CSI PV")
		_, err = v.RecreatePV(pvObj)
		if err != nil {
			return nil, err
		}
	}
	return pvObj, nil
}

func (v *VolumeMigrator) generateCSIPVC(pvName string) (*corev1.PersistentVolumeClaim, bool, error) {
	pvObj, err := v.KubeClientset.CoreV1().
		PersistentVolumes().
		Get(pvName, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	pvcName := pvObj.Spec.ClaimRef.Name
	pvcNamespace := pvObj.Spec.ClaimRef.Namespace
	pvcObj, err := v.KubeClientset.CoreV1().PersistentVolumeClaims(pvcNamespace).
		Get(pvcName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, false, err
		}
	}
	if pvcObj.Annotations["volume.beta.kubernetes.io/storage-provisioner"] != cstorCSIDriver {
		klog.Infof("Generating equivalent CSI PVC %s", pvcName)
		csiPVC := &corev1.PersistentVolumeClaim{}
		csiPVC.Name = pvcName
		csiPVC.Namespace = pvcNamespace
		csiPVC.Annotations = map[string]string{
			"volume.beta.kubernetes.io/storage-provisioner": cstorCSIDriver,
		}
		csiPVC.Spec.AccessModes = pvObj.Spec.AccessModes
		csiPVC.Spec.Resources.Requests = pvObj.Spec.Capacity
		csiPVC.Spec.StorageClassName = &pvObj.Spec.StorageClassName
		csiPVC.Spec.VolumeMode = pvObj.Spec.VolumeMode
		csiPVC.Spec.VolumeName = pvObj.Name

		return csiPVC, true, nil
	}
	klog.Infof("pvc already migrated")
	return pvcObj, false, nil
}

func (v *VolumeMigrator) generateCSIPV(
	pvName string,
	pvcObj *corev1.PersistentVolumeClaim,
) (*corev1.PersistentVolume, bool, error) {
	pvObj, err := v.KubeClientset.CoreV1().
		PersistentVolumes().
		Get(pvName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, false, err
		}
		if k8serrors.IsNotFound(err) {
			pvObj, err = v.generateCSIPVFromCV(pvName, pvcObj)
			if err != nil {
				return nil, false, err
			}
			return pvObj, true, nil
		}
	}
	if pvObj.Spec.PersistentVolumeSource.CSI == nil {
		klog.Infof("Generating equivalent CSI PV %s", v.PVName)
		csiPV := &corev1.PersistentVolume{}
		csiPV.Name = pvObj.Name
		csiPV.Annotations = map[string]string{
			"pv.kubernetes.io/provisioned-by": cstorCSIDriver,
		}
		csiPV.Spec.AccessModes = pvObj.Spec.AccessModes
		csiPV.Spec.ClaimRef = &corev1.ObjectReference{
			APIVersion: pvcObj.APIVersion,
			Kind:       pvcObj.Kind,
			Name:       pvcObj.Name,
			Namespace:  pvcObj.Namespace,
		}
		csiPV.Spec.Capacity = pvObj.Spec.Capacity
		csiPV.Spec.PersistentVolumeSource = corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				Driver:       cstorCSIDriver,
				FSType:       pvObj.Spec.PersistentVolumeSource.ISCSI.FSType,
				VolumeHandle: pvObj.Name,
				VolumeAttributes: map[string]string{
					"openebs.io/cas-type":                          "cstor",
					"storage.kubernetes.io/csiProvisionerIdentity": csiProvisionerIdentity,
				},
			},
		}
		csiPV.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete
		if v.StorageClass.ReclaimPolicy != nil {
			csiPV.Spec.PersistentVolumeReclaimPolicy = *v.StorageClass.ReclaimPolicy
		}
		csiPV.Spec.StorageClassName = pvObj.Spec.StorageClassName
		csiPV.Spec.VolumeMode = pvObj.Spec.VolumeMode
		return csiPV, true, nil
	}
	klog.Infof("PV %s already in csi form", pvObj.Name)
	return pvObj, false, nil
}

func (v *VolumeMigrator) generateCSIPVFromCV(
	cvName string,
	pvcObj *corev1.PersistentVolumeClaim,
) (*corev1.PersistentVolume, error) {
	klog.Infof("Generating equivalent CSI PV %s", v.PVName)
	cvObj, err := cv.NewKubeclient().WithNamespace(v.CVNamespace).
		Get(cvName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	csiPV := &corev1.PersistentVolume{}
	csiPV.Name = cvObj.Name
	csiPV.Spec.AccessModes = pvcObj.Spec.AccessModes
	csiPV.Spec.ClaimRef = &corev1.ObjectReference{
		APIVersion: pvcObj.APIVersion,
		Kind:       pvcObj.Kind,
		Name:       pvcObj.Name,
		Namespace:  pvcObj.Namespace,
	}
	csiPV.Spec.Capacity = corev1.ResourceList{
		corev1.ResourceStorage: cvObj.Spec.Capacity,
	}
	csiPV.Spec.PersistentVolumeSource = corev1.PersistentVolumeSource{
		CSI: &corev1.CSIPersistentVolumeSource{
			Driver:       cstorCSIDriver,
			FSType:       cvObj.Annotations["openebs.io/fs-type"],
			VolumeHandle: cvObj.Name,
			VolumeAttributes: map[string]string{
				"openebs.io/cas-type":                          "cstor",
				"storage.kubernetes.io/csiProvisionerIdentity": csiProvisionerIdentity,
			},
		},
	}
	csiPV.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete
	if v.StorageClass.ReclaimPolicy != nil {
		csiPV.Spec.PersistentVolumeReclaimPolicy = *v.StorageClass.ReclaimPolicy
	}
	csiPV.Spec.StorageClassName = v.StorageClass.Name
	csiPV.Spec.VolumeMode = pvcObj.Spec.VolumeMode
	return csiPV, nil
}

func (v *VolumeMigrator) createCVC(pvName string) error {
	var (
		err    error
		cvcObj *cstor.CStorVolumeConfig
		cvObj  *apis.CStorVolume
	)
	cvcObj, err = v.OpenebsClientset.CstorV1().CStorVolumeConfigs(v.OpenebsNamespace).
		Get(pvName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if k8serrors.IsNotFound(err) {
		cvObj, err = cv.NewKubeclient().WithNamespace(v.CVNamespace).
			Get(pvName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		cvrList, err := cvr.NewKubeclient().WithNamespace(v.OpenebsNamespace).
			List(metav1.ListOptions{
				LabelSelector: "",
			})
		if err != nil {
			return err
		}
		if len(cvrList.Items) == 0 {
			return errors.Errorf("failed to get cvrs for volume %s", v.PVName)
		}
		annotations := map[string]string{
			"openebs.io/volumeID":      pvName,
			"openebs.io/volume-policy": pvName,
		}
		labels := map[string]string{
			"openebs.io/cstor-pool-cluster": v.StorageClass.Parameters["cstorPoolCluster"],
		}
		if len(cvObj.Labels["openebs.io/source-volume"]) != 0 {
			labels["openebs.io/source-volume"] = cvObj.Labels["openebs.io/source-volume"]
		}
		finalizer := "cvc.openebs.io/finalizer"
		cvcObj = cstor.NewCStorVolumeConfig().
			WithName(cvObj.Name).
			WithNamespace(v.OpenebsNamespace).
			WithAnnotations(annotations).
			WithLabelsNew(labels).
			WithFinalizer(finalizer)
		cvcObj.Spec.Capacity = corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceStorage): cvObj.Spec.Capacity,
		}
		cvcObj.Spec.Provision.Capacity = corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(cvrList.Items[0].Spec.Capacity),
		}
		cvcObj.Spec.Provision.ReplicaCount = cvObj.Spec.ReplicationFactor
		cvcObj.Status.Phase = cstor.CStorVolumeConfigPhasePending
		cvcObj.VersionDetails = cstor.VersionDetails{
			Desired:     version.Current(),
			AutoUpgrade: false,
			Status: cstor.VersionStatus{
				Current:            version.Current(),
				DependentsUpgraded: true,
			},
		}
		if len(cvObj.Labels["openebs.io/source-volume"]) != 0 {
			cvcObj.Spec.CStorVolumeSource = cvObj.Labels["openebs.io/source-volume"] + "@" + cvObj.Annotations["openebs.io/snapshot"]
		}
		_, err = v.OpenebsClientset.CstorV1().CStorVolumeConfigs(v.OpenebsNamespace).
			Create(cvcObj)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VolumeMigrator) patchTargetSVCOwnerRef() error {
	svcObj, err := v.KubeClientset.CoreV1().
		Services(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cvcObj, err := v.OpenebsClientset.CstorV1().
		CStorVolumeConfigs(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	newSVCObj := svcObj.DeepCopy()
	newSVCObj.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(cvcObj,
			cstor.SchemeGroupVersion.WithKind(cvcKind)),
	}
	data, err := GetPatchData(svcObj, newSVCObj)
	if err != nil {
		return err
	}
	_, err = v.KubeClientset.CoreV1().
		Services(v.OpenebsNamespace).
		Patch(v.PVName, k8stypes.StrategicMergePatchType, data)
	return err
}

// updateStorageClass recreates a new storageclass with the csi provisioner
// the older annotations with the casconfig are also preserved for information
// as the information about the storageclass cannot be gathered from other
// resources a temporary storageclass is created before deleting the original
func (v *VolumeMigrator) updateStorageClass(pvName, scName string) error {
	var tmpSCObj *storagev1.StorageClass
	klog.Infof("Updating storageclass %s with csi parameters", scName)
	scObj, err := v.KubeClientset.StorageV1().
		StorageClasses().
		Get(scName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	}
	if scObj == nil || scObj.Provisioner != cstorCSIDriver {
		tmpSCObj, err = v.createTmpSC(scName)
		if err != nil {
			return err
		}
		replicaCount, err := v.getReplicaCount(pvName)
		if err != nil {
			return err
		}
		cspcName, err := v.getCSPCName(pvName)
		if err != nil {
			return err
		}
		csiSC := tmpSCObj.DeepCopy()
		csiSC.ObjectMeta = metav1.ObjectMeta{
			Name:        scName,
			Annotations: tmpSCObj.Annotations,
		}
		csiSC.Provisioner = cstorCSIDriver
		csiSC.AllowVolumeExpansion = &trueBool
		csiSC.Parameters = map[string]string{
			"cas-type":         "cstor",
			"replicaCount":     replicaCount,
			"cstorPoolCluster": cspcName,
		}
		if scObj != nil {
			err = v.KubeClientset.StorageV1().
				StorageClasses().Delete(scObj.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
		scObj, err = v.KubeClientset.StorageV1().
			StorageClasses().Create(csiSC)
		if err != nil {
			return err
		}
		v.StorageClass = scObj

	}
	err = v.KubeClientset.StorageV1().
		StorageClasses().Delete("tmp-migrate-"+scObj.Name, &metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to delete temporary storageclass")
	}
	return nil
}

func (v *VolumeMigrator) createTmpSC(scName string) (*storagev1.StorageClass, error) {
	tmpSCName := "tmp-migrate-" + scName
	tmpSCObj, err := v.KubeClientset.StorageV1().
		StorageClasses().Get(tmpSCName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		scObj, err := v.KubeClientset.StorageV1().
			StorageClasses().Get(scName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		tmpSCObj = scObj.DeepCopy()
		tmpSCObj.ObjectMeta = metav1.ObjectMeta{
			Name:        tmpSCName,
			Annotations: scObj.Annotations,
		}
		tmpSCObj, err = v.KubeClientset.StorageV1().
			StorageClasses().Create(tmpSCObj)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create temporary storageclass")
		}
	}
	return tmpSCObj, nil
}

func (v *VolumeMigrator) getReplicaCount(pvName string) (string, error) {
	cvObj, err := cv.NewKubeclient().WithNamespace(v.CVNamespace).
		Get(pvName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return strconv.Itoa(cvObj.Spec.ReplicationFactor), nil
}

// the cv can be in the pvc namespace or openebs namespace
func (v *VolumeMigrator) populateCVNamespace(cvName string) error {
	v.CVNamespace = v.OpenebsNamespace
	cvList, err := cv.NewKubeclient().WithNamespace("").
		List(metav1.ListOptions{
			LabelSelector: "openebs.io/persistent-volume=" + v.PVName,
		})
	if err != nil {
		return err
	}
	if len(cvList.Items) != 1 {
		return errors.Errorf("expected exactly 1 cv for %s, got %d", v.PVName, len(cvList.Items))
	}
	for _, cvObj := range cvList.Items {
		if cvObj.Name == cvName {
			v.CVNamespace = cvObj.Namespace
			return nil
		}
	}
	return errors.Errorf("cv %s not found for given pv", cvName)
}

func (v *VolumeMigrator) getCSPCName(pvName string) (string, error) {
	cvrList, err := cvr.NewKubeclient().WithNamespace(v.OpenebsNamespace).
		List(metav1.ListOptions{
			LabelSelector: "openebs.io/persistent-volume=" + pvName,
		})
	if err != nil {
		return "", err
	}
	if len(cvrList.Items) == 0 {
		return "", errors.Errorf("no cvr found for pv %s", pvName)
	}
	cspiName := cvrList.Items[0].Labels["cstorpoolinstance.openebs.io/name"]
	if cspiName == "" {
		return "", errors.Errorf("no cspi label found on cvr %s", cvrList.Items[0].Name)
	}
	lastIndex := strings.LastIndex(cspiName, "-")
	return cspiName[:lastIndex], nil
}

// validatePVName checks whether there exist any pvc for given pv name
// this is required in case the pv gets deleted and only pvc is left
func (v *VolumeMigrator) validatePVName() (*corev1.PersistentVolumeClaim, bool, error) {
	var pvcObj *corev1.PersistentVolumeClaim
	_, err := v.KubeClientset.CoreV1().
		PersistentVolumes().
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, false, err
		}
		pvcList, err := v.KubeClientset.CoreV1().
			PersistentVolumeClaims("").
			List(metav1.ListOptions{})
		if err != nil {
			return pvcObj, false, err
		}
		for _, pvcItem := range pvcList.Items {
			pvcItem := pvcItem // pin it
			if pvcItem.Spec.VolumeName == v.PVName {
				pvcObj = &pvcItem
				return pvcObj, false, nil
			}
		}
		return pvcObj, false, errors.Errorf("No PVC found for the given PV %s", v.PVName)
	}
	return pvcObj, true, nil
}

func (v *VolumeMigrator) removeOldTarget() error {
	_, err := v.OpenebsClientset.CstorV1().
		CStorVolumeConfigs(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if k8serrors.IsNotFound(err) {
		err = v.KubeClientset.AppsV1().
			Deployments(v.CVNamespace).
			Delete(v.PVName+"-target", &metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	// if cv namespace and openebs namespace is not same
	// migrate the target service to openebs namespace
	if v.CVNamespace != v.OpenebsNamespace {
		err = v.migrateTargetSVC()
	}
	return nil
}

func (v *VolumeMigrator) migrateTargetSVC() error {
	svcObj, err := v.KubeClientset.CoreV1().
		Services(v.CVNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if err == nil {
		err = v.KubeClientset.CoreV1().
			Services(v.CVNamespace).
			Delete(svcObj.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	// get the target service in openebs namespace
	_, err = v.KubeClientset.CoreV1().
		Services(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	// if the service is not found in openebs namespace create it
	if k8serrors.IsNotFound(err) {
		svcObj, err := v.getTargetSVC()
		if err != nil {
			return err
		}
		klog.Infof("creating target service %s in %s namespace", svcObj.Name, v.OpenebsNamespace)
		svcObj, err = v.KubeClientset.CoreV1().Services(v.OpenebsNamespace).Create(svcObj)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VolumeMigrator) getTargetSVC() (*corev1.Service, error) {
	cvObj, err := cv.NewKubeclient().WithNamespace(v.CVNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	svcObj := &corev1.Service{}
	svcObj.ObjectMeta = metav1.ObjectMeta{
		Name:      v.PVName,
		Namespace: v.OpenebsNamespace,
		Labels: map[string]string{
			"openebs.io/storage-engine-type": "cstor",
			"openebs.io/cas-type":            "cstor",
			"openebs.io/target-service":      "cstor-target-svc",
			"openebs.io/persistent-volume":   v.PVName,
			"openebs.io/version":             version.Current(),
		},
	}
	svcObj.Spec = corev1.ServiceSpec{
		ClusterIP: cvObj.Spec.TargetIP,
		Ports: []corev1.ServicePort{
			{
				Name:       "cstor-iscsi",
				Port:       3260,
				Protocol:   "TCP",
				TargetPort: intstr.FromInt(3260),
			},
			{
				Name:       "cstor-grpc",
				Port:       7777,
				Protocol:   "TCP",
				TargetPort: intstr.FromInt(7777),
			},
			{
				Name:       "mgmt",
				Port:       6060,
				Protocol:   "TCP",
				TargetPort: intstr.FromInt(6060),
			},
			{
				Name:       "exporter",
				Port:       9500,
				Protocol:   "TCP",
				TargetPort: intstr.FromInt(9500),
			},
		},
		Selector: map[string]string{
			"app":                          "cstor-volume-manager",
			"openebs.io/target":            "cstor-target",
			"openebs.io/persistent-volume": v.PVName,
		},
	}
	return svcObj, nil
}

func (v *VolumeMigrator) createTempPolicy() error {
	klog.Infof("Checking for a temporary policy of volume %s", v.PVName)
	_, err := v.OpenebsClientset.CstorV1().
		CStorVolumePolicies(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !k8serrors.IsNotFound(err) {
		return err
	}
	klog.Infof("Creating temporary policy %s for migration", v.PVName)
	targetDeploy, err := v.KubeClientset.AppsV1().
		Deployments(v.CVNamespace).
		Get(v.PVName+"-target", metav1.GetOptions{})
	if err != nil {
		return err
	}
	cvObj, err := cv.NewKubeclient().WithNamespace(v.CVNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	replicas, err := cvr.NewKubeclient().
		WithNamespace(v.OpenebsNamespace).
		List(metav1.ListOptions{
			LabelSelector: "openebs.io/persistent-volume=" + v.PVName,
		})
	if err != nil {
		return err
	}
	tempPolicy := &cstor.CStorVolumePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v.PVName,
			Namespace: v.OpenebsNamespace,
		},
		Spec: cstor.CStorVolumePolicySpec{
			Target: cstor.TargetSpec{
				PriorityClassName: targetDeploy.Spec.Template.Spec.PriorityClassName,
				Tolerations:       targetDeploy.Spec.Template.Spec.Tolerations,
			},
		},
	}
	if targetDeploy.Spec.Template.Spec.Affinity != nil {
		tempPolicy.Spec.Target.PodAffinity = targetDeploy.Spec.Template.Spec.Affinity.PodAffinity
	}
	if targetDeploy.Spec.Template.Spec.NodeSelector != nil {
		tempPolicy.Spec.Target.NodeSelector = targetDeploy.Spec.Template.Spec.NodeSelector
	}
	if len(replicas.Items) != cvObj.Spec.ReplicationFactor {
		return errors.Errorf("failed to get cvrs for volume %s, expected %d got %d",
			v.PVName, cvObj.Spec.ReplicationFactor, len(replicas.Items),
		)
	}
	for _, replica := range replicas.Items {
		tempPolicy.Spec.ReplicaPoolInfo = append(tempPolicy.Spec.ReplicaPoolInfo,
			cstor.ReplicaPoolInfo{
				PoolName: replica.Labels[cspiNameLabel],
			},
		)
	}
	_, err = v.OpenebsClientset.CstorV1().
		CStorVolumePolicies(v.OpenebsNamespace).
		Create(tempPolicy)
	return err
}

func (v *VolumeMigrator) validateMigratedVolume() error {
	klog.Info("Validating the migrated volume")
retry:
	cvcObj, err := v.OpenebsClientset.CstorV1().
		CStorVolumeConfigs(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if cvcObj.Status.Phase != cstor.CStorVolumeConfigPhaseBound {
		klog.Infof("Waiting for cvc %s to become Bound, got: %s", v.PVName, cvcObj.Status.Phase)
		time.Sleep(10 * time.Second)
		goto retry
	}
	policy, err := v.OpenebsClientset.CstorV1().
		CStorVolumePolicies(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cvrList, err := v.OpenebsClientset.CstorV1().
		CStorVolumeReplicas(v.OpenebsNamespace).
		List(metav1.ListOptions{
			LabelSelector: "openebs.io/persistent-volume=" + v.PVName,
		})
	if err != nil {
		return err
	}
	for _, info := range policy.Spec.ReplicaPoolInfo {
		found := false
		cvrPools := []string{}
		for _, replica := range cvrList.Items {
			if info.PoolName == replica.Labels[types.CStorPoolInstanceNameLabelKey] {
				found = true
			}
			if len(cvrPools) != len(cvrList.Items) {
				cvrPools = append(cvrPools, replica.Labels[types.CStorPoolInstanceLabelKey])
			}
		}
		if !found {
			return errors.Errorf("cvr expected to be scheduled %s pool, but cvrs scheduled on pools %v",
				info.PoolName,
				cvrPools,
			)
		}
	}
	for {
		cvObj, err1 := v.OpenebsClientset.CstorV1().
			CStorVolumes(v.OpenebsNamespace).
			Get(v.PVName, metav1.GetOptions{})
		if err1 != nil {
			klog.Errorf("failed to get cv %s: %s", v.PVName, err1.Error())
		} else {
			if cvObj.Status.Phase == cstor.CStorVolumePhase("Healthy") {
				break
			}
			klog.Infof("Waiting for cv %s to come to Healthy state, got: %s",
				v.PVName, cvObj.Status.Phase)
		}
		time.Sleep(10 * time.Second)
	}
	klog.Info("Patching the target svc with cvc owner ref")
	err = v.patchTargetSVCOwnerRef()
	if err != nil {
		errors.Wrap(err, "failed to patch cvc owner ref to target svc")
	}
	return nil
}

func (v *VolumeMigrator) patchTargetPodAffinity() error {
	cvp, err := v.OpenebsClientset.CstorV1().
		CStorVolumePolicies(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if cvp.Spec.Target.PodAffinity == nil {
		return nil
	}
	klog.Info("Patching target pod with old pod affinity rules")
	targetDeploy, err := v.KubeClientset.AppsV1().
		Deployments(v.OpenebsNamespace).
		Get(v.PVName+"-target", metav1.GetOptions{})
	if err != nil {
		return err
	}
	newTargetDeploy := targetDeploy.DeepCopy()
	if newTargetDeploy.Spec.Template.Spec.Affinity == nil {
		newTargetDeploy.Spec.Template.Spec.Affinity = &corev1.Affinity{}
	}
	newTargetDeploy.Spec.Template.Spec.Affinity.PodAffinity = cvp.Spec.Target.PodAffinity
	data, err := GetPatchData(targetDeploy, newTargetDeploy)
	if err != nil {
		return err
	}
	_, err = v.KubeClientset.AppsV1().
		Deployments(v.OpenebsNamespace).
		Patch(v.PVName+"-target", k8stypes.StrategicMergePatchType, data)
	return err
}

func (v *VolumeMigrator) cleanupOldResources() error {
	klog.Info("Cleaning up old volume resources")
	cvrList, err := cvr.NewKubeclient().
		WithNamespace(v.OpenebsNamespace).
		List(metav1.ListOptions{
			LabelSelector: "openebs.io/persistent-volume=" + v.PVName,
		})
	if err != nil {
		return errors.Wrapf(err, "failed too list cvrs for %s", v.PVName)
	}
	for _, replica := range cvrList.Items {
		rep := replica // pin it
		rep.Finalizers = []string{}
		_, err = cvr.NewKubeclient().
			WithNamespace(v.OpenebsNamespace).
			Update(&rep)
		if err != nil {
			return errors.Wrapf(err, "failed to remove finalizer from cvr %s", rep.Name)
		}
		err = cvr.NewKubeclient().
			WithNamespace(v.OpenebsNamespace).
			Delete(replica.Name)
		if err != nil {
			return err
		}
	}
	cvcObj, err := v.OpenebsClientset.CstorV1().
		CStorVolumeConfigs(v.OpenebsNamespace).
		Get(v.PVName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	newCVCObj := cvcObj.DeepCopy()
	delete(newCVCObj.Annotations, "openebs.io/volume-policy")
	data, err := GetPatchData(cvcObj, newCVCObj)
	if err != nil {
		return err
	}
	_, err = v.OpenebsClientset.CstorV1().
		CStorVolumeConfigs(v.OpenebsNamespace).
		Patch(v.PVName, k8stypes.MergePatchType, data)
	if err != nil {
		return err
	}
	err = cv.NewKubeclient().
		WithNamespace(v.CVNamespace).
		Delete(v.PVName)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

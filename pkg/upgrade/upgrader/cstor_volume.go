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
	"context"
	"time"

	cstor "github.com/openebs/api/v3/pkg/apis/cstor/v1"
	v1Alpha1API "github.com/openebs/api/v3/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/openebs/upgrade/pkg/upgrade/patch"
)

// CStorVolumePatch is the patch required to upgrade CStorVolume
type CStorVolumePatch struct {
	*ResourcePatch
	Namespace string
	CVC       *patch.CVC
	CV        *patch.CV
	Deploy    *patch.Deployment
	Service   *patch.Service
	Utask     *v1Alpha1API.UpgradeTask
	*Client
}

// CStorVolumePatchOptions ...
type CStorVolumePatchOptions func(*CStorVolumePatch)

// WithCStorVolumeResorcePatch ...
func WithCStorVolumeResorcePatch(r *ResourcePatch) CStorVolumePatchOptions {
	return func(obj *CStorVolumePatch) {
		obj.ResourcePatch = r
	}
}

// WithCStorVolumeClient ...
func WithCStorVolumeClient(c *Client) CStorVolumePatchOptions {
	return func(obj *CStorVolumePatch) {
		obj.Client = c
	}
}

// NewCStorVolumePatch ...
func NewCStorVolumePatch(opts ...CStorVolumePatchOptions) *CStorVolumePatch {
	obj := &CStorVolumePatch{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// PreUpgrade ...
func (obj *CStorVolumePatch) PreUpgrade() (string, error) {
	err := isOperatorUpgraded("cvc-operator", obj.Namespace, obj.To, obj.KubeClientset)
	if err != nil {
		return "failed to verify cvc-operator", err
	}
	err = obj.CVC.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify CVC", err
	}
	err = obj.CV.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify CV", err
	}
	err = obj.Deploy.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify target deploy", err
	}
	err = obj.Service.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify target svc", err
	}
	return "", nil
}

// Init initializes all the fields of the CStorVolumePatch
func (obj *CStorVolumePatch) Init() (string, error) {
	label := "openebs.io/persistent-volume=" + obj.Name
	obj.Namespace = obj.OpenebsNamespace
	obj.CVC = patch.NewCVC(
		patch.WithCVCClient(obj.OpenebsClientset),
	)
	err := obj.CVC.Get(obj.Name, obj.Namespace)
	if err != nil {
		return "failed to get CVC for volume" + obj.Name, err
	}
	obj.CV = patch.NewCV(
		patch.WithCVClient(obj.OpenebsClientset),
	)
	err = obj.CV.Get(obj.Name, obj.Namespace)
	if err != nil {
		return "failed to get CV for volume" + obj.Name, err
	}
	obj.Deploy = patch.NewDeployment(
		patch.WithDeploymentClient(obj.KubeClientset),
	)
	err = obj.Deploy.Get(label, obj.Namespace)
	if err != nil {
		return "failed to get target deploy for volume" + obj.Name, err
	}
	obj.Service = patch.NewService(
		patch.WithKubeClient(obj.KubeClientset),
	)
	err = obj.Service.Get(label, obj.Namespace)
	if err != nil {
		return "failed to get target svc for volume" + obj.Name, err
	}
	return "", nil
}

func (obj *CStorVolumePatch) GetVolumePatches() (string, error) {
	err := obj.getCVCPatchData()
	if err != nil {
		return "failed to create CVC patch for volume" + obj.Name, err
	}
	err = obj.getCVPatchData()
	if err != nil {
		return "failed to create CV patch for volume" + obj.Name, err
	}
	err = obj.getCVDeployPatchData()
	if err != nil {
		return "failed to create target deploy patch for volume" + obj.Name, err
	}
	err = getCVServicePatchData(obj)
	if err != nil {
		return "failed to create target svc patch for volume" + obj.Name, err
	}
	return "", nil
}

func (obj *CStorVolumePatch) getCVCPatchData() error {
	newCVC := obj.CVC.Object.DeepCopy()
	err := obj.transformCVC(newCVC, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.CVC.Data, err = GetPatchData(obj.CVC.Object, newCVC)
	return err
}

func (obj *CStorVolumePatch) transformCVC(c *cstor.CStorVolumeConfig, res *ResourcePatch) error {
	pvObj, err := obj.KubeClientset.CoreV1().PersistentVolumes().
		Get(context.TODO(), obj.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	c.Annotations["openebs.io/persistent-volume-claim"] = pvObj.Spec.ClaimRef.Name
	c.VersionDetails.Desired = res.To
	return nil
}

func (obj *CStorVolumePatch) getCVPatchData() error {
	newCV := obj.CV.Object.DeepCopy()
	err := obj.transformCV(newCV, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.CV.Data, err = GetPatchData(obj.CV.Object, newCV)
	return err
}

func (obj *CStorVolumePatch) transformCV(c *cstor.CStorVolume, res *ResourcePatch) error {
	pvObj, err := obj.KubeClientset.CoreV1().PersistentVolumes().
		Get(context.TODO(), obj.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	c.Labels["openebs.io/persistent-volume-claim"] = pvObj.Spec.ClaimRef.Name
	c.VersionDetails.Desired = res.To
	return nil
}

func (obj *CStorVolumePatch) getCVDeployPatchData() error {
	newDeploy := obj.Deploy.Object.DeepCopy()
	err := obj.transformCVDeploy(newDeploy, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.Deploy.Data, err = GetPatchData(obj.Deploy.Object, newDeploy)
	return err
}

func (obj *CStorVolumePatch) transformCVDeploy(d *appsv1.Deployment, res *ResourcePatch) error {
	// update deployment images
	tag := res.To
	if res.ImageTag != "" {
		tag = res.ImageTag
	}
	cons := len(d.Spec.Template.Spec.Containers)
	for i := 0; i < cons; i++ {
		url, err := getImageURL(
			d.Spec.Template.Spec.Containers[i].Image,
			res.BaseURL,
		)
		if err != nil {
			return err
		}
		// remove the -amd64 prefix from the image
		url = removeSuffixFromEnd(url, "-amd64")
		d.Spec.Template.Spec.Containers[i].Image = url + ":" + tag
	}
	pvObj, err := obj.KubeClientset.CoreV1().PersistentVolumes().
		Get(context.TODO(), obj.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	d.Labels["openebs.io/persistent-volume-claim"] = pvObj.Spec.ClaimRef.Name
	d.Spec.Template.Labels["openebs.io/persistent-volume-claim"] = pvObj.Spec.ClaimRef.Name
	d.Labels["openebs.io/version"] = res.To
	d.Spec.Template.Labels["openebs.io/version"] = res.To
	d.Spec.Template.Spec.ServiceAccountName = cstorOperatorServiceAccount
	return nil
}

func getCVServicePatchData(obj *CStorVolumePatch) error {
	newSVC := obj.Service.Object.DeepCopy()
	err := transformCVService(newSVC, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.Service.Data, err = GetPatchData(obj.Service.Object, newSVC)
	return err
}

func transformCVService(svc *corev1.Service, res *ResourcePatch) error {
	svc.Labels["openebs.io/version"] = res.To
	return nil
}

// CStorVolumeUpgrade ...
func (obj *CStorVolumePatch) CStorVolumeUpgrade() (string, error) {
	err := obj.Deploy.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to patch target deploy", err
	}
	err = obj.Service.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to patch target svc", err
	}
	err = obj.CV.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to patch CV", err
	}
	err = obj.verifyCVVersionReconcile()
	if err != nil {
		return "failed to verify version reconcile on CV", err
	}
	err = obj.CVC.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to patch CVC", err
	}
	err = obj.verifyCVCVersionReconcile()
	if err != nil {
		return "failed to verify version reconcile on CVC", err
	}
	return "", nil
}

// Upgrade execute the steps to upgrade CStorVolume
func (obj *CStorVolumePatch) Upgrade() error {
	var err, uerr error
	obj.Utask, err = getOrCreateUpgradeTask(
		"cstorVolume",
		obj.ResourcePatch,
		obj.Client,
	)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}
	statusObj := v1Alpha1API.UpgradeDetailedStatuses{Step: v1Alpha1API.PreUpgrade}
	statusObj.Phase = v1Alpha1API.StepWaiting
	obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}
	statusObj.Phase = v1Alpha1API.StepErrored
	msg, err := obj.Init()
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	msg, err = obj.PreUpgrade()
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	msg, err = obj.GetVolumePatches()
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	statusObj.Phase = v1Alpha1API.StepCompleted
	statusObj.Message = "Pre-upgrade steps were successful"
	statusObj.Reason = ""
	obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}

	statusObj = v1Alpha1API.UpgradeDetailedStatuses{Step: v1Alpha1API.ReplicaUpgrade}
	statusObj.Phase = v1Alpha1API.StepWaiting
	obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}
	statusObj.Phase = v1Alpha1API.StepErrored
	res := *obj.ResourcePatch
	cvrList, err := obj.Client.OpenebsClientset.CstorV1().
		CStorVolumeReplicas(obj.Namespace).List(context.TODO(),
		metav1.ListOptions{
			LabelSelector: "openebs.io/persistent-volume=" + obj.Name,
		},
	)
	if err != nil && isUpgradeTaskJob {
		msg = "failed to list cvrs for volume"
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	for _, cvrObj := range cvrList.Items {
		res.Name = cvrObj.Name
		dependant := NewCVRPatch(
			WithCVRResorcePatch(&res),
			WithCVRClient(obj.Client),
		)
		err = dependant.Upgrade()
		if err != nil {
			msg = "failed to patch cvr " + cvrObj.Name
			statusObj.Message = msg
			statusObj.Reason = err.Error()
			obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
			if uerr != nil && isUpgradeTaskJob {
				return uerr
			}
			return errors.Wrap(err, msg)
		}
	}
	statusObj.Phase = v1Alpha1API.StepCompleted
	statusObj.Message = "Replica upgrade was successful"
	statusObj.Reason = ""
	obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}
	statusObj = v1Alpha1API.UpgradeDetailedStatuses{Step: v1Alpha1API.TargetUpgrade}
	statusObj.Phase = v1Alpha1API.StepWaiting
	obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}
	statusObj.Phase = v1Alpha1API.StepErrored
	msg, err = obj.CStorVolumeUpgrade()
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	statusObj.Phase = v1Alpha1API.StepCompleted
	statusObj.Message = "Target upgrade was successful"
	statusObj.Reason = ""
	obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}
	return nil
}

func (obj *CStorVolumePatch) verifyCVVersionReconcile() error {
	// get the latest cvc object
	err := obj.CV.Get(obj.Name, obj.Namespace)
	if err != nil {
		return err
	}
	// waiting for the current version to be equal to desired version
	for obj.CV.Object.VersionDetails.Status.Current != obj.To {
		klog.Infof("Verifying the reconciliation of version for %s", obj.CV.Object.Name)
		// Sleep equal to the default sync time
		time.Sleep(10 * time.Second)
		err = obj.CV.Get(obj.Name, obj.Namespace)
		if err != nil {
			return err
		}
		if obj.CV.Object.VersionDetails.Status.Message != "" {
			klog.Errorf("failed to reconcile: %s", obj.CV.Object.VersionDetails.Status.Reason)
		}
	}
	return nil
}

func (obj *CStorVolumePatch) verifyCVCVersionReconcile() error {
	// get the latest cvc object
	err := obj.CVC.Get(obj.Name, obj.Namespace)
	if err != nil {
		return err
	}
	// waiting for the current version to be equal to desired version
	for obj.CVC.Object.VersionDetails.Status.Current != obj.To {
		klog.Infof("Verifying the reconciliation of version for %s", obj.CVC.Object.Name)
		// Sleep equal to the default sync time
		time.Sleep(10 * time.Second)
		err = obj.CVC.Get(obj.Name, obj.Namespace)
		if err != nil {
			return err
		}
		if obj.CVC.Object.VersionDetails.Status.Message != "" {
			klog.Errorf("failed to reconcile: %s", obj.CVC.Object.VersionDetails.Status.Reason)
		}
	}
	return nil
}

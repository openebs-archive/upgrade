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
	"time"

	apis "github.com/openebs/api/pkg/apis/cstor/v1"
	"github.com/openebs/upgrade/pkg/upgrade/patch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

// CStorVolumePatch is the patch required to upgrade CStorVolume
type CStorVolumePatch struct {
	*ResourcePatch
	Namespace string
	CVC       *patch.CVC
	CV        *patch.CV
	Deploy    *patch.Deployment
	Service   *patch.Service
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
func (obj *CStorVolumePatch) PreUpgrade() error {
	err := isOperatorUpgraded("cvc-operator", obj.Namespace, obj.To, obj.KubeClientset)
	if err != nil {
		return err
	}
	err = obj.CVC.PreChecks(obj.From, obj.To)
	if err != nil {
		return err
	}
	err = obj.CV.PreChecks(obj.From, obj.To)
	if err != nil {
		return err
	}
	err = obj.Deploy.PreChecks(obj.From, obj.To)
	if err != nil {
		return err
	}
	err = obj.Service.PreChecks(obj.From, obj.To)
	return err
}

// Init initializes all the fields of the CStorVolumePatch
func (obj *CStorVolumePatch) Init() error {
	label := "openebs.io/persistent-volume=" + obj.Name
	obj.Namespace = obj.OpenebsNamespace
	obj.CVC = patch.NewCVC(
		patch.WithCVCClient(obj.OpenebsClientset),
	)
	err := obj.CVC.Get(obj.Name, obj.Namespace)
	if err != nil {
		return err
	}
	obj.CV = patch.NewCV(
		patch.WithCVClient(obj.OpenebsClientset),
	)
	err = obj.CV.Get(obj.Name, obj.Namespace)
	if err != nil {
		return err
	}
	obj.Deploy = patch.NewDeployment(
		patch.WithDeploymentClient(obj.KubeClientset),
	)
	err = obj.Deploy.Get(label, obj.Namespace)
	if err != nil {
		return err
	}
	obj.Service = patch.NewService(
		patch.WithKubeClient(obj.KubeClientset),
	)
	err = obj.Service.Get(label, obj.Namespace)
	if err != nil {
		return err
	}
	err = getCVCPatchData(obj)
	if err != nil {
		return err
	}
	err = getCVPatchData(obj)
	if err != nil {
		return err
	}
	err = getCVDeployPatchData(obj)
	if err != nil {
		return err
	}
	err = getCVServicePatchData(obj)
	return err
}

func getCVCPatchData(obj *CStorVolumePatch) error {
	newCVC := obj.CVC.Object.DeepCopy()
	err := transformCVC(newCVC, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.CVC.Data, err = GetPatchData(obj.CVC.Object, newCVC)
	return err
}

func transformCVC(c *apis.CStorVolumeConfig, res *ResourcePatch) error {
	c.VersionDetails.Desired = res.To
	return nil
}

func getCVPatchData(obj *CStorVolumePatch) error {
	newCV := obj.CV.Object.DeepCopy()
	err := transformCV(newCV, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.CV.Data, err = GetPatchData(obj.CV.Object, newCV)
	return err
}

func transformCV(c *apis.CStorVolume, res *ResourcePatch) error {
	c.VersionDetails.Desired = res.To
	return nil
}

func getCVDeployPatchData(obj *CStorVolumePatch) error {
	newDeploy := obj.Deploy.Object.DeepCopy()
	err := transformCVDeploy(newDeploy, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.Deploy.Data, err = GetPatchData(obj.Deploy.Object, newDeploy)
	return err
}

func transformCVDeploy(d *appsv1.Deployment, res *ResourcePatch) error {
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
		d.Spec.Template.Spec.Containers[i].Image = url + ":" + tag
	}
	d.Labels["openebs.io/version"] = res.To
	d.Spec.Template.Labels["openebs.io/version"] = res.To
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
func (obj *CStorVolumePatch) CStorVolumeUpgrade() error {
	err := obj.Deploy.Patch(obj.From, obj.To)
	if err != nil {
		return err
	}
	err = obj.Service.Patch(obj.From, obj.To)
	if err != nil {
		return err
	}
	err = obj.CV.Patch(obj.From, obj.To)
	if err != nil {
		return err
	}
	err = obj.verifyCVVersionReconcile()
	if err != nil {
		return err
	}
	err = obj.CVC.Patch(obj.From, obj.To)
	if err != nil {
		return err
	}
	err = obj.verifyCVCVersionReconcile()
	return err
}

// Upgrade execute the steps to upgrade CStorVolume
func (obj *CStorVolumePatch) Upgrade() error {
	err := obj.Init()
	if err != nil {
		return err
	}
	err = obj.PreUpgrade()
	if err != nil {
		return err
	}
	res := *obj.ResourcePatch
	cvrList, err := obj.Client.OpenebsClientset.CstorV1().
		CStorVolumeReplicas(obj.Namespace).List(
		metav1.ListOptions{
			LabelSelector: "openebs.io/persistent-volume=" + obj.Name,
		},
	)
	if err != nil {
		return err
	}
	for _, cvrObj := range cvrList.Items {
		res.Name = cvrObj.Name
		dependant := NewCVRPatch(
			WithCVRResorcePatch(&res),
			WithCVRClient(obj.Client),
		)
		err = dependant.Upgrade()
		if err != nil {
			return err
		}
	}
	err = obj.CStorVolumeUpgrade()
	return err
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

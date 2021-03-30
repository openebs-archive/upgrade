/*
Copyright 2021 The OpenEBS Authors

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

	v1Alpha1API "github.com/openebs/api/v2/pkg/apis/openebs.io/v1alpha1"
	jv "github.com/openebs/jiva-operator/pkg/apis/openebs/v1alpha1"
	"github.com/openebs/upgrade/pkg/upgrade/patch"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
)

// JivaVolumePatch is the patch required to upgrade JivaVolume
type JivaVolumePatch struct {
	*ResourcePatch
	Namespace    string
	Controller   *patch.Deployment
	Replicas     *patch.StatefulSet
	Service      *patch.Service
	JivaVolumeCR *patch.JV
	Utask        *v1Alpha1API.UpgradeTask
	*Client
}

// JivaVolumePatchOptions ...
type JivaVolumePatchOptions func(*JivaVolumePatch)

// WithJivaVolumeResorcePatch ...
func WithJivaVolumeResorcePatch(r *ResourcePatch) JivaVolumePatchOptions {
	return func(obj *JivaVolumePatch) {
		obj.ResourcePatch = r
	}
}

// WithJivaVolumeClient ...
func WithJivaVolumeClient(c *Client) JivaVolumePatchOptions {
	return func(obj *JivaVolumePatch) {
		obj.Client = c
	}
}

// NewJivaVolumePatch ...
func NewJivaVolumePatch(opts ...JivaVolumePatchOptions) *JivaVolumePatch {
	obj := &JivaVolumePatch{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// PreUpgrade ...
func (obj *JivaVolumePatch) PreUpgrade() (string, error) {
	err := isOperatorUpgraded("jiva-operator", obj.Namespace, obj.To, obj.KubeClientset)
	if err != nil {
		return "failed to verify jiva-operator", err
	}
	err = obj.Controller.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify controller deploy", err
	}
	err = obj.Replicas.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify replica statefulset", err
	}
	err = obj.Service.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify target svc", err
	}
	return "", nil
}

// Init initializes all the fields of the JivaVolumePatch
func (obj *JivaVolumePatch) Init() (string, error) {
	pvLabel := "openebs.io/persistent-volume=" + obj.Name
	replicaLabel := "openebs.io/component=jiva-replica," + pvLabel
	controllerLabel := "openebs.io/component=jiva-controller," + pvLabel
	serviceLabel := "openebs.io/component=jiva-controller-service," + pvLabel
	obj.Namespace = obj.OpenebsNamespace
	obj.Controller = patch.NewDeployment(
		patch.WithDeploymentClient(obj.KubeClientset),
	)
	err := obj.Controller.Get(controllerLabel, obj.Namespace)
	if err != nil {
		return "failed to get controller deployment for volume" + obj.Name, err
	}
	obj.Replicas = patch.NewStatefulSet(
		patch.WithStatefulSetClient(obj.KubeClientset),
	)
	err = obj.Replicas.Get(replicaLabel, obj.Namespace)
	if err != nil {
		return "failed to list replica statefulset for volume" + obj.Name, err
	}
	obj.Service = patch.NewService(
		patch.WithKubeClient(obj.KubeClientset),
	)
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(jv.AddToScheme(scheme))
	clientgoscheme.AddToScheme(scheme)
	cl, err := client.New(config.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return "failed to create runtime client", err
	}
	obj.JivaVolumeCR = patch.NewJV(
		patch.WithJVClient(cl),
	)

	err = obj.Service.Get(serviceLabel, obj.Namespace)
	if err != nil {
		return "failed to get target svc for volume" + obj.Name, err
	}
	err = obj.JivaVolumeCR.Get(obj.Name, obj.Namespace)
	if err != nil {
		return "failed to get jivavolume CR for volume" + obj.Name, err
	}
	err = obj.getJivaControllerPatchData()
	if err != nil {
		return "failed to create target deploy patch for volume" + obj.Name, err
	}
	err = getJivaServicePatchData(obj)
	if err != nil {
		return "failed to create target svc patch for volume" + obj.Name, err
	}
	err = obj.getJivaReplicaPatchData()
	if err != nil {
		return "failed to create replica sts patch for volume" + obj.Name, err
	}
	err = obj.getJVPatchData()
	if err != nil {
		return "failed to create jivavolume patch for volume" + obj.Name, err
	}
	return "", nil
}

func (obj *JivaVolumePatch) getJivaControllerPatchData() error {
	newDeploy := obj.Controller.Object.DeepCopy()
	err := obj.transformJivaController(newDeploy, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.Controller.Data, err = GetPatchData(obj.Controller.Object, newDeploy)
	return err
}

func (obj *JivaVolumePatch) transformJivaController(d *appsv1.Deployment, res *ResourcePatch) error {
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

func (obj *JivaVolumePatch) getJivaReplicaPatchData() error {
	newSTS := obj.Replicas.Object.DeepCopy()
	err := obj.transformJivaReplica(newSTS, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.Replicas.Data, err = GetPatchData(obj.Replicas.Object, newSTS)
	return err
}

func (obj *JivaVolumePatch) transformJivaReplica(s *appsv1.StatefulSet, res *ResourcePatch) error {
	// update deployment images
	tag := res.To
	if res.ImageTag != "" {
		tag = res.ImageTag
	}
	cons := len(s.Spec.Template.Spec.Containers)
	for i := 0; i < cons; i++ {
		url, err := getImageURL(
			s.Spec.Template.Spec.Containers[i].Image,
			res.BaseURL,
		)
		if err != nil {
			return err
		}
		s.Spec.Template.Spec.Containers[i].Image = url + ":" + tag
	}
	s.Labels["openebs.io/version"] = res.To
	s.Spec.Template.Labels["openebs.io/version"] = res.To
	return nil
}

func (obj *JivaVolumePatch) getJVPatchData() error {
	newJV := obj.JivaVolumeCR.Object.DeepCopy()
	err := obj.transformJV(newJV, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.JivaVolumeCR.NewObject = newJV
	return err
}

func (obj *JivaVolumePatch) transformJV(c *jv.JivaVolume, res *ResourcePatch) error {
	c.VersionDetails.Desired = res.To
	return nil
}

func getJivaServicePatchData(obj *JivaVolumePatch) error {
	newSVC := obj.Service.Object.DeepCopy()
	err := transformJivaService(newSVC, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.Service.Data, err = GetPatchData(obj.Service.Object, newSVC)
	return err
}

func transformJivaService(svc *corev1.Service, res *ResourcePatch) error {
	svc.Labels["openebs.io/version"] = res.To
	return nil
}

// JivaVolumeUpgrade ...
func (obj *JivaVolumePatch) JivaVolumeUpgrade() (string, error) {
	err := obj.Controller.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to patch target deploy", err
	}
	err = obj.Service.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to patch target svc", err
	}
	err = obj.JivaVolumeCR.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to patch JivaCR", err
	}
	err = obj.verifyJivaVolumeCRversionReconcile()
	if err != nil {
		return "failed to verify version reconcile on JivaVolumeCR", err
	}
	return "", nil
}

// Upgrade execute the steps to upgrade JivaVolume
func (obj *JivaVolumePatch) Upgrade() error {
	var err, uerr error
	obj.Utask, err = getOrCreateUpgradeTask(
		"jivaVolume",
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

	err = obj.Replicas.Patch(obj.From, obj.To)
	if err != nil {
		statusObj.Message = "failed to patch replica sts"
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
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
	msg, err = obj.JivaVolumeUpgrade()
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

func (obj *JivaVolumePatch) verifyJivaVolumeCRversionReconcile() error {
	// get the latest cvc object
	err := obj.JivaVolumeCR.Get(obj.Name, obj.Namespace)
	if err != nil {
		return err
	}
	// waiting for the current version to be equal to desired version
	for obj.JivaVolumeCR.Object.VersionDetails.Status.Current != obj.To {
		klog.Infof("Verifying the reconciliation of version for %s", obj.JivaVolumeCR.Object.Name)
		// Sleep equal to the default sync time
		time.Sleep(10 * time.Second)
		err = obj.JivaVolumeCR.Get(obj.Name, obj.Namespace)
		if err != nil {
			return err
		}
		if obj.JivaVolumeCR.Object.VersionDetails.Status.Message != "" {
			klog.Errorf("failed to reconcile: %s", obj.JivaVolumeCR.Object.VersionDetails.Status.Reason)
		}
	}
	return nil
}

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

	cstor "github.com/openebs/api/v2/pkg/apis/cstor/v1"
	v1Alpha1API "github.com/openebs/api/v2/pkg/apis/openebs.io/v1alpha1"
	"github.com/openebs/api/v2/pkg/apis/types"
	translate "github.com/openebs/upgrade/pkg/migrate/cstor"
	"github.com/openebs/upgrade/pkg/upgrade/patch"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

// CSPIPatch is the patch required to upgrade cspi
type CSPIPatch struct {
	*ResourcePatch
	Namespace string
	Deploy    *patch.Deployment
	CSPI      *patch.CSPI
	Utask     *v1Alpha1API.UpgradeTask
	*Client
}

// CSPIPatchOptions ...
type CSPIPatchOptions func(*CSPIPatch)

// WithCSPIResorcePatch ...
func WithCSPIResorcePatch(r *ResourcePatch) CSPIPatchOptions {
	return func(obj *CSPIPatch) {
		obj.ResourcePatch = r
	}
}

// WithCSPIClient ...
func WithCSPIClient(c *Client) CSPIPatchOptions {
	return func(obj *CSPIPatch) {
		obj.Client = c
	}
}

// WithCSPIDeploy ...
func WithCSPIDeploy(t *patch.Deployment) CSPIPatchOptions {
	return func(obj *CSPIPatch) {
		obj.Deploy = t
	}
}

// NewCSPIPatch ...
func NewCSPIPatch(opts ...CSPIPatchOptions) *CSPIPatch {
	obj := &CSPIPatch{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// PreUpgrade ...
func (obj *CSPIPatch) PreUpgrade() (string, error) {
	err := obj.Deploy.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify cstor pool deployment", err
	}
	err = obj.CSPI.PreChecks(obj.From, obj.To)
	if err != nil {
		return "failed to verify cstor pool instance", err
	}
	return "", nil
}

// DeployUpgrade ...
func (obj *CSPIPatch) DeployUpgrade() (string, error) {
	err := obj.Deploy.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to patch cstor pool deployment", err
	}
	return "", nil
}

// CSPIUpgrade ...
func (obj *CSPIPatch) CSPIUpgrade() (string, error) {
	err := obj.CSPI.Patch(obj.From, obj.To)
	if err != nil {
		return "failed to verify cstor pool instance", err
	}
	return "", nil
}

// Upgrade execute the steps to upgrade cspi
func (obj *CSPIPatch) Upgrade() error {
	var err, uerr error
	obj.Utask, err = getOrCreateUpgradeTask(
		"cstorPoolInstance",
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

	statusObj = v1Alpha1API.UpgradeDetailedStatuses{Step: v1Alpha1API.PoolInstanceUpgrade}
	statusObj.Phase = v1Alpha1API.StepWaiting
	obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}
	statusObj.Phase = v1Alpha1API.StepErrored
	msg, err = obj.DeployUpgrade()
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	msg, err = obj.CSPIUpgrade()
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	msg, err = obj.verifyCSPIVersionReconcile()
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
		if uerr != nil && isUpgradeTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	msg, err = obj.upgradeBackupRestore()
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
	statusObj.Message = "Pool instance upgrade was successful"
	statusObj.Reason = ""
	obj.Utask, uerr = updateUpgradeDetailedStatus(obj.Utask, statusObj, obj.OpenebsNamespace, obj.Client)
	if uerr != nil && isUpgradeTaskJob {
		return uerr
	}
	return nil
}

// Init initializes all the fields of the CSPIPatch
func (obj *CSPIPatch) Init() (string, error) {
	var err error
	statusObj := v1Alpha1API.UpgradeDetailedStatuses{Step: v1Alpha1API.PreUpgrade}
	statusObj.Phase = v1Alpha1API.StepErrored
	obj.Deploy = patch.NewDeployment(
		patch.WithDeploymentClient(obj.KubeClientset),
	)
	obj.Namespace = obj.OpenebsNamespace
	label := "openebs.io/cstor-pool-instance=" + obj.Name
	err = obj.Deploy.Get(label, obj.Namespace)
	if err != nil {
		return "failed to get cstor pool deployment", err
	}
	obj.CSPI = patch.NewCSPI(
		patch.WithCSPIClient(obj.OpenebsClientset),
	)
	err = obj.CSPI.Get(obj.Name, obj.Namespace)
	if err != nil {
		return "failed to get cstor pool instance", err
	}
	err = getCSPIDeployPatchData(obj)
	if err != nil {
		return "failed to create cstor pool deployment patch", err
	}
	err = getCSPIPatchData(obj)
	if err != nil {
		return "failed to create cstor pool instance patch", err
	}
	return "", nil
}

func getCSPIDeployPatchData(obobj *CSPIPatch) error {
	newDeploy := obobj.Deploy.Object.DeepCopy()
	err := transformCSPIDeploy(newDeploy, obobj.ResourcePatch)
	if err != nil {
		return err
	}
	obobj.Deploy.Data, err = GetPatchData(obobj.Deploy.Object, newDeploy)
	return err
}

func transformCSPIDeploy(d *appsv1.Deployment, res *ResourcePatch) error {
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
	d.Labels["openebs.io/version"] = res.To
	d.Spec.Template.Labels["openebs.io/version"] = res.To
	return nil
}

func getCSPIPatchData(obj *CSPIPatch) error {
	newCSPI := obj.CSPI.Object.DeepCopy()
	err := transformCSPI(newCSPI, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.CSPI.Data, err = GetPatchData(obj.CSPI.Object, newCSPI)
	return err
}

func transformCSPI(c *cstor.CStorPoolInstance, res *ResourcePatch) error {
	c.Labels["openebs.io/version"] = res.To
	c.VersionDetails.Desired = res.To
	return nil
}

func (obj *CSPIPatch) verifyCSPIVersionReconcile() (string, error) {
	// get the latest cspi object
	err := obj.CSPI.Get(obj.Name, obj.Namespace)
	if err != nil {
		return "failed to get cstor pool to verify ", err
	}
	// waiting for the current version to be equal to desired version
	for obj.CSPI.Object.VersionDetails.Status.Current != obj.To {
		klog.Infof("Verifying the reconciliation of version for %s", obj.CSPI.Object.Name)
		// Sleep equal to the default sync time
		time.Sleep(10 * time.Second)
		err = obj.CSPI.Get(obj.Name, obj.Namespace)
		if err != nil {
			return "failed to get cstor pool to verify ", err
		}
		if obj.CSPI.Object.VersionDetails.Status.Message != "" {
			klog.Errorf("failed to reconcile: %s", obj.CSPI.Object.VersionDetails.Status.Reason)
		}
	}
	return "", nil
}

func (obj *CSPIPatch) upgradeBackupRestore() (string, error) {
	// Migrate backup to v1 version
	oldBackupList, err := obj.OpenebsClientset.OpenebsV1alpha1().
		CStorBackups(obj.OpenebsNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: types.CStorPoolInstanceNameLabelKey + "=" + obj.Name,
	})
	if err != nil && !k8serrors.IsNotFound(err) {
		return "failed to list v1alpha1 cstorbackups", err
	}
	for _, oldBackup := range oldBackupList.Items {
		newBackup := translate.TranslateBackupToV1(oldBackup)
		_, err := obj.OpenebsClientset.CstorV1().
			CStorBackups(obj.OpenebsNamespace).Create(context.TODO(),
			newBackup, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return "failed to create v1 cstorbackup for " + oldBackup.Name, err
		}

	}
	if len(oldBackupList.Items) != 0 {
		err = obj.OpenebsClientset.OpenebsV1alpha1().
			CStorBackups(obj.OpenebsNamespace).
			DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: types.CStorPoolInstanceNameLabelKey + "=" + obj.Name,
			})
		if err != nil && !k8serrors.IsNotFound(err) {
			return "failed to delete v1alpha1 cstorbackups", err
		}
	}

	// Migrate restore to v1 version
	oldRestoreList, err := obj.OpenebsClientset.OpenebsV1alpha1().
		CStorRestores(obj.OpenebsNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: types.CStorPoolInstanceNameLabelKey + "=" + obj.Name,
	})
	if err != nil && !k8serrors.IsNotFound(err) {
		return "failed to list v1alpha1 cstorrestores", err
	}
	for _, oldRestore := range oldRestoreList.Items {
		newRestore := translate.TranslateRestoreToV1(oldRestore)
		_, err := obj.OpenebsClientset.CstorV1().
			CStorRestores(obj.OpenebsNamespace).Create(context.TODO(),
			newRestore, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return "failed to create v1 cstorrestore for " + oldRestore.Name, err
		}

	}
	if len(oldRestoreList.Items) != 0 {
		err = obj.OpenebsClientset.OpenebsV1alpha1().
			CStorRestores(obj.OpenebsNamespace).
			DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: types.CStorPoolInstanceNameLabelKey + "=" + obj.Name,
			})
		if err != nil && !k8serrors.IsNotFound(err) {
			return "failed to delete v1alpha1 cstorrestores", err
		}
	}

	// Migrate completedbackup to v1 version
	oldCompletedBackupList, err := obj.OpenebsClientset.OpenebsV1alpha1().
		CStorCompletedBackups(obj.OpenebsNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: types.CStorPoolInstanceNameLabelKey + "=" + obj.Name,
	})
	if err != nil && !k8serrors.IsNotFound(err) {
		return "failed to list v1alpha1 cstorcompletedbackups", err
	}
	for _, oldCompletedBackup := range oldCompletedBackupList.Items {
		newCompletedBackup := translate.TranslateCompletedBackupToV1(oldCompletedBackup)
		_, err := obj.OpenebsClientset.CstorV1().
			CStorCompletedBackups(obj.OpenebsNamespace).Create(context.TODO(),
			newCompletedBackup, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return "failed to create v1 cstorcompletedbackup for " + oldCompletedBackup.Name, err
		}

	}
	if len(oldCompletedBackupList.Items) != 0 {
		err = obj.OpenebsClientset.OpenebsV1alpha1().
			CStorCompletedBackups(obj.OpenebsNamespace).
			DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: types.CStorPoolInstanceNameLabelKey + "=" + obj.Name,
			})
		if err != nil && !k8serrors.IsNotFound(err) {
			return "failed to delete v1alpha1 cstorcompletedbackups", err
		}
	}
	return "", nil
}

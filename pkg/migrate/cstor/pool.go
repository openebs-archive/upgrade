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
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"

	cstor "github.com/openebs/api/v2/pkg/apis/cstor/v1"
	v1Alpha1API "github.com/openebs/api/v2/pkg/apis/openebs.io/v1alpha1"
	"github.com/openebs/api/v2/pkg/apis/types"
	openebsclientset "github.com/openebs/api/v2/pkg/client/clientset/versioned"
	apis "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	csp "github.com/openebs/maya/pkg/cstor/pool/v1alpha3"
	cvr "github.com/openebs/maya/pkg/cstor/volumereplica/v1alpha1"
	spc "github.com/openebs/maya/pkg/storagepoolclaim/v1alpha1"
	"github.com/openebs/upgrade/pkg/version"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

const (
	cspNameLabel           = "cstorpool.openebs.io/name"
	cspUIDLabel            = "cstorpool.openebs.io/uid"
	cspHostnameAnnotation  = "cstorpool.openebs.io/hostname"
	cspiNameLabel          = "cstorpoolinstance.openebs.io/name"
	cspiUIDLabel           = "cstorpoolinstance.openebs.io/uid"
	cspiHostnameAnnotation = "cstorpoolinstance.openebs.io/hostname"
	spcFinalizer           = "storagepoolclaim.openebs.io/finalizer"
	cspcFinalizer          = "cstorpoolcluster.openebs.io/finalizer"
	cspcKind               = "CStorPoolCluster"
)

// CSPCMigrator ...
type CSPCMigrator struct {
	// kubeclientset is a standard kubernetes clientset
	KubeClientset kubernetes.Interface
	// openebsclientset is a openebs custom resource package generated for custom API group.
	OpenebsClientset openebsclientset.Interface
	CSPCObj          *cstor.CStorPoolCluster
	SPCObj           *apis.StoragePoolClaim
	OpenebsNamespace string
	CSPCName         string
}

// SetCSPCName is used to initialize custom name if provided
func (c *CSPCMigrator) SetCSPCName(name string) {
	c.CSPCName = name
}

// Migrate ...
func (c *CSPCMigrator) Migrate(name, namespace string) error {
	c.OpenebsNamespace = namespace
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(err, "error building kubeconfig")
	}
	c.KubeClientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "error building kubernetes clientset")
	}
	c.OpenebsClientset, err = openebsclientset.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "error building openebs clientset")
	}
	mtask, err := getOrCreateMigrationTask("cstorPool", name, namespace, c, c.OpenebsClientset)
	if err != nil {
		return err
	}
	statusObj := v1Alpha1API.MigrationDetailedStatuses{Step: "Pre-migration"}
	statusObj.Phase = v1Alpha1API.StepWaiting
	mtask, uerr := updateMigrationDetailedStatus(mtask, statusObj, c.OpenebsNamespace, c.OpenebsClientset)
	if uerr != nil && IsMigrationTaskJob {
		return uerr
	}
	msg, err := c.preMigrate(name)
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		mtask, uerr = updateMigrationDetailedStatus(mtask, statusObj, c.OpenebsNamespace, c.OpenebsClientset)
		if uerr != nil && IsMigrationTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	statusObj.Phase = v1Alpha1API.StepCompleted
	statusObj.Message = "Pre-migration steps were successful"
	statusObj.Reason = ""
	mtask, uerr = updateMigrationDetailedStatus(mtask, statusObj, c.OpenebsNamespace, c.OpenebsClientset)
	if uerr != nil && IsMigrationTaskJob {
		return uerr
	}

	statusObj = v1Alpha1API.MigrationDetailedStatuses{Step: "Migrate"}
	statusObj.Phase = v1Alpha1API.StepWaiting
	mtask, uerr = updateMigrationDetailedStatus(mtask, statusObj, c.OpenebsNamespace, c.OpenebsClientset)
	if uerr != nil && IsMigrationTaskJob {
		return uerr
	}
	msg, err = c.migrate(name)
	if err != nil {
		statusObj.Message = msg
		statusObj.Reason = err.Error()
		mtask, uerr = updateMigrationDetailedStatus(mtask, statusObj, c.OpenebsNamespace, c.OpenebsClientset)
		if uerr != nil && IsMigrationTaskJob {
			return uerr
		}
		return errors.Wrap(err, msg)
	}
	statusObj.Phase = v1Alpha1API.StepCompleted
	statusObj.Message = "Migration steps were successful"
	statusObj.Reason = ""
	mtask, uerr = updateMigrationDetailedStatus(mtask, statusObj, c.OpenebsNamespace, c.OpenebsClientset)
	if uerr != nil && IsMigrationTaskJob {
		return uerr
	}
	return nil
}

func (c *CSPCMigrator) preMigrate(name string) (string, error) {
	var msg string
	err := c.validateCSPCOperator()
	if err != nil {
		msg = "error validating cspc operator"
		return msg, err
	}
	if c.CSPCName == "" {
		c.CSPCName = name
	}
	err = c.checkForExistingCSPC(name)
	if err != nil {
		msg = "error while checking for existing cspc"
		return msg, err
	}
	err = c.correctBDs(name)
	if err != nil {
		msg = "error while correcting incorrect bds in spc"
		return msg, err
	}
	return "", nil
}

func (c *CSPCMigrator) validateCSPCOperator() error {
	currentVersion := strings.Split(version.Current(), "-")[0]
	operatorPods, err := c.KubeClientset.CoreV1().
		Pods(c.OpenebsNamespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: "openebs.io/component-name=cspc-operator",
		})
	if err != nil {
		return err
	}
	if len(operatorPods.Items) == 0 {
		return fmt.Errorf("cspc operator pod missing")
	}
	for _, pod := range operatorPods.Items {
		operatorVersion := strings.Split(pod.Labels["openebs.io/version"], "-")[0]
		if err != nil {
			return errors.Wrap(err, "failed to get operator version")
		}
		if operatorVersion != currentVersion {
			return fmt.Errorf("cspc operator is in %s version, please upgrade it to %s version or use migrate image with tag same as cspc operator",
				pod.Labels["openebs.io/version"], currentVersion)
		}
	}
	return nil
}

// Pool migrates the pool from SPC schema to CSPC schema
func (c *CSPCMigrator) migrate(spcName string) (string, error) {
	var err error
	var migrated bool
	var msg string
	c.SPCObj, migrated, err = c.getSPCWithMigrationStatus(spcName)
	if migrated {
		klog.Infof("spc %s is already migrated to cspc", spcName)
		return "", nil
	}
	if err != nil {
		msg = "error checking migration status of spc " + spcName
		return msg, err
	}
	err = c.validateSPC()
	if err != nil {
		msg = "failed to validate spc " + spcName
		return msg, err
	}
	err = c.updateBDCLabels()
	if err != nil {
		msg = "failed to update bdc labels for spc " + spcName
		return msg, err
	}
	klog.Infof("Creating equivalent cspc %s for spc %s", c.CSPCName, spcName)
	c.CSPCObj, err = c.generateCSPC(spcName)
	if err != nil {
		msg = "failed to create equivalent cspc for spc " + spcName
		return msg, err
	}
	err = c.updateBDCOwnerRef()
	if err != nil {
		msg = "failed to update bdc with cspc ownerReference"
		return msg, err
	}
	// List all cspi created with reconcile off
	cspiList, err := c.OpenebsClientset.CstorV1().
		CStorPoolInstances(c.OpenebsNamespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: string(apis.CStorPoolClusterCPK) + "=" + c.CSPCObj.Name,
		})
	if err != nil {
		msg = "failed to list cspi for cspc " + c.CSPCName
		return msg, err
	}

	// Perform migration for each cspi created on similar node as csp
	for _, cspiItem := range cspiList.Items {
		cspiItem := cspiItem // pin it
		cspiObj := &cspiItem
		err = c.cspTocspi(cspiObj)
		if err != nil {
			msg = "failed to migrate cspi " + cspiObj.Name
			return msg, err
		}
	}
	err = c.addSkipAnnotationToSPC(c.SPCObj.Name)
	if err != nil {
		msg = "failed to add skip-validation annotation to spc " + spcName
		return msg, err
	}
	// Clean up old SPC resources after the migration is complete
	err = spc.NewKubeClient().
		Delete(spcName, &metav1.DeleteOptions{})
	if err != nil {
		msg = "failed to clean up spc " + spcName
		return msg, err
	}
	return "", nil
}

// checkForExistingCSPC verifies the migration as follows:
// spc = getSPC
// 1. if spc does not exist
// 	return nil (getSPCWithMigrationStatus will handle it)
// cspc = getCSPC(from flag or same as spc)
// 2. if cspc exist &&
// 	a. spc has anno with cspc-name && cspc has migration anno with spc-name
// 		return nil
// 	b. else
// 		return err cspc already exists
// 3. if cspc does not exist &&
// 	a. spc has no anno
// 		patch spc with anno
// 	b. spc has diff anno than current cspc-name
// 		return err
func (c *CSPCMigrator) checkForExistingCSPC(spcName string) error {
	spcObj, err := spc.NewKubeClient().Get(spcName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if k8serrors.IsNotFound(err) {
		return nil
	}
	cspc, err := c.OpenebsClientset.CstorV1().
		CStorPoolClusters(c.OpenebsNamespace).
		Get(context.TODO(), c.CSPCName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if err == nil {
		if spcObj.Annotations[types.CStorPoolClusterLabelKey] == c.CSPCName &&
			cspc.Annotations["openebs.io/migrated-from"] == spcName {
			return nil
		}
		return errors.Errorf(
			"failed to validate migration: the spc %s is set to be renamed as %s, but got cspc-name %s instead",
			spcName,
			spcObj.Annotations[types.CStorPoolClusterLabelKey],
			c.CSPCName,
		)
	}
	if k8serrors.IsNotFound(err) {
		if spcObj.Annotations == nil || spcObj.Annotations[types.CStorPoolClusterLabelKey] == "" {
			return addCSPCAnnotationToSPC(spcObj, c.CSPCName)
		}
		if spcObj.Annotations[types.CStorPoolClusterLabelKey] != c.CSPCName {
			return errors.Errorf(
				"failed to validate migration: the spc %s is set to be renamed as %s, but got cspc-name %s instead",
				spcName,
				spcObj.Annotations[types.CStorPoolClusterLabelKey],
				c.CSPCName,
			)
		}
	}
	return nil
}

// validateSPC determines that if the spc is allowed to migrate or not.
// If the max pool count does not match the number of csp in case auto spc provisioning,
// or the blocldevice list in spc does not match bds from the csp, in case of manual provisioning
// pool migration can not be allowed.
func (c *CSPCMigrator) validateSPC() error {
	cspClient := csp.KubeClient()
	cspList, err := cspClient.List(metav1.ListOptions{
		LabelSelector: string(apis.StoragePoolClaimCPK) + "=" + c.SPCObj.Name,
	})
	if err != nil {
		return err
	}
	if c.SPCObj.Spec.BlockDevices.BlockDeviceList == nil {
		if c.SPCObj.Spec.MaxPools == nil {
			return errors.Errorf("invalid spc %s neither has bdc list nor maxpools", c.SPCObj.Name)
		}
		if *c.SPCObj.Spec.MaxPools != len(cspList.Items) {
			return errors.Errorf("maxpool count does not match csp count expected: %d got: %d",
				*c.SPCObj.Spec.MaxPools, len(cspList.Items))
		}
		return nil
	}
	bdMap := map[string]int{}
	for _, bdName := range c.SPCObj.Spec.BlockDevices.BlockDeviceList {
		bdMap[bdName]++
	}
	for _, cspObj := range cspList.Items {
		for _, rg := range cspObj.Spec.Group {
			for _, bdObj := range rg.Item {
				bdMap[bdObj.Name]++
			}
		}
	}
	for bdName, count := range bdMap {
		// if bd is configured properly it should occur exactly twice
		// one in spc spec and one in csp spec
		if count != 2 {
			return errors.Errorf("bd %s is not configured properly", bdName)
		}
	}
	return nil
}

// getSPCWithMigrationStatus gets the spc by name and verifies if the spc is already
// migrated or not. The spc will not be present in the cluster as the last step
// of migration deletes the spc.
func (c *CSPCMigrator) getSPCWithMigrationStatus(spcName string) (*apis.StoragePoolClaim, bool, error) {
	spcObj, err := spc.NewKubeClient().
		Get(spcName, metav1.GetOptions{})
	// verify if the spc is already migrated. If an equivalent cspc exists then
	// spc is already migrated as spc is only deleted as last step.
	if k8serrors.IsNotFound(err) {
		klog.Infof("spc %s not found.", spcName)
		_, err = c.OpenebsClientset.CstorV1().
			CStorPoolClusters(c.OpenebsNamespace).Get(context.TODO(), c.CSPCName, metav1.GetOptions{})
		if err != nil {
			return nil, false, errors.Wrapf(err, "failed to get equivalent cspc %s for spc %s", c.CSPCName, spcName)
		}
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return spcObj, false, nil
}

// csptocspi migrates a CSP to CSPI based on hostname
func (c *CSPCMigrator) cspTocspi(cspiObj *cstor.CStorPoolInstance) error {
	var err1 error
	hostnameLabel := types.HostNameLabelKey + "=" + cspiObj.Labels[types.HostNameLabelKey]
	spcLabel := string(apis.StoragePoolClaimCPK) + "=" + c.SPCObj.Name
	cspLabel := hostnameLabel + "," + spcLabel
	cspObj, err := getCSP(cspLabel)
	if err != nil {
		return err
	}
	if cspiObj.Annotations[types.OpenEBSDisableReconcileLabelKey] != "" {
		klog.Infof("Migrating csp %s to cspi %s", cspObj.Name, cspiObj.Name)
		err = c.scaleDownDeployment(cspObj.Name, cspiObj.Name, c.OpenebsNamespace)
		if err != nil {
			return err
		}
		// once the old pool pod is scaled down and bdcs are patched
		// bring up the cspi pod so that the old pool can be renamed and imported.
		cspiObj.Annotations[types.OpenEBSCStorExistingPoolName] = "cstor-" + string(cspObj.UID)
		cspiObj.Status.Phase = cstor.CStorPoolStatusOffline
		delete(cspiObj.Annotations, types.OpenEBSDisableReconcileLabelKey)
		cspiObj, err = c.OpenebsClientset.CstorV1().
			CStorPoolInstances(c.OpenebsNamespace).
			Update(context.TODO(), cspiObj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	for {
		cspiObj, err1 = c.OpenebsClientset.CstorV1().
			CStorPoolInstances(c.OpenebsNamespace).
			Get(context.TODO(), cspiObj.Name, metav1.GetOptions{})
		if err1 != nil {
			klog.Errorf("failed to get cspi %s: %s", cspiObj.Name, err1.Error())
		} else {
			if cspiObj.Status.Phase == "ONLINE" {
				break
			}
			klog.Infof("waiting for cspi %s to come to ONLINE state, got %s",
				cspiObj.Name, cspiObj.Status.Phase)
		}
		time.Sleep(10 * time.Second)
	}
	err = c.updateCVRsLabels(cspObj, cspiObj)
	if err != nil {
		return err
	}
	err = c.upgradeBackupRestore(string(cspObj.UID), cspiObj)
	if err != nil {
		return err
	}
	// remove the finalizers from csp object for cleanup
	// as pool pod is no longer running to remove them.
	return removeCSPFinalizers(cspObj)
}

func removeCSPFinalizers(cspObj *apis.CStorPool) error {
	cspClient := csp.KubeClient()
	newCSP := cspObj.DeepCopy()
	newCSP.Finalizers = []string{}
	patchData, err := GetPatchData(cspObj, newCSP)
	if err != nil {
		return err
	}
	_, err = cspClient.Patch(
		cspObj.Name,
		k8stypes.MergePatchType,
		patchData,
	)
	return err
}

// get csp for cspi on the basis of cspLabel, which is the combination of
// hostname label on which cspi came up and the spc label.
func getCSP(cspLabel string) (*apis.CStorPool, error) {
	cspClient := csp.KubeClient()
	cspList, err := cspClient.List(metav1.ListOptions{
		LabelSelector: cspLabel,
	})
	if err != nil {
		return nil, err
	}
	if len(cspList.Items) != 1 {
		return nil, fmt.Errorf("Invalid number of pools on one node: %v", cspList.Items)
	}
	cspObj := cspList.Items[0]
	return &cspObj, nil
}

// The old pool pod should be scaled down before the new cspi pod reconcile is
// enabled to avoid importing the pool at two places at the same time.
func (c *CSPCMigrator) scaleDownDeployment(cspName, cspiName, openebsNamespace string) error {
	var zero int32 = 0
	klog.Infof("Scaling down csp deployment %s", cspName)
	cspDeployList, err := c.KubeClientset.AppsV1().
		Deployments(openebsNamespace).List(context.TODO(),
		metav1.ListOptions{
			LabelSelector: "openebs.io/cstor-pool=" + cspName,
		})
	if err != nil {
		return err
	}
	if len(cspDeployList.Items) != 1 {
		return errors.Errorf("invalid number of csp deployment found for %s: expected 1, got %d", cspName, len(cspDeployList.Items))
	}
	newCSPDeploy := cspDeployList.Items[0]
	cspiDeploy, err := c.KubeClientset.AppsV1().
		Deployments(openebsNamespace).Get(context.TODO(), cspiName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get deployment for cspi %s", cspiName)
	}
	// While scaling down the csp deployment changing the
	// volumes as well so that the zrepl.lock file used by
	// csp and cspi becomes the same to avoid data corruption
	// due to multiple imports at the same time.
	newCSPDeploy.Spec.Replicas = &zero
	newCSPDeploy.Spec.Template.Spec.Volumes = cspiDeploy.Spec.Template.Spec.Volumes
	patchData, err := GetPatchData(cspDeployList.Items[0], newCSPDeploy)
	if err != nil {
		return errors.Wrapf(err, "failed to patch data for csp %s", cspName)
	}
	_, err = c.KubeClientset.AppsV1().Deployments(openebsNamespace).
		Patch(context.TODO(),
			cspDeployList.Items[0].Name,
			k8stypes.StrategicMergePatchType,
			patchData,
			metav1.PatchOptions{},
		)
	if err != nil {
		return err
	}
	for {
		cspPods, err1 := c.KubeClientset.CoreV1().
			Pods(openebsNamespace).
			List(context.TODO(), metav1.ListOptions{
				LabelSelector: "openebs.io/cstor-pool=" + cspName,
			})
		if err1 != nil {
			klog.Errorf("failed to list pods for csp %s deployment: %s", cspName, err1.Error())
		} else {
			if len(cspPods.Items) == 0 {
				break
			}
			klog.Infof("waiting for csp %s deployment to scale down", cspName)
		}
		time.Sleep(10 * time.Second)
	}
	return nil
}

// Update the bdc with the cspc labels instead of spc labels to allow
// filtering of bds claimed by the migrated cspc.
func (c *CSPCMigrator) updateBDCLabels() error {
	bdcList, err := c.OpenebsClientset.OpenebsV1alpha1().BlockDeviceClaims(c.OpenebsNamespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: string(apis.StoragePoolClaimCPK) + "=" + c.SPCObj.Name,
		})
	if err != nil {
		return err
	}
	for _, bdcItem := range bdcList.Items {
		if bdcItem.Labels[string(apis.StoragePoolClaimCPK)] != "" {
			bdcItem := bdcItem // pin it
			bdcObj := &bdcItem
			klog.Infof("Updating bdc %s with cspc labels & finalizer.", bdcObj.Name)
			delete(bdcObj.Labels, string(apis.StoragePoolClaimCPK))
			bdcObj.Labels[types.CStorPoolClusterLabelKey] = c.CSPCName
			for i, finalizer := range bdcObj.Finalizers {
				if finalizer == spcFinalizer {
					bdcObj.Finalizers[i] = cspcFinalizer
				}
			}
			_, err := c.OpenebsClientset.OpenebsV1alpha1().BlockDeviceClaims(c.OpenebsNamespace).
				Update(context.TODO(), bdcObj, metav1.UpdateOptions{})
			if err != nil {
				return errors.Wrapf(err, "failed to update bdc %s with cspc label & finalizer", bdcObj.Name)
			}
		}
	}
	return nil
}

// Update the bdc with the cspc OwnerReferences instead of spc OwnerReferences
// to allow clean up of bdcs on deletion of cspc.
func (c *CSPCMigrator) updateBDCOwnerRef() error {
	bdcList, err := c.OpenebsClientset.OpenebsV1alpha1().
		BlockDeviceClaims(c.OpenebsNamespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: types.CStorPoolClusterLabelKey + "=" + c.CSPCObj.Name,
		})
	if err != nil {
		return err
	}
	for _, bdcItem := range bdcList.Items {
		if bdcItem.OwnerReferences[0].Kind != cspcKind {
			bdcItem := bdcItem // pin it
			bdcObj := &bdcItem
			klog.Infof("Updating bdc %s with cspc %s ownerRef.", bdcObj.Name, c.CSPCObj.Name)
			bdcObj.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(c.CSPCObj,
					cstor.SchemeGroupVersion.WithKind(cspcKind)),
			}
			_, err := c.OpenebsClientset.OpenebsV1alpha1().BlockDeviceClaims(c.OpenebsNamespace).
				Update(context.TODO(), bdcObj, metav1.UpdateOptions{})
			if err != nil {
				return errors.Wrapf(err, "failed to update bdc %s with cspc onwerRef", bdcObj.Name)
			}
		}
	}
	return nil
}

// Update the cvrs on the old csp with the migrated cspi labels and annotations
// to allow backward compatibility with old external provisioned volumes.
func (c *CSPCMigrator) updateCVRsLabels(cspObj *apis.CStorPool, cspiObj *cstor.CStorPoolInstance) error {
	cvrList, err := cvr.NewKubeclient().
		WithNamespace(c.OpenebsNamespace).List(metav1.ListOptions{
		LabelSelector: cspNameLabel + "=" + cspObj.Name,
	})
	if err != nil {
		return err
	}
	for _, cvrItem := range cvrList.Items {
		if cvrItem.Labels[cspiNameLabel] == "" {
			cvrItem := cvrItem // pin it
			cvrObj := &cvrItem
			klog.Infof("Updating cvr %s with cspi %s info.", cvrObj.Name, cspiObj.Name)
			delete(cvrObj.Labels, cspNameLabel)
			delete(cvrObj.Labels, cspUIDLabel)
			delete(cvrObj.Annotations, cspHostnameAnnotation)
			cvrObj.Labels[cspiNameLabel] = cspiObj.Name
			cvrObj.Labels[cspiUIDLabel] = string(cspiObj.UID)
			cvrObj.Annotations[cspiHostnameAnnotation] = cspiObj.Spec.HostName
			_, err = cvr.NewKubeclient().WithNamespace(c.OpenebsNamespace).
				Update(cvrObj)
			if err != nil {
				return errors.Wrapf(err, "failed to update cvr %s with cspc info", cvrObj.Name)
			}
		}
	}
	return nil
}

func (c *CSPCMigrator) addSkipAnnotationToSPC(spcName string) error {
retry:
	spcObj, err := spc.NewKubeClient().Get(spcName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if spcObj.Annotations == nil {
		spcObj.Annotations = map[string]string{}
	}
	spcObj.Annotations["openebs.io/skip-validations"] = "true"
	_, err = spc.NewKubeClient().Update(spcObj)
	if k8serrors.IsConflict(err) {
		klog.Errorf("failed to update spc with skip-validation annotation due to conflict error")
		time.Sleep(2 * time.Second)
		goto retry
	}
	return err
}

func addCSPCAnnotationToSPC(spcObj *apis.StoragePoolClaim, cspcName string) error {
retry:
	if spcObj.Annotations == nil {
		spcObj.Annotations = map[string]string{}
	}
	spcObj.Annotations[types.CStorPoolClusterLabelKey] = cspcName
	_, err := spc.NewKubeClient().Update(spcObj)
	if k8serrors.IsConflict(err) {
		klog.Errorf("failed to update spc with cspc annotation due to conflict error")
		goto retry
	}
	return err
}

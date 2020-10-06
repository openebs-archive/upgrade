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
	"strings"
	"time"

	"github.com/openebs/api/pkg/apis/openebs.io/v1alpha1"
	openebstypes "github.com/openebs/api/pkg/apis/types"
	apis "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	csp "github.com/openebs/maya/pkg/cstor/pool/v1alpha3"
	pod "github.com/openebs/maya/pkg/kubernetes/pod/v1alpha1"
	spc "github.com/openebs/maya/pkg/storagepoolclaim/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

var (
	correctedBD = map[string]string{}
)

const (
	unit = 1024
)

func (c *CSPCMigrator) correctBDs(spcName string) error {
	_, err := c.OpenebsClientset.CstorV1().
		CStorPoolClusters(c.OpenebsNamespace).Get(c.CSPCName, metav1.GetOptions{})
	// if the CSPC already exists then no need to correct the schema
	if err == nil {
		return nil
	}
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to get cspc %s", c.CSPCName)
	}

retryspcupdate:
	spcObj, err := spc.NewKubeClient().Get(spcName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if spcObj.Spec.Type == string(apis.TypeSparseCPV) {
		return nil
	}

	if spcObj.Annotations[string(apis.OpenEBSDisableReconcileKey)] != "true" {
		spcObj.Annotations[string(apis.OpenEBSDisableReconcileKey)] = "true"
		spcObj, err = spc.NewKubeClient().Update(spcObj)
		if err != nil {
			if k8serrors.IsConflict(err) {
				klog.Errorf("failed to update spc with OpenEBSDisableReconcile annotation due to conflict error")
				time.Sleep(2 * time.Second)
				goto retryspcupdate
			}
			return err
		}
	}

	cspList, err := csp.KubeClient().List(
		metav1.ListOptions{
			LabelSelector: string(apis.StoragePoolClaimCPK) + "=" + spcName,
		},
	)
	if err != nil {
		return err
	}
	for _, cspObj := range cspList.Items {
		err = c.correctCSPBDs(spcObj, cspObj)
		if err != nil {
			return err
		}
	}

	for i, bdname := range spcObj.Spec.BlockDevices.BlockDeviceList {
		if correctedBD[bdname] != "" {
			spcObj.Spec.BlockDevices.BlockDeviceList[i] = correctedBD[bdname]
		}
	}
	delete(spcObj.Annotations, string(apis.OpenEBSDisableReconcileKey))
	_, err = spc.NewKubeClient().Update(spcObj)
	return err
}

func (c *CSPCMigrator) correctCSPBDs(spcObj *apis.StoragePoolClaim, cspObj apis.CStorPool) error {
	podList, err := c.KubeClientset.CoreV1().Pods(c.OpenebsNamespace).
		List(metav1.ListOptions{
			LabelSelector: "openebs.io/cstor-pool=" + cspObj.Name,
		})
	if err != nil {
		return err
	}
	if len(podList.Items) != 1 {
		return errors.Errorf("failed to get csp pod expected 1 got %d", len(podList.Items))
	}
	devLinks, err := c.getDevlinks(podList.Items[0].Name)
	if err != nil {
		return errors.Wrapf(err, "failed to get devlinks for csp %s", cspObj.Name)
	}
	devLinkBDMap := map[string]string{}
	index := 0
	for _, group := range cspObj.Spec.Group {
		for _, bd := range group.Item {
			devLinkBDMap[devLinks[index]] = bd.Name
			correctedBD[bd.Name] = ""
			index = index + 1
		}
	}
	correctCSPBD := map[string]string{}
	hostName := podList.Items[0].Spec.NodeSelector[openebstypes.HostNameLabelKey]
	for devlink, bdname := range devLinkBDMap {
		newBD, err := c.findBDforDevlink(devlink, hostName)
		if err != nil {
			return err
		}
		if newBD != bdname {
			correctCSPBD[bdname] = newBD
			correctedBD[bdname] = newBD
		}
	}

	newCSPObj := cspObj.DeepCopy()
	klog.Infof("Correcting the following bd in csp %s \n %v", cspObj.Name, correctCSPBD)
	for i, rg := range newCSPObj.Spec.Group {
		for j, bd := range rg.Item {
			if correctCSPBD[bd.Name] != "" {
				err = c.updateBDRefsAndlabels(spcObj, bd.Name, correctCSPBD[bd.Name])
				if err != nil {
					return errors.Wrapf(err, "failed to update old bd %s & new bd %s for spc %s",
						bd.Name, correctCSPBD[bd.Name], spcObj.Name,
					)
				}
				newCSPObj.Spec.Group[i].Item[j].Name = correctCSPBD[bd.Name]
				// this field will automatically corrected by the operator
				newCSPObj.Spec.Group[i].Item[j].DeviceID = ""
			}
		}
	}
	data, err := GetPatchData(cspObj, newCSPObj)
	if err != nil {
		return err
	}
	_, err = csp.KubeClient().
		Patch(cspObj.Name, types.MergePatchType, data)

	return err
}

func (c *CSPCMigrator) findBDforDevlink(devlink, hostname string) (string, error) {
	bds, err := c.OpenebsClientset.OpenebsV1alpha1().
		BlockDevices(c.OpenebsNamespace).
		List(metav1.ListOptions{
			LabelSelector: "kubernetes.io/hostname=" + hostname,
		})
	if err != nil {
		return "", err
	}
	for _, bd := range bds.Items {
		if bd.Spec.Path != "" {
			if strings.Contains(devlink, bd.Spec.Path) {
				ok, err := c.verifyBDStatus(bd, hostname)
				if err != nil {
					return "", err
				}
				if ok {
					return bd.Name, nil
				}
			}
		}
		for _, devLinks := range bd.Spec.DevLinks {
			for _, link := range devLinks.Links {
				if strings.Contains(devlink, link) {
					ok, err := c.verifyBDStatus(bd, hostname)
					if err != nil {
						return "", err
					}
					if ok {
						return bd.Name, nil
					}
				}
			}
		}
	}
	return "", errors.Errorf("blockdevice not found for devlink %s", devlink)
}

func (c *CSPCMigrator) verifyBDStatus(bdObj v1alpha1.BlockDevice, hostName string) (bool, error) {
	if bdObj.Status.State == v1alpha1.BlockDeviceActive {
		nodes, err := c.KubeClientset.CoreV1().Nodes().
			List(metav1.ListOptions{
				LabelSelector: openebstypes.HostNameLabelKey + "=" + hostName,
			})
		if err != nil {
			return false, err
		}
		if len(nodes.Items) == 1 {
			return true, nil
		}
	}
	return false, nil
}

func (c *CSPCMigrator) updateBDRefsAndlabels(spcObj *apis.StoragePoolClaim, oldBD, newBD string) error {
	spcKind := "StoragePoolClaim"
	oldBDObj, err := c.OpenebsClientset.OpenebsV1alpha1().
		BlockDevices(c.OpenebsNamespace).Get(oldBD, metav1.GetOptions{})
	if err != nil {
		return err
	}
	newBDObj, err := c.OpenebsClientset.OpenebsV1alpha1().
		BlockDevices(c.OpenebsNamespace).Get(newBD, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// For the new bd
	// - if bdc already present:
	//   - if ownerref already present: update the bdc with correct ownerref & label
	//   - if ownerref not present: add the spc ownerref & label bdc

	if newBDObj.Spec.ClaimRef != nil {
		newBDCObj, err := c.OpenebsClientset.OpenebsV1alpha1().
			BlockDeviceClaims(c.OpenebsNamespace).
			Get(newBDObj.Spec.ClaimRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if newBDCObj.Labels["openebs.io/storage-pool-claim"] != spcObj.Name {
			newBDCObj.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(spcObj,
					apis.SchemeGroupVersion.WithKind(spcKind)),
			}
			if newBDCObj.Labels == nil {
				newBDCObj.Labels = map[string]string{}
			}
			newBDCObj.Labels["openebs.io/storage-pool-claim"] = spcObj.Name
			newBDCObj, err = c.OpenebsClientset.OpenebsV1alpha1().
				BlockDeviceClaims(c.OpenebsNamespace).Update(newBDCObj)
		}
	} else {
		// - if bdc not present: remove legacy annotation on bd
		// this will enable claiming of the bd
		delete(newBDObj.Annotations, "internal.openebs.io/uuid-scheme")
		newBDObj, err = c.OpenebsClientset.OpenebsV1alpha1().
			BlockDevices(c.OpenebsNamespace).Update(newBDObj)
		if err != nil {
			return err
		}
		resourceList, err := getCapacity(byteCount(newBDObj.Spec.Capacity.Storage))
		if err != nil {
			return errors.Errorf("failed to get capacity from block device %s:%s",
				newBDObj.Name, err)
		}
		// claim the bd immediately so no other resource can take it
		newBDCObj := v1alpha1.NewBlockDeviceClaim().
			WithName("bdc-cstor-" + string(newBDObj.UID)).
			WithNamespace(c.OpenebsNamespace).
			WithLabels(map[string]string{string(apis.StoragePoolClaimCPK): spcObj.Name}).
			WithBlockDeviceName(newBDObj.Name).
			WithHostName(newBDObj.Labels[openebstypes.HostNameLabelKey]).
			WithCSPCOwnerReference(*metav1.NewControllerRef(spcObj,
				apis.SchemeGroupVersion.WithKind(spcKind))).
			WithCapacity(resourceList).
			WithFinalizer("storagepoolclaim.openebs.io/finalizer")

		newBDCObj, err = c.OpenebsClientset.OpenebsV1alpha1().
			BlockDeviceClaims(c.OpenebsNamespace).Create(newBDCObj)

	retryBDCStatus:
		newBDCObj, err = c.OpenebsClientset.OpenebsV1alpha1().
			BlockDeviceClaims(c.OpenebsNamespace).Get(newBDCObj.Name, metav1.GetOptions{})
		if newBDCObj.Status.Phase != v1alpha1.BlockDeviceClaimStatusDone {
			klog.Infof("waiting for bdc %s to get bound", newBDObj.Name)
			time.Sleep(2 * time.Second)
			goto retryBDCStatus
		}
	}
	// For the old bd
	// - if active: remove the ownerref & label from corresponding bd claim present
	// - if inactive: leave it as it is
	if oldBDObj.Status.State == "Active" {
		oldBDCObj, err := c.OpenebsClientset.OpenebsV1alpha1().
			BlockDeviceClaims(c.OpenebsNamespace).
			Get(oldBDObj.Spec.ClaimRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if oldBDCObj.Labels["openebs.io/storage-pool-claim"] == spcObj.Name {
			oldBDCObj.OwnerReferences = []metav1.OwnerReference{}
			delete(oldBDCObj.Labels, "openebs.io/storage-pool-claim")
			oldBDCObj, err = c.OpenebsClientset.OpenebsV1alpha1().
				BlockDeviceClaims(c.OpenebsNamespace).Update(oldBDCObj)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *CSPCMigrator) getDevlinks(podName string) ([]string, error) {
	podClient := pod.NewKubeClient()
	output, err := podClient.WithNamespace(c.OpenebsNamespace).
		Exec(podName, &corev1.PodExecOptions{
			Container: "cstor-pool",
			Command: []string{
				"bash",
				"-c",
				fmt.Sprint(`zpool status -P | grep \/dev | awk '{print $1}'`),
			},
			Stdin:  false,
			Stdout: true,
			Stderr: true,
		})
	if err != nil {
		return nil, err
	}
	if output.Stdout == "" {
		return nil, errors.Errorf("no devlinks found for pool pod %s", podName)
	}
	devlinks := strings.Split(output.Stdout, "\n")
	return devlinks, nil
}

func getCapacity(capacity string) (resource.Quantity, error) {
	resCapacity, err := resource.ParseQuantity(capacity)
	if err != nil {
		return resource.Quantity{}, errors.Errorf("Failed to parse capacity:{%s}", err.Error())
	}
	return resCapacity, nil
}

func byteCount(b uint64) string {
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, index := uint64(unit), 0
	for val := b / unit; val >= unit; val /= unit {
		div *= unit
		index++
	}
	return fmt.Sprintf("%d%c",
		uint64(b)/uint64(div), "KMGTPE"[index])
}

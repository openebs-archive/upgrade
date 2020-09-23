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
	// apis "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"fmt"

	apis "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	csp "github.com/openebs/maya/pkg/cstor/pool/v1alpha3"

	spc "github.com/openebs/maya/pkg/storagepoolclaim/v1alpha1"

	"strings"

	pod "github.com/openebs/maya/pkg/kubernetes/pod/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

var (
	correctedBD = map[string]string{}
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

	spcObj.Annotations[string(apis.OpenEBSDisableReconcileKey)] = "true"
	spcObj, err = spc.NewKubeClient().Update(spcObj)
	if err != nil {
		if k8serrors.IsConflict(err) {
			klog.Infof("retrying update on spc %s", spcName)
			goto retryspcupdate
		}
		return err
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
		err = c.correctCSPBDs(cspObj)
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

func (c *CSPCMigrator) correctCSPBDs(cspObj apis.CStorPool) error {
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
	hostName := podList.Items[0].Spec.NodeSelector["kubernetes.io/hostname"]
	for devlink, bdname := range devLinkBDMap {
		bdObj, err := c.OpenebsClientset.OpenebsV1alpha1().
			BlockDevices(c.OpenebsNamespace).
			Get(bdname, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if bdObj.Status.State == "Active" && bdObj.Status.ClaimState == "Claimed" {
			nodes, err := c.KubeClientset.CoreV1().Nodes().
				List(metav1.ListOptions{
					LabelSelector: "kubernetes.io/hostname=" + hostName,
				})
			if err != nil {
				return err
			}
			if len(nodes.Items) == 1 {
				continue
			}
		}
		newBD, err := c.findBDforDevlink(devlink, hostName)
		if err != nil {
			return err
		}
		correctCSPBD[bdname] = newBD
		correctedBD[bdname] = newBD
	}

	newCSPObj := cspObj.DeepCopy()

	for i, rg := range newCSPObj.Spec.Group {
		for j, bd := range rg.Item {
			if correctCSPBD[bd.Name] != "" {
				newCSPObj.Spec.Group[i].Item[j].Name = correctCSPBD[bd.Name]
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
		for _, devLinks := range bd.Spec.DevLinks {
			for _, link := range devLinks.Links {
				if strings.Contains(devlink, link) && bd.Status.State == "Active" &&
					bd.Status.ClaimState == "Unclaimed" {
					return bd.Name, nil
				}
			}
		}
	}
	return "", errors.Errorf("blockdevice not found for devlink %s", devlink)
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
	klog.Info(output)
	if err != nil {
		return nil, err
	}
	if output.Stdout == "" {
		return nil, errors.Errorf("no devlinks found for pool pod %s", podName)
	}
	devlinks := strings.Split(output.Stdout, "\n")
	return devlinks, nil
}

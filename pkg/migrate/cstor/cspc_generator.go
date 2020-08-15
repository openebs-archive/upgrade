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
	"time"

	cstor "github.com/openebs/api/pkg/apis/cstor/v1"
	"github.com/openebs/api/pkg/apis/types"
	apis "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	csp "github.com/openebs/maya/pkg/cstor/pool/v1alpha3"
	deploy "github.com/openebs/maya/pkg/kubernetes/deployment/appsv1/v1alpha1"
	"github.com/openebs/maya/pkg/util/retry"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	typeMap = map[string]string{
		string(apis.PoolTypeStripedCPV):  string(apis.PoolStriped),
		string(apis.PoolTypeMirroredCPV): string(apis.PoolMirrored),
		string(apis.PoolTypeRaidzCPV):    string(apis.PoolRaidz),
		string(apis.PoolTypeRaidz2CPV):   string(apis.PoolRaidz2),
	}
)

func getDataRaidGroups(cspObj apis.CStorPool) []cstor.RaidGroup {
	dataRaidGroups := []cstor.RaidGroup{}
	for _, rg := range cspObj.Spec.Group {
		// as the current spc provisioning stripe pool creates
		// different raid groups
		if cspObj.Spec.PoolSpec.PoolType == string(apis.PoolTypeStripedCPV) {
			if len(dataRaidGroups) == 0 {
				dataRaidGroups = append(dataRaidGroups, cstor.RaidGroup{})
			}
			dataRaidGroups[0].CStorPoolInstanceBlockDevices = append(dataRaidGroups[0].CStorPoolInstanceBlockDevices, getBDList(rg)...)
		} else {
			dataRaidGroups = append(dataRaidGroups,
				cstor.RaidGroup{
					CStorPoolInstanceBlockDevices: getBDList(rg),
				},
			)
		}
	}
	return dataRaidGroups
}

func getBDList(rg apis.BlockDeviceGroup) []cstor.CStorPoolInstanceBlockDevice {
	list := []cstor.CStorPoolInstanceBlockDevice{}
	for _, bdcObj := range rg.Item {
		list = append(list,
			cstor.CStorPoolInstanceBlockDevice{
				BlockDeviceName: bdcObj.Name,
			},
		)
	}
	return list
}

func (c *CSPCMigrator) getCSPCSpecForSPC(spcName string) (*cstor.CStorPoolCluster, error) {
	cspClient := csp.KubeClient()
	cspList, err := cspClient.List(metav1.ListOptions{
		LabelSelector: string(apis.StoragePoolClaimCPK) + "=" + c.SPCObj.Name,
	})
	if err != nil {
		return nil, err
	}
	cspcObj := &cstor.CStorPoolCluster{}
	cspcObj.Name = c.CSPCName
	cspcObj.Annotations = map[string]string{
		// This annotation will be used to disable reconciliation on the dependants.
		// In this case that will be CSPI
		types.OpenEBSDisableDependantsReconcileKey: "true",
		"openebs.io/migrated-from":                 spcName,
	}
	for _, cspObj := range cspList.Items {
		cspDeployList, err := deploy.NewKubeClient().WithNamespace(c.OpenebsNamespace).
			List(&metav1.ListOptions{
				LabelSelector: "openebs.io/cstor-pool=" + cspObj.Name,
			})
		if err != nil {
			return nil, err
		}
		if len(cspDeployList.Items) != 1 {
			return nil, errors.Errorf("invalid number of deployments found for csp %s: %d", cspObj.Name, len(cspDeployList.Items))
		}
		poolSpec := cstor.PoolSpec{
			NodeSelector: map[string]string{
				types.HostNameLabelKey: cspObj.Labels[string(apis.HostNameCPK)],
			},
			DataRaidGroups: getDataRaidGroups(cspObj),
			PoolConfig: cstor.PoolConfig{
				DataRaidGroupType: typeMap[cspObj.Spec.PoolSpec.PoolType],
				ThickProvision:    cspObj.Spec.PoolSpec.ThickProvisioning,
				Resources:         getCSPResources(cspDeployList.Items[0]),
				Tolerations:       cspDeployList.Items[0].Spec.Template.Spec.Tolerations,
				PriorityClassName: &cspDeployList.Items[0].Spec.Template.Spec.PriorityClassName,
				AuxResources:      getCSPAuxResources(cspDeployList.Items[0]),
				ROThresholdLimit:  &cspObj.Spec.PoolSpec.ROThresholdLimit,
			},
		}
		cspcObj.Spec.Pools = append(cspcObj.Spec.Pools, poolSpec)
	}
	return cspcObj, nil
}

func getCSPResources(cspDeploy appsv1.Deployment) *corev1.ResourceRequirements {
	for _, con := range cspDeploy.Spec.Template.Spec.Containers {
		if con.Name == "cstor-pool" {
			return &con.Resources
		}
	}
	return nil
}

func getCSPAuxResources(cspDeploy appsv1.Deployment) *corev1.ResourceRequirements {
	for _, con := range cspDeploy.Spec.Template.Spec.Containers {
		if con.Name == "cstor-pool-mgmt" {
			return &con.Resources
		}
	}
	return nil
}

// generateCSPC creates an equivalent cspc for the given spc object
func (c *CSPCMigrator) generateCSPC(spcName string) (
	*cstor.CStorPoolCluster, error) {
	cspcObj, err := c.OpenebsClientset.CstorV1().
		CStorPoolClusters(c.OpenebsNamespace).Get(c.CSPCName, metav1.GetOptions{})
	if !k8serrors.IsNotFound(err) && err != nil {
		return nil, err
	}
	if err != nil {
		cspcObj, err = c.getCSPCSpecForSPC(spcName)
		if err != nil {
			return nil, err
		}
		cspcObj, err = c.OpenebsClientset.CstorV1().
			CStorPoolClusters(c.OpenebsNamespace).Create(cspcObj)
		if err != nil {
			return nil, err
		}
	}
	if cspcObj.Annotations[types.OpenEBSDisableDependantsReconcileKey] == "" {
		return cspcObj, nil
	}
	// verify the number of cspi created is correct
	err = retry.
		Times(60).
		Wait(5 * time.Second).
		Try(func(attempt uint) error {
			cspiList, err1 := c.OpenebsClientset.CstorV1().
				CStorPoolInstances(c.OpenebsNamespace).List(
				metav1.ListOptions{
					LabelSelector: types.CStorPoolClusterLabelKey + "=" + cspcObj.Name,
				})
			if err1 != nil {
				return err1
			}
			if len(cspiList.Items) != len(cspcObj.Spec.Pools) {
				return errors.Errorf("failed to verify cspi count expected: %d got: %d",
					len(cspcObj.Spec.Pools),
					len(cspiList.Items),
				)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	cspcObj, err = c.OpenebsClientset.CstorV1().
		CStorPoolClusters(c.OpenebsNamespace).Get(c.CSPCName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// after all the cspi come up which reconcilation disabled delete the
	// OpenEBSDisableDependantsReconcileKey annotation so that in future when
	// a cspi is delete and it comes back on reconciliation it should not have
	// reconciliation disabled
	delete(cspcObj.Annotations, types.OpenEBSDisableDependantsReconcileKey)
	cspcObj, err = c.OpenebsClientset.CstorV1().
		CStorPoolClusters(c.OpenebsNamespace).
		Update(cspcObj)
	if err != nil {
		return nil, err
	}
	return cspcObj, nil
}

/*
Copyright 2020 The OpenEBS Authors.

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

package executor

import (
	"os"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/klog"

	v1Alpha1API "github.com/openebs/api/pkg/apis/openebs.io/v1alpha1"
	openebsclientset "github.com/openebs/api/pkg/client/clientset/versioned"
	"github.com/openebs/maya/pkg/util"
	cmdUtil "github.com/openebs/upgrade/cmd/util"
	upgrade "github.com/openebs/upgrade/pkg/upgrade"
	"github.com/openebs/upgrade/pkg/version"
	errors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	resourceUpgradeCmdHelpText = `
This command upgrades the resource mentioned in the UpgradeTask CR.
The name of the UpgradeTask CR is extracted from the ENV UPGRADE_TASK

Usage: upgrade resource
`
)

// ResourceOptions stores information required for upgradeTask upgrade
type ResourceOptions struct {
	name string
}

// NewUpgradeResourceJob upgrade a resource from upgradeTask
func NewUpgradeResourceJob() *cobra.Command {
	client, err := initClient()
	util.CheckErr(err, util.Fatal)
	cmd := &cobra.Command{
		Use:     "resource",
		Short:   "Upgrade a resource using the details specified in the UpgradeTask CR.",
		Long:    resourceUpgradeCmdHelpText,
		Example: `upgrade resource`,
		Run: func(cmd *cobra.Command, args []string) {
			upgradeTaskLabel := cmdUtil.GetUpgradeTaskLabel()
			openebsNamespace := cmdUtil.GetOpenEBSNamespace()
			upgradeTaskList, err := client.OpenebsV1alpha1().UpgradeTasks(openebsNamespace).
				List(metav1.ListOptions{
					LabelSelector: upgradeTaskLabel,
				})
			util.CheckErr(err, util.Fatal)
			if len(upgradeTaskList.Items) == 0 {
				util.Fatal("No resource found for given label")
			}
			for _, cr := range upgradeTaskList.Items {
				util.CheckErr(options.InitializeFromUpgradeTaskResource(cr), util.Fatal)
				util.CheckErr(options.RunPreFlightChecks(cmd), util.Fatal)
				util.CheckErr(options.RunResourceUpgradeChecks(cmd), util.Fatal)
				util.CheckErr(options.InitializeDefaults(cmd), util.Fatal)
				err := options.RunResourceUpgrade(cmd)
				if err != nil {
					utaskObj, err := client.OpenebsV1alpha1().UpgradeTasks(openebsNamespace).
						Get(cr.Name, metav1.GetOptions{})
					if err != nil {
						util.Fatal(err.Error())
					}
					backoffLimit, err := getBackoffLimit(openebsNamespace)
					if err != nil {
						util.Fatal(err.Error())
					}
					utaskObj.Status.Retries = utaskObj.Status.Retries + 1
					if utaskObj.Status.Retries == backoffLimit {
						utaskObj.Status.Phase = v1Alpha1API.UpgradeError
						utaskObj.Status.CompletedTime = metav1.Now()
					}
					_, err = client.OpenebsV1alpha1().UpgradeTasks(openebsNamespace).
						Update(utaskObj)
					if err != nil {
						util.Fatal(err.Error())
					}
				} else {
					utaskObj, err := client.OpenebsV1alpha1().UpgradeTasks(openebsNamespace).
						Get(cr.Name, metav1.GetOptions{})
					if err != nil {
						util.Fatal(err.Error())
					}
					utaskObj.Status.Phase = v1Alpha1API.UpgradeSuccess
					utaskObj.Status.CompletedTime = metav1.Now()
					_, err = client.OpenebsV1alpha1().UpgradeTasks(openebsNamespace).
						Update(utaskObj)
					if err != nil {
						util.Fatal(err.Error())
					}
				}
			}
		},
	}
	return cmd
}

// InitializeFromUpgradeTaskResource will populate the UpgradeOptions from given UpgradeTask
func (u *UpgradeOptions) InitializeFromUpgradeTaskResource(
	upgradeTaskCRObj v1Alpha1API.UpgradeTask) error {

	if len(strings.TrimSpace(u.openebsNamespace)) == 0 {
		return errors.Errorf("Cannot execute upgrade job: namespace is missing")
	}
	if len(strings.TrimSpace(upgradeTaskCRObj.Spec.FromVersion)) != 0 {
		u.fromVersion = upgradeTaskCRObj.Spec.FromVersion
	}

	if len(strings.TrimSpace(upgradeTaskCRObj.Spec.ToVersion)) != 0 {
		u.toVersion = upgradeTaskCRObj.Spec.ToVersion
	}

	if len(strings.TrimSpace(upgradeTaskCRObj.Spec.ImagePrefix)) != 0 {
		u.imageURLPrefix = upgradeTaskCRObj.Spec.ImagePrefix
	}

	if len(strings.TrimSpace(upgradeTaskCRObj.Spec.ImageTag)) != 0 {
		u.toVersionImageTag = upgradeTaskCRObj.Spec.ImageTag
	}

	switch {
	case upgradeTaskCRObj.Spec.ResourceSpec.CStorPoolInstance != nil:
		u.resourceKind = "cstorpoolinstance"
		u.name = upgradeTaskCRObj.Spec.ResourceSpec.CStorPoolInstance.CSPIName

	case upgradeTaskCRObj.Spec.ResourceSpec.CStorPoolCluster != nil:
		u.resourceKind = "cstorpoolcluster"
		u.name = upgradeTaskCRObj.Spec.ResourceSpec.CStorPoolCluster.CSPCName

	case upgradeTaskCRObj.Spec.ResourceSpec.CStorVolume != nil:
		u.resourceKind = "cstorVolume"
		u.name = upgradeTaskCRObj.Spec.ResourceSpec.CStorVolume.PVName
	}

	return nil
}

// RunResourceUpgradeChecks will ensure the sanity of the upgradeTask upgrade options
func (u *UpgradeOptions) RunResourceUpgradeChecks(cmd *cobra.Command) error {
	if len(strings.TrimSpace(u.name)) == 0 {
		return errors.Errorf("Cannot execute upgrade job: resource name is missing")
	}

	return nil
}

// RunResourceUpgrade upgrades the given upgradeTask
func (u *UpgradeOptions) RunResourceUpgrade(cmd *cobra.Command) error {
	if version.IsCurrentVersionValid(u.fromVersion) && version.IsDesiredVersionValid(u.toVersion) {
		klog.Infof("Upgrading to %s", u.toVersion)
		err := upgrade.Exec(u.fromVersion, u.toVersion,
			u.resourceKind,
			u.name,
			u.openebsNamespace,
			u.imageURLPrefix,
			u.toVersionImageTag)
		if err != nil {
			return errors.Wrapf(err, "Failed to upgrade %v %v", u.resourceKind, u.name)
		}
	} else {
		return errors.Errorf("Invalid from version %s or to version %s", u.fromVersion, u.toVersion)
	}
	return nil
}

func initClient() (openebsclientset.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "error building kubeconfig")
	}
	client, err := openebsclientset.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "error building openebs clientset")
	}
	return client, nil
}

func getBackoffLimit(openebsNamespace string) (int, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return 0, errors.Wrap(err, "error building kubeconfig")
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return 0, errors.Wrap(err, "error building openebs clientset")
	}
	podName := os.Getenv("POD_NAME")
	podObj, err := client.CoreV1().Pods(openebsNamespace).
		Get(podName, metav1.GetOptions{})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get backoff limit")
	}
	jobObj, err := client.BatchV1().Jobs(openebsNamespace).
		Get(podObj.OwnerReferences[0].Name, metav1.GetOptions{})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get backoff limit")
	}
	// if backoffLimit not present it returns the default as 6
	if jobObj.Spec.BackoffLimit == nil {
		return 6, nil
	}
	backoffLimit := int(*jobObj.Spec.BackoffLimit)
	return backoffLimit, nil
}

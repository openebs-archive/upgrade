/*
Copyright 2021 The OpenEBS Authors.

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
	"fmt"

	"github.com/openebs/maya/pkg/util"
	"github.com/spf13/cobra"
	"k8s.io/klog"

	upgrade "github.com/openebs/upgrade/pkg/upgrade"
	"github.com/openebs/upgrade/pkg/version"
	errors "github.com/pkg/errors"
)

var (
	jivaVolumeUpgradeCmdHelpText = `
This command upgrades the jiva volume

Usage: upgrade jiva-volume --options... <volume-name>...
`
)

// NewUpgradeJivaVolumeJob upgrades Jiva Volumes
func NewUpgradeJivaVolumeJob() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "jiva-volume",
		Short:   "Upgrade Jiva Volume",
		Long:    jivaVolumeUpgradeCmdHelpText,
		Example: `upgrade jiva-volume <spc-name>...`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				util.Fatal("failed to upgrade: no volume name provided")
			}
			for _, name := range args {
				options.resourceKind = "jivaVolume"
				util.CheckErr(options.RunPreFlightChecks(cmd), util.Fatal)
				util.CheckErr(options.InitializeDefaults(cmd), util.Fatal)
				util.CheckErr(options.RunJivaVolumeUpgrade(cmd, name), util.Fatal)
			}
		},
	}
	return cmd
}

// RunJivaVolumeUpgrade upgrades the given Jiva Volume.
func (u *UpgradeOptions) RunJivaVolumeUpgrade(cmd *cobra.Command, name string) error {

	if version.IsCurrentVersionValid(u.fromVersion) && version.IsDesiredVersionValid(u.toVersion) {
		klog.Infof("Upgrading %s to %s", name, u.toVersion)
		err := upgrade.Exec(u.fromVersion, u.toVersion,
			u.resourceKind,
			name,
			u.openebsNamespace,
			u.imageURLPrefix,
			u.toVersionImageTag)
		if err != nil {
			klog.Error(err)
			return errors.Errorf("Failed to upgrade JivaVolume %v", name)
		}
		klog.Infof("Successfully upgraded %s to %s", name, u.toVersion)
	} else {
		fmt.Println(version.IsCurrentVersionValid(u.fromVersion), version.IsDesiredVersionValid(u.toVersion))
		return errors.Errorf("Invalid from version %s or to version %s", u.fromVersion, u.toVersion)
	}
	return nil
}

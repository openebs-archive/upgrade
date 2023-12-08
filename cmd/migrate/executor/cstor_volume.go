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
	"strings"

	"github.com/openebs/maya/pkg/util"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	cstor "github.com/openebs/upgrade/pkg/migrate/cstor"

	"github.com/pkg/errors"
)

var (
	cstorVolumeMigrateCmdHelpText = `
This command migrates the cStor Volume to csi format

Usage: migrate cstor-volume --pv-name <pv-name>
`
)

// NewMigrateCStorVolumeJob migrates all the cStor Pools associated with
// a given Storage Pool Claim
func NewMigrateCStorVolumeJob() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cstor-volume",
		Short:   "Migrate cStor Volume",
		Long:    cstorVolumeMigrateCmdHelpText,
		Example: `migrate cstor-volume <pv-name>`,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(options.RunPreFlightChecks(), util.Fatal)
			util.CheckErr(options.RunCStorVolumeMigrateChecks(), util.Fatal)
			util.CheckErr(options.RunCStorVolumeMigrate(), util.Fatal)
		},
	}

	cmd.Flags().StringVarP(&options.pvName,
		"pv-name", "",
		options.pvName,
		"cstor Volume name to be migrated. Run \"kubectl get pv\", to get pv-name")

	return cmd
}

// RunCStorVolumeMigrateChecks will ensure the sanity of the cstor Volume migrate options
func (m *MigrateOptions) RunCStorVolumeMigrateChecks() error {
	if len(strings.TrimSpace(m.pvName)) == 0 {
		return errors.Errorf("Cannot execute migrate job: cstor pv name is missing")
	}

	return nil
}

// RunCStorVolumeMigrate migrates the given pv.
func (m *MigrateOptions) RunCStorVolumeMigrate() error {

	klog.Infof("Migrating volume %s to csi spec", m.pvName)
	migrator := cstor.VolumeMigrator{}
	err := migrator.Migrate(m.pvName, m.openebsNamespace)
	if err != nil {
		klog.Error(err)
		return errors.Errorf("Failed to migrate cStor Volume : %s", m.pvName)
	}
	klog.Infof("Successfully migrated volume %s, scale up the application to verify the migration", m.pvName)

	return nil
}

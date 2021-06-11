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
	cstor "github.com/openebs/upgrade/pkg/migrate/cstor"
	"github.com/spf13/cobra"
	"k8s.io/klog"

	errors "github.com/pkg/errors"
)

var (
	cstorSPCMigrateCmdHelpText = `
This command migrates the cStor SPC to CSPC

Usage: migrate cstor-spc --spc-name <spc-name>
`
)

// NewMigratePoolJob migrates all the cStor Pools associated with
// a given Storage Pool Claim
func NewMigratePoolJob() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cstor-spc",
		Short:   "Migrate cStor SPC",
		Long:    cstorSPCMigrateCmdHelpText,
		Example: `migrate cstor-spc --spc-name <spc-name>`,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(options.RunPreFlightChecks(), util.Fatal)
			util.CheckErr(options.RunCStorSPCMigrateChecks(), util.Fatal)
			util.CheckErr(options.RunCStorSPCMigrate(), util.Fatal)
		},
	}

	cmd.Flags().StringVarP(&options.spcName,
		"spc-name", "",
		options.spcName,
		"cstor SPC name to be migrated. Run \"kubectl get spc\", to get spc-name")

	cmd.Flags().StringVarP(&options.cspcName,
		"cspc-name", "",
		options.cspcName,
		"[optional] custom cspc name. By default cspc is created with same name as spc")

	return cmd
}

// RunCStorSPCMigrateChecks will ensure the sanity of the cstor SPC migrate options
func (m *MigrateOptions) RunCStorSPCMigrateChecks() error {
	if len(strings.TrimSpace(m.spcName)) == 0 {
		return errors.Errorf("Cannot execute migrate job: cstor spc name is missing")
	}

	return nil
}

// RunCStorSPCMigrate migrates the given spc.
func (m *MigrateOptions) RunCStorSPCMigrate() error {

	klog.Infof("Migrating spc %s to cspc", m.spcName)
	migrator := cstor.CSPCMigrator{}
	if m.cspcName != "" {
		klog.Infof("using custom cspc name as %s", m.cspcName)
		migrator.SetCSPCName(m.cspcName)
	}
	err := migrator.Migrate(m.spcName, m.openebsNamespace)
	if err != nil {
		klog.Error(err)
		return errors.Errorf("Failed to migrate cStor SPC : %s", m.spcName)
	}
	klog.Infof("Successfully migrated spc %s to cspc", m.spcName)

	klog.Infof("Make sure to migrate the associated PVs, "+
		"to list CStorVolumes for the PVs which are pending migration use `kubectl get cstorvolume.openebs.io -n %s`, "+
		"and to list CStorVolumes for the migrated/CSI PVs use `kubectl get cstorvolume.cstor.openebs.io -n %s`",
		m.openebsNamespace, m.openebsNamespace)
	return nil
}

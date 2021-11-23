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
	"context"
	"os"
	"strings"

	"k8s.io/client-go/rest"

	v1Alpha1API "github.com/openebs/api/v3/pkg/apis/openebs.io/v1alpha1"
	openebsclientset "github.com/openebs/api/v3/pkg/client/clientset/versioned"
	"github.com/openebs/maya/pkg/util"
	cmdUtil "github.com/openebs/upgrade/cmd/util"
	migrate "github.com/openebs/upgrade/pkg/migrate/cstor"
	errors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	resourceMigrateCmdHelpText = `
This command migrates the resource mentioned in the MigrationTask CR.
The name of the MigrationTask CR is extracted from the ENV UPGRADE_TASK

Usage: migrate resource
`
)

// ResourceOptions stores information required for migrationTask migrate
type ResourceOptions struct {
	name string
}

// NewMigrateResourceJob migrate a resource from migrationTask
func NewMigrateResourceJob() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resource",
		Short:   "Migrate a resource using the details specified in the MigrationTask CR.",
		Long:    resourceMigrateCmdHelpText,
		Example: `migrate resource`,
		Run: func(cmd *cobra.Command, args []string) {
			client, err := initClient()
			util.CheckErr(err, util.Fatal)
			name := args[0]
			openebsNamespace := cmdUtil.GetOpenEBSNamespace()
			migrationTaskObj, err := client.OpenebsV1alpha1().
				MigrationTasks(openebsNamespace).
				Get(context.TODO(), name, metav1.GetOptions{})
			util.CheckErr(err, util.Fatal)
			util.CheckErr(options.InitializeFromMigrationTaskResource(migrationTaskObj), util.Fatal)
			util.CheckErr(options.RunPreFlightChecks(), util.Fatal)
			err = options.RunResourceMigrate()
			if err != nil {
				migrationTaskObj, uerr := client.OpenebsV1alpha1().MigrationTasks(openebsNamespace).
					Get(context.TODO(), name, metav1.GetOptions{})
				if uerr != nil {
					util.Fatal(uerr.Error())
				}
				backoffLimit, uerr := getBackoffLimit(openebsNamespace)
				if uerr != nil {
					util.Fatal(uerr.Error())
				}
				migrationTaskObj.Status.Retries = migrationTaskObj.Status.Retries + 1
				if migrationTaskObj.Status.Retries == backoffLimit {
					migrationTaskObj.Status.Phase = v1Alpha1API.MigrateError
					migrationTaskObj.Status.CompletedTime = metav1.Now()
				}
				_, uerr = client.OpenebsV1alpha1().MigrationTasks(openebsNamespace).
					Update(context.TODO(), migrationTaskObj, metav1.UpdateOptions{})
				if uerr != nil {
					util.Fatal(uerr.Error())
				}
				util.Fatal(err.Error())
			} else {
				migrationTaskObj, uerr := client.OpenebsV1alpha1().MigrationTasks(openebsNamespace).
					Get(context.TODO(), name, metav1.GetOptions{})
				if uerr != nil {
					util.Fatal(uerr.Error())
				}
				migrationTaskObj.Status.Phase = v1Alpha1API.MigrateSuccess
				migrationTaskObj.Status.CompletedTime = metav1.Now()
				_, uerr = client.OpenebsV1alpha1().MigrationTasks(openebsNamespace).
					Update(context.TODO(), migrationTaskObj, metav1.UpdateOptions{})
				if uerr != nil {
					util.Fatal(uerr.Error())
				}
			}
		},
	}
	return cmd
}

// InitializeFromMigrationTaskResource will populate the MigrateOptions from given MigrationTask
func (m *MigrateOptions) InitializeFromMigrationTaskResource(
	migrationTaskObj *v1Alpha1API.MigrationTask) error {

	if len(strings.TrimSpace(m.openebsNamespace)) == 0 {
		return errors.Errorf("Cannot execute migrate job: namespace is missing")
	}

	switch {
	case migrationTaskObj.Spec.MigrateResource.MigrateCStorPool != nil:
		m.resourceKind = "storagePoolClaim"
		m.spcName = migrationTaskObj.Spec.MigrateCStorPool.SPCName
		m.cspcName = migrationTaskObj.Spec.MigrateCStorPool.Rename

	case migrationTaskObj.Spec.MigrateResource.MigrateCStorVolume != nil:
		m.resourceKind = "cstorVolume"
		m.pvName = migrationTaskObj.Spec.MigrateCStorVolume.PVName
	}

	return nil
}

// RunResourceMigrate migrates the given migrationTask
func (m *MigrateOptions) RunResourceMigrate() error {
	migrate.IsMigrationTaskJob = true
	var err error
	switch m.resourceKind {
	case "storagePoolClaim":
		err = m.RunCStorSPCMigrate()
	case "cstorVolume":
		err = m.RunCStorVolumeMigrate()
	}
	return err
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
		Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get backoff limit")
	}
	jobObj, err := client.BatchV1().Jobs(openebsNamespace).
		Get(context.TODO(), podObj.OwnerReferences[0].Name, metav1.GetOptions{})
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

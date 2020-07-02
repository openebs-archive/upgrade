/*
Copyright 2019 The OpenEBS Authors

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

package util

import (
	"os"
)

const (
	openebsNamespaceEnv = "OPENEBS_NAMESPACE"
	upgradeTaskLabel    = "UPGRADE_TASK_LABEL"
)

// GetOpenEBSNamespace gets the openebs namespace set to
// the OPENEBS_NAMESPACE env
func GetOpenEBSNamespace() string {
	return os.Getenv(openebsNamespaceEnv)
}

// GetUpgradeTaskLabel gets the upgradeTask label set to
// the UPGRADE_TASK_LABEL env
func GetUpgradeTaskLabel() string {
	return os.Getenv(upgradeTaskLabel)
}

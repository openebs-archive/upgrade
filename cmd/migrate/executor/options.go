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

	errors "github.com/pkg/errors"
)

// MigrateOptions stores information required for migration of
// OpenEBS resources
type MigrateOptions struct {
	openebsNamespace string
	spcName          string
	cspcName         string
	pvName           string
}

var (
	options = &MigrateOptions{
		openebsNamespace: "openebs",
	}
)

// RunPreFlightChecks will ensure the sanity of the common migrate options
func (m *MigrateOptions) RunPreFlightChecks() error {
	if len(strings.TrimSpace(m.openebsNamespace)) == 0 {
		return errors.Errorf("Cannot execute migrate job: openebs namespace is missing")
	}
	return nil
}

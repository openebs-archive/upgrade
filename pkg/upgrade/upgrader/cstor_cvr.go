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

package upgrader

import (
	"context"
	"time"

	apis "github.com/openebs/api/v2/pkg/apis/cstor/v1"
	"github.com/openebs/upgrade/pkg/upgrade/patch"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

// CVRPatch is the patch required to upgrade cvr
type CVRPatch struct {
	*ResourcePatch
	Namespace string
	CVR       *patch.CVR
	*Client
}

// CVRPatchOptions ...
type CVRPatchOptions func(*CVRPatch)

// WithCVRResorcePatch ...
func WithCVRResorcePatch(r *ResourcePatch) CVRPatchOptions {
	return func(obj *CVRPatch) {
		obj.ResourcePatch = r
	}
}

// WithCVRClient ...
func WithCVRClient(c *Client) CVRPatchOptions {
	return func(obj *CVRPatch) {
		obj.Client = c
	}
}

// NewCVRPatch ...
func NewCVRPatch(opts ...CVRPatchOptions) *CVRPatch {
	obj := &CVRPatch{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// PreUpgrade ...
func (obj *CVRPatch) PreUpgrade() error {
	err := obj.verifyCSPIVersion()
	if err != nil {
		return err
	}
	err = obj.CVR.PreChecks(obj.From, obj.To)
	return err
}

// CVRUpgrade ...
func (obj *CVRPatch) CVRUpgrade() error {
	err := obj.CVR.Patch(obj.From, obj.To)
	if err != nil {
		return err
	}
	return nil
}

// Upgrade execute the steps to upgrade cvr
func (obj *CVRPatch) Upgrade() error {
	err := obj.Init()
	if err != nil {
		return err
	}
	err = obj.PreUpgrade()
	if err != nil {
		return err
	}
	err = obj.CVRUpgrade()
	if err != nil {
		return err
	}
	err = obj.verifyCVRVersionReconcile()
	return err
}

// Init initializes all the fields of the CVRPatch
func (obj *CVRPatch) Init() error {
	obj.Namespace = obj.OpenebsNamespace
	obj.CVR = patch.NewCVR(
		patch.WithCVRClient(obj.OpenebsClientset),
	)
	err := obj.CVR.Get(obj.Name, obj.Namespace)
	if err != nil {
		return err
	}
	err = getCVRPatchData(obj)
	return err
}

func getCVRPatchData(obj *CVRPatch) error {
	newCVR := obj.CVR.Object.DeepCopy()
	err := transformCVR(newCVR, obj.ResourcePatch)
	if err != nil {
		return err
	}
	obj.CVR.Data, err = GetPatchData(obj.CVR.Object, newCVR)
	return err
}

func transformCVR(c *apis.CStorVolumeReplica, res *ResourcePatch) error {
	c.Labels["openebs.io/version"] = res.To
	c.VersionDetails.Desired = res.To
	return nil
}

func (obj *CVRPatch) verifyCVRVersionReconcile() error {
	// get the latest cvr object
	err := obj.CVR.Get(obj.Name, obj.Namespace)
	if err != nil {
		return err
	}
	// waiting for the current version to be equal to desired version
	for obj.CVR.Object.VersionDetails.Status.Current != obj.To {
		klog.Infof("Verifying the reconciliation of version for %s", obj.CVR.Object.Name)
		// Sleep equal to the default sync time
		time.Sleep(10 * time.Second)
		err = obj.CVR.Get(obj.Name, obj.Namespace)
		if err != nil {
			return err
		}
		if obj.CVR.Object.VersionDetails.Status.Message != "" {
			klog.Errorf("failed to reconcile: %s", obj.CVR.Object.VersionDetails.Status.Reason)
		}
	}
	return nil
}

func (obj *CVRPatch) verifyCSPIVersion() error {
	cspName := obj.CVR.Object.Labels["cstorpoolinstance.openebs.io/name"]
	if cspName == "" {
		return errors.Errorf("missing cspi label for cvr %s", obj.Name)
	}
	cspiObj, err := obj.OpenebsClientset.CstorV1().CStorPoolInstances(obj.Namespace).
		Get(context.TODO(), cspName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get cspi %s", cspName)
	}
	if cspiObj.Labels["openebs.io/version"] != obj.To {
		return errors.Errorf(
			"cspi %s not in %s version",
			cspiObj.Name,
			obj.To,
		)
	}
	return nil
}

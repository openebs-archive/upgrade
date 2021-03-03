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

package patch

import (
	"context"
	"strings"

	apis "github.com/openebs/api/v2/pkg/apis/cstor/v1"
	clientset "github.com/openebs/api/v2/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

// CVR ...
type CVR struct {
	Object *apis.CStorVolumeReplica
	Data   []byte
	Client clientset.Interface
}

// CVROptions ...
type CVROptions func(*CVR)

// NewCVR ...
func NewCVR(opts ...CVROptions) *CVR {
	obj := &CVR{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// WithCVRClient ...
func WithCVRClient(c clientset.Interface) CVROptions {
	return func(obj *CVR) {
		obj.Client = c
	}
}

// PreChecks ...
func (c *CVR) PreChecks(from, to string) error {
	if c.Object == nil {
		return errors.Errorf("nil cvr object")
	}
	version := strings.Split(c.Object.VersionDetails.Status.Current, "-")[0]
	if version != strings.Split(from, "-")[0] && version != strings.Split(to, "-")[0] {
		return errors.Errorf(
			"cvr version %s is neither %s nor %s",
			c.Object.VersionDetails.Status.Current,
			from,
			to,
		)
	}
	return nil
}

// Patch ...
func (c *CVR) Patch(from, to string) error {
	klog.Info("patching cvr ", c.Object.Name)
	version := c.Object.VersionDetails.Desired
	if version == to {
		klog.Infof("cvr already in %s version", to)
		return nil
	}
	if version == from {
		patch := c.Data
		_, err := c.Client.CstorV1().CStorVolumeReplicas(c.Object.Namespace).Patch(
			context.TODO(),
			c.Object.Name,
			types.MergePatchType,
			[]byte(patch),
			metav1.PatchOptions{},
		)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to patch cvr %s",
				c.Object.Name,
			)
		}
		klog.Infof("cvr %s patched", c.Object.Name)
	}
	return nil
}

// Get ...
func (c *CVR) Get(name, namespace string) error {
	cvrObj, err := c.Client.CstorV1().CStorVolumeReplicas(namespace).
		Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get cvr %s in %s namespace", name, namespace)
	}
	c.Object = cvrObj
	return nil
}

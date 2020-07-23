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
	"strings"

	apis "github.com/openebs/api/pkg/apis/cstor/v1"
	clientset "github.com/openebs/api/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

// CV ...
type CV struct {
	Object *apis.CStorVolume
	Data   []byte
	Client clientset.Interface
}

// CVOptions ...
type CVOptions func(*CV)

// NewCV ...
func NewCV(opts ...CVOptions) *CV {
	obj := &CV{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// WithCVClient ...
func WithCVClient(c clientset.Interface) CVOptions {
	return func(obj *CV) {
		obj.Client = c
	}
}

// PreChecks ...
func (c *CV) PreChecks(from, to string) error {
	if c.Object == nil {
		return errors.Errorf("nil cv object")
	}
	version := strings.Split(c.Object.VersionDetails.Status.Current, "-")[0]
	if version != strings.Split(from, "-")[0] && version != strings.Split(to, "-")[0] {
		return errors.Errorf(
			"cv version %s is neither %s nor %s",
			version,
			from,
			to,
		)
	}
	return nil
}

// Patch ...
func (c *CV) Patch(from, to string) error {
	klog.Info("patching cv ", c.Object.Name)
	version := c.Object.VersionDetails.Desired
	if version == to {
		klog.Infof("cv already in %s version", to)
		return nil
	}
	if version == from {
		patch := c.Data
		_, err := c.Client.CstorV1().CStorVolumes(c.Object.Namespace).Patch(
			c.Object.Name,
			types.MergePatchType,
			[]byte(patch),
		)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to patch cv %s",
				c.Object.Name,
			)
		}
		klog.Infof("cv %s patched", c.Object.Name)
	}
	return nil
}

// Get ...
func (c *CV) Get(name, namespace string) error {
	cvObj, err := c.Client.CstorV1().CStorVolumes(namespace).
		Get(name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get cv %s in %s namespace", name, namespace)
	}
	c.Object = cvObj
	return nil
}

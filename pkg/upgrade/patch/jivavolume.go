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

	// clientset "github.com/openebs/api/v2/pkg/client/clientset/versioned"
	jv "github.com/openebs/jiva-operator/pkg/apis/openebs/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// JV ...
type JV struct {
	Object    *jv.JivaVolume
	NewObject *jv.JivaVolume
	Client    client.Client
}

// JVOptions ...
type JVOptions func(*JV)

// NewJV ...
func NewJV(opts ...JVOptions) *JV {
	obj := &JV{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// WithJVClient ...
func WithJVClient(c client.Client) JVOptions {
	return func(obj *JV) {
		obj.Client = c
	}
}

// PreChecks ...
func (j *JV) PreChecks(from, to string) error {
	if j.Object == nil {
		return errors.Errorf("nil jv object")
	}
	version := strings.Split(j.Object.VersionDetails.Status.Current, "-")[0]
	if version != strings.Split(from, "-")[0] && version != strings.Split(to, "-")[0] {
		return errors.Errorf(
			"jv version %s is neither %s nor %s",
			j.Object.VersionDetails.Status.Current,
			from,
			to,
		)
	}
	return nil
}

// Patch ...
func (j *JV) Patch(from, to string) error {
	klog.Info("patching jv ", j.Object.Name)
	version := j.Object.VersionDetails.Desired
	if version == to {
		klog.Infof("jv already in %s version", to)
		return nil
	}
	if version == from {
		patch := client.MergeFrom(j.Object)
		err := j.Client.Patch(
			context.TODO(),
			j.NewObject,
			patch,
		)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to patch jv %s",
				j.Object.Name,
			)
		}
		klog.Infof("jv %s patched", j.Object.Name)
	}
	return nil
}

func (j *JV) Get(name, namespace string) error {
	instance := &jv.JivaVolume{}
	if err := j.Client.Get(context.TODO(),
		types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, instance); err != nil {
		return errors.Wrapf(err, "failed to get jv %s in %s namespace", name, namespace)
	}

	// update cr with the latest change
	j.Object = instance.DeepCopy()
	return nil
}

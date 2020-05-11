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
	"time"

	deploy "github.com/openebs/maya/pkg/kubernetes/deployment/appsv1/v1alpha1"
	retry "github.com/openebs/maya/pkg/util/retry"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

// Deployment ...
type Deployment struct {
	Object *appsv1.Deployment
	Data   []byte
	Client kubernetes.Interface
}

// DeploymentOptions ...
type DeploymentOptions func(*Deployment)

// NewDeployment ...
func NewDeployment(opts ...DeploymentOptions) *Deployment {
	obj := &Deployment{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// WithDeploymentClient ...
func WithDeploymentClient(c kubernetes.Interface) DeploymentOptions {
	return func(obj *Deployment) {
		obj.Client = c
	}
}

// PreChecks ...
func (d *Deployment) PreChecks(from, to string) error {
	if d.Object == nil {
		return errors.Errorf("nil deployment object")
	}
	version := d.Object.Labels["openebs.io/version"]
	if version != from && version != to {
		return errors.Errorf(
			"deployment version %s is neither %s nor %s",
			version,
			from,
			to,
		)
	}
	return nil
}

// Patch ...
func (d *Deployment) Patch(from, to string) error {
	klog.Info("patching deployment ", d.Object.Name)
	version := d.Object.Labels["openebs.io/version"]
	if version == to {
		klog.Infof("deployment already in %s version", to)
		return nil
	}
	if version == from {
		_, err := d.Client.AppsV1().Deployments(d.Object.Namespace).Patch(
			d.Object.Name,
			types.StrategicMergePatchType,
			d.Data,
		)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to patch deployment %s",
				d.Object.Name,
			)
		}
		err = retry.
			Times(60).
			Wait(5 * time.Second).
			Try(func(attempt uint) error {
				deloyClient := deploy.NewKubeClient()
				rolloutStatus, err1 := deloyClient.WithNamespace(d.Object.Namespace).
					RolloutStatus(d.Object.Name)
				if err1 != nil {
					return err1
				}
				if !rolloutStatus.IsRolledout {
					return errors.Errorf("failed to rollout: %s", rolloutStatus.Message)
				}
				return nil
			})
		if err != nil {
			return err
		}
		klog.Infof("deployment %s patched successfully", d.Object.Name)
	}
	return nil
}

// Get ...
func (d *Deployment) Get(label, namespace string) error {
	deployments, err := d.Client.AppsV1().Deployments(namespace).List(
		metav1.ListOptions{
			LabelSelector: label,
		},
	)
	if err != nil {
		return errors.Wrapf(err, "failed to get deployment for %s", label)
	}
	if len(deployments.Items) != 1 {
		return errors.Errorf("no deployments found for label: %s in %s namespace", label, namespace)
	}
	d.Object = &deployments.Items[0]
	return nil
}

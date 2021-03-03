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
	"time"

	retry "github.com/openebs/maya/pkg/util/retry"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
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
	version := strings.Split(d.Object.Labels["openebs.io/version"], "-")[0]
	if version != strings.Split(from, "-")[0] && version != strings.Split(to, "-")[0] {
		return errors.Errorf(
			"deployment version %s is neither %s nor %s",
			d.Object.Labels["openebs.io/version"],
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
		deployObj, err := d.Client.AppsV1().Deployments(d.Object.Namespace).Patch(
			context.TODO(),
			d.Object.Name,
			types.StrategicMergePatchType,
			d.Data,
			metav1.PatchOptions{},
		)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to patch deployment %s",
				d.Object.Name,
			)
		}
		revision, err := deploymentutil.Revision(deployObj)
		if err != nil {
			return err
		}
		err = retry.
			Times(60).
			Wait(5 * time.Second).
			Try(func(attempt uint) error {
				deployObj, err1 := d.Client.AppsV1().Deployments(d.Object.Namespace).
					Get(context.TODO(), d.Object.Name, metav1.GetOptions{})
				if err != nil {
					return err1
				}
				statusViewer := DeploymentStatusViewer{}
				msg, rolledOut, err1 := statusViewer.Status(deployObj, revision+1)
				if err1 != nil {
					return err1
				}
				klog.Info("rollout status: ", msg)
				if !rolledOut {
					return errors.Wrapf(err1, "failed to rollout: %s", msg)
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
	deployments, err := d.Client.AppsV1().Deployments(namespace).List(context.TODO(),
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

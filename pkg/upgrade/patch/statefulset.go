/*
Copyright 2020 The OpenEBS Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implies.
See the License for the specific language governing permissions and
limitations under the License.
*/

package patch

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

// StatefulSet ...
type StatefulSet struct {
	Object *appsv1.StatefulSet
	Data   []byte
	Client kubernetes.Interface
}

// StatefulSetOptions ...
type StatefulSetOptions func(*StatefulSet)

// NewStatefulSet ...
func NewStatefulSet(opts ...StatefulSetOptions) *StatefulSet {
	obj := &StatefulSet{}
	for _, o := range opts {
		o(obj)
	}
	return obj
}

// WithStatefulSetClient ...
func WithStatefulSetClient(c kubernetes.Interface) StatefulSetOptions {
	return func(obj *StatefulSet) {
		obj.Client = c
	}
}

// PreChecks ...
func (s *StatefulSet) PreChecks(from, to string) error {
	if s.Object == nil {
		return errors.Errorf("nil statefulset object")
	}
	version := strings.Split(s.Object.Labels["openebs.io/version"], "-")[0]
	if version != strings.Split(from, "-")[0] && version != strings.Split(to, "-")[0] {
		return errors.Errorf(
			"statefulset version %s is neither %s nor %s",
			s.Object.Labels["openebs.io/version"],
			from,
			to,
		)
	}
	return nil
}

// Patch ...
func (s *StatefulSet) Patch(from, to string) error {
	klog.Info("patching statefulset ", s.Object.Name)
	version := s.Object.Labels["openebs.io/version"]
	if version == to {
		klog.Infof("statefulset already in %s version", to)
		return nil
	}
	if version == from {
		_, err := s.Client.AppsV1().StatefulSets(s.Object.Namespace).Patch(
			context.TODO(),
			s.Object.Name,
			types.StrategicMergePatchType,
			s.Data,
			metav1.PatchOptions{},
		)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to patch statefulset %s",
				s.Object.Name,
			)
		}
		for {
			stsObj, err1 := s.Client.AppsV1().StatefulSets(s.Object.Namespace).
				Get(context.TODO(), s.Object.Name, metav1.GetOptions{})
			if err != nil {
				return err1
			}
			statusViewer := StatefulSetStatusViewer{}
			msg, rolledOut, err1 := statusViewer.Status(stsObj)
			if err1 != nil {
				return err1
			}
			klog.Info("rollout status: ", msg)
			if !rolledOut {
				time.Sleep(5 * time.Second)
			} else {
				break
			}
		}
		klog.Infof("statefulset %s patched successfully", s.Object.Name)
	}
	return nil
}

// Get ...
func (s *StatefulSet) Get(label, namespace string) error {
	statefulsets, err := s.Client.AppsV1().StatefulSets(namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: label,
		},
	)
	if err != nil {
		return errors.Wrapf(err, "failed to get statefulset for %s", label)
	}
	if len(statefulsets.Items) != 1 {
		return errors.Errorf("no statefulsets found for label: %s in %s namespace", label, namespace)
	}
	s.Object = &statefulsets.Items[0]
	return nil
}

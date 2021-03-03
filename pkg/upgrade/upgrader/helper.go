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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

func getImageURL(url, prefix string) (string, error) {
	lastIndex := strings.LastIndex(url, ":")
	if lastIndex == -1 {
		return "", errors.Errorf("no version tag found on image %s", url)
	}
	baseImage := url[:lastIndex]
	if prefix != "" {
		// urlPrefix is the url to the directory where the images are present
		// the below logic takes the image name from current baseImage and
		// appends it to the given urlPrefix
		// For example baseImage is abc/quay.io/openebs/jiva
		// and urlPrefix is xyz/aws-56546546/openebsdirectory/
		// it will take jiva from current url and append it to urlPrefix
		// and return xyz/aws-56546546/openebsdirectory/jiva
		urlSubstr := strings.Split(baseImage, "/")
		baseImage = prefix + urlSubstr[len(urlSubstr)-1]
	}
	return baseImage, nil
}

// GetPatchData returns patch data by
// marshalling and taking diff of two objects
func GetPatchData(oldObj, newObj interface{}) ([]byte, error) {
	oldData, err := json.Marshal(oldObj)
	if err != nil {
		return nil, fmt.Errorf("marshal old object failed: %v", err)
	}
	newData, err := json.Marshal(newObj)
	if err != nil {
		return nil, fmt.Errorf("mashal new object failed: %v", err)
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, oldObj)
	if err != nil {
		return nil, fmt.Errorf("CreateTwoWayMergePatch failed: %v", err)
	}
	return patchBytes, nil
}

func isOperatorUpgraded(componentName string, namespace string,
	toVersion string, kubeClient kubernetes.Interface) error {
	operatorPods, err := kubeClient.CoreV1().
		Pods(namespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: "openebs.io/component-name=" + componentName,
		})
	if err != nil {
		return err
	}
	if len(operatorPods.Items) == 0 {
		return fmt.Errorf("operator pod missing for %s", componentName)
	}
	for _, pod := range operatorPods.Items {
		if pod.Labels["openebs.io/version"] != toVersion {
			return fmt.Errorf("%s is in %s version, please upgrade it to %s version",
				componentName, pod.Labels["openebs.io/version"], toVersion)
		}
	}
	return nil
}

// Remove the suffix only if it is present
// at the end of the string
func removeSuffixFromEnd(str, suffix string) string {
	slice := strings.Split(str, "/")
	// if the image is from openebs registry and has a suffix -amd64
	// then only perform the operation
	if slice[len(slice)-2] == "openebs" && strings.HasSuffix(slice[len(slice)-1], "-amd64") {
		i := strings.LastIndex(str, suffix)
		if i > -1 && i == len(str)-len(suffix) {
			str = str[:i]
		}
	}
	return str
}

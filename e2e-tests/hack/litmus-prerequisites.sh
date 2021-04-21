#!/bin/bash
# Copyright 2021 The OpenEBS Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

error_handler()
{
rc=$1; message=$(echo $2 | cut -d "=" -f 2); act=$(echo $3 | cut -d "=" -f 2)
if [ $rc -ne 0 ]; then
  echo "$message"
  if [ "$act" == "exit" ]; then
    exit 1 
  fi
fi
}

default_kube_config_path="$HOME/.kube/config"
read -p "Provide the KUBECONFIG path: [default=$default_kube_config_path] " answer
: ${answer:=$default_kube_config_path}

echo "Selected kubeconfig file: $answer"

echo "Applying the e2e RBAC.."
kubectl apply -f rbac.yaml; retcode=$?
error_handler $retcode msg="Unable to setup e2e RBAC, exiting" action="exit"

echo "Applying the e2e(chaos) experiment result CRDs.."
kubectl apply -f crds.yaml; retcode=$?
error_handler $retcode msg="Unable to create result CRDs, exiting" action="exit"

cp $answer admin.conf; retcode=$?
error_handler $retcode msg="Unable to find the kubeconfig file, exiting" action="exit"

echo "Creating configmap.."
kubectl create configmap kubeconfig --from-file=admin.conf -n e2e; retcode=$?
error_handler $retcode msg="Unable to create kubeconfig configmap, exiting" action="exit"


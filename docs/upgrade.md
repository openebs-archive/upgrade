# Upgrade OpenEBS

This document describes the steps for upgrading the following OpenEBS reources:

- [CSPC pools](#cspc-pools)
- [cStor CSI volumes](#cstor-csi-volumes)

**Note:** 
 - If current version of ndm-operator is 1.12.0 or below and using virtual disks as blockdevices for provisioning cStor pool please refer this [doc](virtual-disk-troubleshoot.md) before proceeding.

## CSPC pools

These instructions will guide you through the process of upgrading cStor CSPC pools from `1.10.0` or later to a newer release up to `2.0.0`.

### Prerequisites

Before upgrading the pools make sure the following prerequisites are taken care of:

 - Upgrade the control plane components by applying the desired version of cstor-operator from the [charts](https://github.com/openebs/charts/tree/gh-pages). You can verify the current version of the control plane using the command:
    ```sh
    $ kubectl -n openebs get pods -l openebs.io/version=<version>
    ```
    where `<version>` is the desired version.
    For example if desired version is `1.12.0` the output should look like:
    ```sh
    $ kubectl -n openebs get pods -l openebs.io/version=1.12.0
    NAME                                              READY   STATUS    RESTARTS   AGE
    cspc-operator-7744bfb75-fj2w8                     1/1     Running   0          6m11s
    cvc-operator-5c6456df79-jpl5c                     1/1     Running   0          6m11s
    openebs-cstor-admission-server-845d78b97d-sgcnh   1/1     Running   0          6m10s
    ```

### Running the upgrade job

To upgrade a CSPC pool a jobs needs to be launched that automates all the steps required. Below is the sample yaml for the job:

```yaml
# This is an example YAML for upgrading cstor CSPC. 
# Some of the values below needs to be changed to
# match your openebs installation. The fields are
# indicated with VERIFY
---
apiVersion: batch/v1
kind: Job
metadata:
  # VERIFY that you have provided a unique name for this upgrade job.
  # The name can be any valid K8s string for name.
  name: cstor-cspc-upgrade

  # VERIFY the value of namespace is same as the namespace where openebs components
  # are installed. You can verify using the command:
  # `kubectl get pods -n <openebs-namespace> -l openebs.io/component-name=maya-apiserver`
  # The above command should return status of the openebs-apiserver.
  namespace: openebs
spec:
  backoffLimit: 4
  template:
    spec:
      # VERIFY the value of serviceAccountName is pointing to service account
      # created within openebs namespace. Use the non-default account.
      # by running `kubectl get sa -n <openebs-namespace>`
      serviceAccountName: openebs-maya-operator
      containers:
      - name:  upgrade
        args:
        - "cstor-cspc"

        # --from-version is the current version of the pool
        - "--from-version=1.10.0"

        # --to-version is the version desired upgrade version
        - "--to-version=1.12.0"
        # if required the image prefix of the pool deployments can be
        # changed using the flag below, defaults to whatever was present on old
        # deployments.
        #- "--to-version-image-prefix=openebs/"
        # if required the image tags for pool deployments can be changed
        # to a custom image tag using the flag below, 
        # defaults to the --to-version mentioned above.
        #- "--to-version-image-tag=ci"

        # VERIFY that you have provided the correct list of CSPC Names
        - "cspc-stripe"

        # Following are optional parameters
        # Log Level
        - "--v=4"
        # DO NOT CHANGE BELOW PARAMETERS
        env:
        - name: OPENEBS_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        tty: true

        # the image version should be same as the --to-version mentioned above
        # in the args of the job
        image: openebs/upgrade-amd64:1.12.0
        imagePullPolicy: IfNotPresent
      restartPolicy: OnFailure
---
```

You can get the above yaml from [here](../examples/upgrade/cstor-cspc.yaml).

To get the CSPC pool names you can use the command:
```sh
$ kubectl -n openebs get cspc
NAME          HEALTHYINSTANCES   PROVISIONEDINSTANCES   DESIREDINSTANCES   AGE
cspc-stripe   1                  1                      1                  3h22m
```

The status of the job can be verified by looking at the logs of the job pod. To get the job pod use the command:
```sh
$ kubectl -n openebs get pods -l job-name=cstor-cspc-upgrade
NAME                               READY   STATUS   RESTARTS   AGE
cstor-cspc-upgrade-2x4bv     1/1     Running    0          34s
```
 
```sh
$ kubectl -n openebs logs -f cstor-cspc-upgrade-2x4bv
I0714 12:37:09.747331       1 cstor_cspc.go:65] Upgrading cspc-stripe to 1.12.0
I0714 12:37:10.062861       1 deployment.go:77] patching deployment cspc-stripe-k7cc
I0714 12:40:11.493424       1 deployment.go:114] deployment cspc-stripe-k7cc patched successfully
I0714 12:40:11.493476       1 cspi.go:73] patching cspi cspc-stripe-k7cc
I0714 12:40:11.503801       1 cspi.go:93] cspi cspc-stripe-k7cc patched
I0714 12:40:11.527764       1 cstor_cspi.go:285] Verifying the reconciliation of version for cspc-stripe-k7cc
I0714 12:40:21.632513       1 cspc.go:75] patching cspc cspc-stripe
I0714 12:40:21.682353       1 cspc.go:95] cspc cspc-stripe patched
I0714 12:40:21.693266       1 cstor_cspc.go:190] Verifying the reconciliation of version for cspc-stripe
I0714 12:40:31.701881       1 cstor_cspc.go:76] Successfully upgraded cspc-stripe to 1.12.0
```

## cStor CSI volumes

These instructions will guide you through the process of upgrading cStor CSI volumes from `1.10.0` or later to a newer release up to `2.0.0`.

### Prerequisites

Before upgrading the volumes make sure the following prerequisites are taken care of:

 - Make sure the CSPC pools are upgraded to desired version using the steps [above](#cspc-pools).
 - Upgrade the cStor csi driver to desired version(same as the cStor CSPC pool) by applying the csi-driver from the [charts](https://github.com/openebs/charts/tree/gh-pages).
  
   **Note:** If the csi-driver is upgraded from 1.12.0 or below then the csi driver sts and deployments are moved to openebs namespace from kube-system namespace. Once the control plane is upgraded remove the old sts and deployments from kube-system namespace.
   ```sh
   $ kubectl -n kube-system delete sts openebs-cstor-csi-controller
   $ kubectl -n kube-system delete ds openebs-cstor-csi-node
   $ kubectl -n kube-system delete sa openebs-cstor-csi-controller-sa,openebs-cstor-csi-node-sa
   ```

 - Check for the `REMOUNT` env in `openebs-cstor-csi-node` daemonset, if disabled then scaling down the application before upgrading the volume is recommended to avoid any read-only issues.

### Running the upgrade job

To upgrade a cStor CSI volume a jobs needs to be launched that automates all the steps required. Below is the sample yaml for the job:

```yaml
# This is an example YAML for upgrading cstor volume.
# Some of the values below needs to be changed to
# match your openebs installation. The fields are
# indicated with VERIFY
---
apiVersion: batch/v1
kind: Job
metadata:
  # VERIFY that you have provided a unique name for this upgrade job.
  # The name can be any valid K8s string for name.
  name: cstor-volume-upgrade

  # VERIFY the value of namespace is same as the namespace where openebs components
  # are installed. You can verify using the command:
  # `kubectl get pods -n <openebs-namespace> -l openebs.io/component-name=maya-apiserver`
  # The above command should return status of the openebs-apiserver.
  namespace: openebs
spec:
  backoffLimit: 4
  template:
    spec:
      # VERIFY the value of serviceAccountName is pointing to service account
      # created within openebs namespace. Use the non-default account.
      # by running `kubectl get sa -n <openebs-namespace>`
      serviceAccountName: openebs-maya-operator
      containers:
      - name:  upgrade
        args:
        - "cstor-volume"

        # --from-version is the current version of the volume
        - "--from-version=1.10.0"

        # --to-version is the version desired upgrade version
        - "--to-version=1.12.0"
        # if required the image prefix of the volume deployments can be
        # changed using the flag below, defaults to whatever was present on old
        # deployments.
        #- "--to-version-image-prefix=openebs/"
        # if required the image tags for volume deployments can be changed
        # to a custom image tag using the flag below, 
        # defaults to the --to-version mentioned above.
        #- "--to-version-image-tag=ci"

        # VERIFY that you have provided the correct list of volume Names
        - "pvc-47f1af68-54fb-462c-b47b-443c267950b0"

        # Following are optional parameters
        # Log Level
        - "--v=4"
        # DO NOT CHANGE BELOW PARAMETERS
        env:
        - name: OPENEBS_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        tty: true

        # the image version should be same as the --to-version mentioned above
        # in the args of the job
        image: openebs/upgrade-amd64:1.12.0
        imagePullPolicy: IfNotPresent
      restartPolicy: OnFailure
---
```
You can get the above yaml from [here](../examples/upgrade/cstor-volume.yaml).

To get the PV names you can use the command:
```sh
$ kubectl get pv
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                        STORAGECLASS        REASON   AGE
pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49   5Gi        RWO            Delete           Bound    default/demo-csi-vol-claim   openebs-cstor-csi            3h19m
```
To identify a cStor CSI volume PV, look for the StorageClass associated with the PV and make sure the StorageClass is having provisioner as `cstor.csi.openebs.io`.

The status of the job can be verified by looking at the logs of the job pod. To get the job pod use the command:
```sh
$ kubectl -n openebs get pods -l job-name=cstor-volume-upgrade
NAME                               READY   STATUS   RESTARTS   AGE
cstor-cspc-upgrade-jd747     1/1     Running    0          34s
```
```sh
$ kubectl -n openebs logs -f cstor-volume-upgrade-jd747
I0714 14:00:53.309707       1 cstor_volume.go:67] Upgrading pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49 to 1.12.0
I0714 14:00:53.818666       1 cvr.go:75] patching cvr pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49-cspc-stripe-k7cc
I0714 14:00:53.863867       1 cvr.go:95] cvr pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49-cspc-stripe-k7cc patched
I0714 14:00:53.923339       1 cstor_cvr.go:138] Verifying the reconciliation of version for pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49-cspc-stripe-k7cc
I0714 14:01:03.935850       1 cstor_cvr.go:138] Verifying the reconciliation of version for pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49-cspc-stripe-k7cc
I0714 14:01:14.021882       1 deployment.go:77] patching deployment pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49-target
I0714 14:03:05.729735       1 deployment.go:114] deployment pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49-target patched successfully
I0714 14:03:05.729787       1 service.go:74] Patching service pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49
I0714 14:03:05.764513       1 service.go:94] Service pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49 patched
I0714 14:03:05.764539       1 cv.go:75] patching cv pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49
I0714 14:03:05.801536       1 cv.go:95] cv pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49 patched
I0714 14:03:05.890751       1 cstor_volume.go:401] Verifying the reconciliation of version for pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49
I0714 14:03:15.897696       1 cvc.go:75] patching cvc pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49
I0714 14:03:15.929871       1 cvc.go:95] cvc pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49 patched
I0714 14:03:16.030782       1 cstor_volume.go:423] Verifying the reconciliation of version for pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49
I0714 14:03:26.046950       1 cstor_volume.go:78] Successfully upgraded pvc-5fdce1bf-2cfc-4692-8353-8bc66deace49 to 1.12.0
```

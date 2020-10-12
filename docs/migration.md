# Migrate cStor pools and volumes from SPC to CSPC 

This document describes the steps for migrating the following OpenEBS cStor custom reources:

- [SPC pools to CSPC pools](#spc-pools-to-cspc-pools)
- [cStor External Provisioned volumes to cStor CSI volumes](#cstor-external-provisioned-volumes-to-cstor-csi-volumes)

**Note:** 
 - If the Kubernetes cluster is on rancher and iscsi is running inside the kubelet container then it is mandatory to install iscsi service on the nodes and add extra binds to the kubelet container as mentioned [here](https://github.com/openebs/cstor-operators/blob/master/docs/troubleshooting/rancher_prerequisite.md).
 - Minimum version of Kubernetes to migrate to CSPC pools / CSI volumes is 1.17.0.
 - If using virtual disks as blockdevices for provisioning cStorpool please refer this [doc](virtual-disk-troubleshoot.md) before proceeding. If you are migrating to OpenEBS 2.2.0 version or above, this step is not mandatory as this step is automated into the job itself.

## SPC pools to CSPC pools

These instructions will guide you through the process of migrating cStor pools from the old v1apha1 SPC spec to v1 CSPC spec. 

### Prerequisites

Before migrating the pools make sure the following prerequisites are taken care of:

 - The current OpenEBS control plane version should be `1.12.0` or above.
    You can use the following command to verify components are in currect version:
    ```
    kubectl get pods -n openebs -l openebs.io/version=<version>
    ```
    For example if the intented version of OpenEBS control plane is `1.12.0`.
    The above command should show that the control plane components are in correct version. The output should look like below:
    ```sh
    $ kubectl get pods -n openebs -l openebs.io/version=1.12.0
    NAME                                           READY   STATUS    RESTARTS   AGE
    maya-apiserver-7b65b8b74f-r7xvv                1/1     Running   0          2m8s
    openebs-admission-server-588b754887-l5krp      1/1     Running   0          2m7s
    openebs-localpv-provisioner-77b965466c-wpfgs   1/1     Running   0          85s
    openebs-ndm-5mzg9                              1/1     Running   0          103s
    openebs-ndm-bmjxx                              1/1     Running   0          107s
    openebs-ndm-operator-5ffdf76bfd-ldxvk          1/1     Running   0          115s
    openebs-ndm-v7vd8                              1/1     Running   0          114s
    openebs-provisioner-678c549559-gh6gm           1/1     Running   0          2m8s
    openebs-snapshot-operator-75dc998946-xdskl     2/2     Running   0          2m6s
    ```
 - The cstor-operator should be installed with version `1.12.0` or above. 
    You can install the correct version of cstor-operator from [charts](https://github.com/openebs/charts/tree/gh-pages). Get the cstor-operator yaml within the correct versioned folder and install.
 - The SPC pool version that needs to migrated should be `1.12.0` or above. 
    You can use the following command to verify the version of SPC. The output should be the current version of SPC. 
    ```sh
    kubectl get spc <spc-name> -o jsonpath="{.versionDetails.status.current}"
    ```
    For example if spc-name is `sparse-claim` then the output will be:
    ```sh
    $ kubectl get spc sparse-claim -o jsonpath="{.versionDetails.status.current}"
    1.12.0
    ```
 - **Also make sure no bad block is present on the nodes before migrating to prevent any  import failures during migration.** To do so list all the CSPs for the SPC and find the nodes on which the CSP is provisioned using the command:
    ```sh
    kubectl get csp -l openebs.io/storage-pool-claim=sparse-claim --show-labels
    NAME                ALLOCATED   FREE    CAPACITY   STATUS    READONLY   TYPE       AGE   LABELS
    sparse-claim-i23o   794K        9.94G   9.94G      Healthy   false      mirrored   30m   kubernetes.io/hostname=127.0.0.1,openebs.io/cas-template-name=cstor-pool-create-default-1.12.0,openebs.io/cas-type=cstor,openebs.io/storage-pool-claim=sparse-claim,openebs.io/version=1.12.0
    ```
    Check for the `kubernetes.io/hostname` and exec into the node. Run the command `fdisk -l`, if the command gets stuck then there is some bad block on the disks attached to the node and migration will fail due to import failures.

### Running the migration job

To migrate a SPC pool a jobs needs to be launched that automates all the steps required. Below is the sample yaml for the job:
```yaml
---
apiVersion: batch/v1
kind: Job
metadata:
  # the name can be of the form migrate-<spc-name>
  name: migrate-spc-sparse-claim
  namespace: openebs
spec:
  backoffLimit: 0
  template:
    spec:
      #VERIFY the value of serviceAccountName is pointing to service account
      # created within openebs namespace. Use the non-default account.
      # by running `kubectl get sa -n <openebs-namespace>`
      serviceAccountName: openebs-maya-operator
      containers:
      - name:  migrate
        args:
        - "cstor-spc"
        # name of the spc that is to be migrated
        - "--spc-name=sparse-claim"
        # optional flag to rename the spc to a specific name
        # - "--cspc-name=sparse-claim-migrated"

        #Following are optional parameters
        #Log Level
        - "--v=4"
        #DO NOT CHANGE BELOW PARAMETERS
        env:
        - name: OPENEBS_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        tty: true
        # the version of the image should be same the 
        # version of cstor-operator installed.
        image: openebs/migrate-amd64:<same-as-cspc-operator>
      restartPolicy: Never
---
```

You can get the above yaml from [here](../examples/migrate/spc-migration.yaml).

The status of the job can be verified by looking at the logs of the job pod. To get the job pod use the command:
```sh
$ kubectl -n openebs get pods -l job-name=migrate-spc-sparse-claim
NAME                               READY   STATUS   RESTARTS   AGE
migrate-spc-sparse-claim-2x4bv     1/1     Running    0          34s
```
```sh
$ k logs -f migrate-spc-sparse-claim-2x4bv
I0520 10:00:53.212255       1 pool.go:73] Migrating spc sparse-claim to cspc
I0520 10:00:53.401079       1 pool.go:334] Updating bdc bdc-17771e05-1c55-43ef-abb3-39dcc36472d4 with cspc labels & finalizer.
I0520 10:00:53.427844       1 pool.go:334] Updating bdc bdc-d9e6b0ff-c951-4081-903e-56bb71900adf with cspc labels & finalizer.
I0520 10:00:53.459935       1 pool.go:102] Creating equivalent cspc for spc sparse-claim
I0520 10:00:59.221476       1 pool.go:367] Updating bdc bdc-17771e05-1c55-43ef-abb3-39dcc36472d4 with cspc ownerRef.
I0520 10:00:59.233294       1 pool.go:367] Updating bdc bdc-d9e6b0ff-c951-4081-903e-56bb71900adf with cspc ownerRef.
I0520 10:00:59.510423       1 pool.go:221] Migrating csp sparse-claim-dzr2 to cspi sparse-claim-mbo2
I0520 10:00:59.510448       1 pool.go:282] Scaling down deployemnt sparse-claim-mbo2
I0520 10:01:25.674512       1 pool.go:395] Updating cvr pvc-9cf5a405-12c0-4522-b031-7816425f443f-sparse-claim-mbo2 with cspi sparse-claim-dzr2 info.
I0520 10:01:25.798889       1 pool.go:80] Successfully migrated spc sparse-claim to cspc
```

## cStor External Provisioned volumes to cStor CSI volumes

These instructions will guide you through the process of migrating cStor volumes from the old v1apha1 external provisioned spec to v1 CSI spec. 

### Prerequisites

Before migrating the volumes make sure the following prerequisites are taken care of:

 - The first two prerequisites for [pool](#spc-pools-to-cspc-pools) are required for volumes as well.
 - The csi-operator should be installed with version 1.12.0 or above. You can install the correct version of csi-operator from [charts](https://github.com/openebs/charts/tree/gh-pages). Get the csi-operator yaml within the correct versioned folder and install. The version should be same as the cstor-operator installed.
 - **The application needs to be scaled down before migrating.** This is required as the PVC and PV spec needs to be modified for migration.
 - If the volume has snapshots then make sure the VolumeSnapshotClass `csi-cstor-snapshotclass` is installed. You can get the VolumeSnapshotClass from [here](https://github.com/openebs/cstor-csi/blob/master/deploy/snapshot-class.yaml).

 ### Running the migration job

To migrate a volume a jobs needs to be launched that automates all the steps required. Below is the sample yaml for the job:

```yaml
---
apiVersion: batch/v1
kind: Job
metadata:
  # the name can be of the form migrate-cstor-<pv-name> 
  name: migrate-cstor-pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d
  namespace: openebs
spec:
  backoffLimit: 4
  template:
    spec:
      #VERIFY the value of serviceAccountName is pointing to service account
      # created within openebs namespace. Use the non-default account.
      # by running `kubectl get sa -n <openebs-namespace>`
      serviceAccountName: openebs-maya-operator
      containers:
      - name:  migrate
        args:
        - "cstor-volume"
        # name of the pv that is to be migrated
        - "--pv-name=pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d"

        #Following are optional parameters
        #Log Level
        - "--v=4"
        #DO NOT CHANGE BELOW PARAMETERS
        env:
        - name: OPENEBS_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        tty: true
        # the version of the image should be same the 
        # version of cstor-operator installed.
        image: openebs/migrate-amd64:<same-as-cvc-operator>
      restartPolicy: Never
---
```

You can get the above yaml from [here](../examples/migrate/cstor-volume-migration.yaml).

The status of the job can be verified by looking at the logs of the job pod. To get the job pod use the command:
```sh
$ kubectl -n openebs get pods -l job-name=migrate-cstor-pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d
NAME                                                             READY   STATUS   RESTARTS   AGE
migrate-cstor-pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d-52w98     1/1     Running    0          34s
```
```sh
$ k logs -f migrate-cstor-pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d-52w98
I0713 12:51:29.090705       1 cstor_volume.go:73] Migrating volume pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d to csi spec
I0713 12:51:29.122973       1 volume.go:807] Checking for a temporary policy of volume pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d
I0713 12:51:29.126303       1 volume.go:817] Creating temporary policy pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d for migration
I0713 12:51:29.264918       1 volume.go:179] Checking volume is not mounted on any application
I0713 12:51:29.326575       1 volume.go:185] Retaining PV to migrate into csi volume
I0713 12:51:29.332122       1 volume.go:528] Updating storageclass openebs-cstor-sparse with csi parameters
I0713 12:51:29.630087       1 volume.go:311] Generating equivalent CSI PVC testclaim-busybox-0
I0713 12:51:29.655155       1 volume.go:245] Recreating equivalent CSI PVC
I0713 12:51:29.838898       1 volume_operations.go:133] Waiting for pvc testclaim-busybox-0 to go away
I0713 12:51:34.879781       1 volume.go:350] Generating equivalent CSI PV pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d
I0713 12:51:34.879807       1 volume.go:285] Recreating equivalent CSI PV
I0713 12:51:34.889090       1 volume_operations.go:150] Waiting for pv pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d to go away
I0713 12:51:40.112757       1 volume.go:215] Creating CVC to bound the volume and trigger CSI driver
I0713 12:51:40.207699       1 volume.go:892] Validating the migrated volume
I0713 12:51:40.296914       1 volume.go:901] Waiting for cvc pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d to become Bound, got: Pending
I0713 12:51:50.309624       1 volume.go:951] Waiting for cv pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d to come to Healthy state, got: 
I0713 12:52:00.319407       1 volume.go:951] Waiting for cv pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d to come to Healthy state, got: 
I0713 12:53:11.175510       1 volume.go:951] Waiting for cv pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d to come to Healthy state, got: Init
I0713 12:53:21.177993       1 volume.go:951] Waiting for cv pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d to come to Healthy state, got: Init
I0713 12:53:31.242181       1 volume.go:956] Patching the target svc with cvc owner ref
I0713 12:53:31.336819       1 volume.go:1029] Cleaning up old volume resources
I0713 12:53:31.714056       1 cstor_volume.go:80] Successfully migrated volume pvc-7ac10812-cc83-4fc5-a2e0-7d24f785e93d, scale up the application to verify the migration
```
**Note:** If target affinity was set to the old volume, the target pod will go into `pending` state after the migration is completed. Once the application is scaled up the target pod should automatically reschedule to the same node as application.

**Note:** For each migrated StorageClass a cStorVolumePolicy is created with the same name as StorageClass during the migration. To configure replica and target affinity for new volumes provisioned using the migrated StorageClass make the below configurations on the cStorVolumePolicy:

#### Replica Affinity

For StatefulSet applications, to distribute single replica volume on separate nodes.

```yaml
apiVersion: cstor.openebs.io/v1
kind: CStorVolumePolicy
metadata:
  name: csi-volume-policy
  namespace: openebs
spec:
  provision:
    replicaAffinity: true
```


#### Volume Target Pod Affinity

The Stateful workloads access the OpenEBS storage volume by connecting to the Volume Target Pod. 
Target Pod Affinity policy can be used to co-locate volume target pod on the same node as workload.
This feature makes use of the Kubernetes Pod Affinity feature that is dependent on the Pod labels. 
User will need to add the following label to both Application and volume Policy.

Configured Policy having target-affinity label for example, using `kubernetes.io/hostname` as a topologyKey in CStorVolumePolicy:

```yaml
apiVersion: cstor.openebs.io/v1
kind: CStorVolumePolicy
metadata:
  name: csi-volume-policy
  namespace: openebs
spec:
  target:
    affinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: openebs.io/target-affinity
            operator: In
            values:
            - fio-cstor                              // application-unique-label
        topologyKey: kubernetes.io/hostname
        namespaces: ["default"]                      // application namespace
```


Set the label configured in volume policy created above `openebs.io/target-affinity: fio-cstor` on the app pod which will be used to find pods, by label, within the domain defined by topologyKey.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: fio-cstor
  namespace: default
  labels:
    name: fio-cstor
    openebs.io/target-affinity: fio-cstor
```

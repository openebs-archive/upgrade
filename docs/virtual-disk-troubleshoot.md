# Troubleshooting Virtual Disk Blockdevices

When using OpenEBS with virtual disks for blockdevices, there can be scenarios where the disk is detached and reattached to the node. This may result in the creation of new blockdevices but the underlying disk remains the same. This issue is fixed in OpenEBS 1.13.0 onwards. Before migrating SPC to CSPC or upgrading OpenEBS from 1.12.0 or below to 1.13.0 or above follow the steps below.

## Steps to update CStor pool with correct blockdevices

- Run the diagnostic [script](verifybds.sh) that will identify if any blockdevice is renamed after reattachment. The script takes `<openebs-namespace>` as the only input.
  ```sh
  $ ./verifybds.sh openebs
  cstor-pool-yfn3
  blockdevice-0089038926179a1b8ca3ab91b9d0e782 --> blockdevice-8e782e6b47ed896a325870fc436e65f9
  blockdevice-0953b46938cfe608d235bb4cae47ff6d --> blockdevice-99596b3a4eb2396b8e45b334daeefccc
  ```
  This will give us the original blockdevice mentioned in the cStor pool and the current renamed blockdevice that needs to be updated in pool for all the CSPs. The SPC name for a given CSP can be found using the command `kubectl get csp --show-labels` and looking for the value of `openebs.io/storage-pool-claim`.

- Claim the renamed blockdevices by using the blow template for blockdeviceclaim. For each renamed blockdevice get output of `kubectl -n <openebs-namespace> get bd <bd-name> --show-labels`
  ```sh
  $ kubectl -n openebs get bd blockdevice-8e782e6b47ed896a325870fc436e65f9 --show-labels 
  NAME                                           NODENAME                                       SIZE          CLAIMSTATE   STATUS   AGE   LABELS
  blockdevice-8e782e6b47ed896a325870fc436e65f9   ip-192-168-20-104.us-east-2.compute.internal   10737418240   Claimed      Active   64m   kubernetes.io/hostname=ip-192-168-20-104,ndm.io/blockdevice-type=blockdevice,ndm.io/managed=true
  ```
  This will help us identify the hostname for the blockdevice from the `kubernetes.io/hostname` label. Fill in the below template and apply it.
  ```yaml
  apiVersion: openebs.io/v1alpha1
  kind: BlockDeviceClaim
  metadata:
    finalizers:
    - storagepoolclaim.openebs.io/finalizer
    labels:
      openebs.io/storage-pool-claim: <spc-name>
    name: bdc-<bd-name>
    namespace: <openebs-namespace>
    ownerReferences:
    - apiVersion: openebs.io/v1alpha1
      blockOwnerDeletion: true
      controller: true
      kind: StoragePoolClaim
      name: <spc-name>
      uid: <spc-uuid>
  spec:
    blockDeviceName: <bd-name>
    blockDeviceNodeAttributes:
      hostName: <host-name>
    hostName: <host-name>
  ```
  Check the BDC status using the command 
  ```sh
  $ kubectl -n <openebs-namespace> get bdc <bdc-name>
  ```
  If the BD is not claimed automatically then check the BD for the `internal.openebs.io/uuid-scheme` annotation. If it is set to `legacy` then
  - add the label `openebs.io/block-device-tag: <spc-name>` and remove the annotation `internal.openebs.io/uuid-scheme`.
  - add the selector below to the BDC created using above yaml.
    ```yaml
    selector:
      matchLabels:
        openebs.io/block-device-tag: <spc-name>
    ```
  This should allow automatic claim for BD for above created BDC.

- Disable reconciliation on SPC. This is required so that the spc controller does not process SPC while we are trying to edit (SPC and CSP) resources. The reconciliation will be diabled for the current SPC only and others SPCs in the system will reconcile as usual.
  ```sh
  $ kubectl edit spc cstor-pool
  apiVersion: openebs.io/v1alpha1
  kind: StoragePoolClaim
  metadata:
    annotations:
      reconcile.openebs.io/disable: "true"
      cas.openebs.io/config: |
        - name: PoolResourceRequests
          value: |-
              memory: 2Gi
        - name: PoolResourceLimits
          value: |-
              memory: 4Gi
    creationTimestamp: "2020-08-03T10:45:22Z"
    finalizers:
    - storagepoolclaim.openebs.io/finalizer
  .......
  .......
  ```
  After adding the above annotation SPC will generate the following events on it. Which means changes made to SPC will not be reconciled until annotation is removed. Verify events using following command
  ```sh
   $ k describe spc cstor-pool 
   Events:
    Type     Reason  Age               From            Message
    ----     ------  ----              ----            -------
    Warning  Update  7s (x2 over 13s)  spc-controller  reconcile is disabled via "reconcile.openebs.io/disable" annotation
  
  ```

- Update the CSP with correct blockdevice details. Replace the old blockdevices with new blockdevices and dev links also needs to be updated.
  ```sh
  $ kubectl edit csp cstor-pool-yfn3 
  apiVersion: openebs.io/v1alpha1
  kind: CStorPool
  metadata:
    annotations:
      openebs.io/csp-lease: '{"holder":"openebs/cstor-pool-yfn3-6bbc79f5fd-9dlmb","leaderTransition":1}'
    creationTimestamp: "2020-08-03T10:45:22Z"
    finalizers:
    - openebs.io/pool-protection
    .....
    .....
  spec:
    group:
    - blockDevice:
      - deviceID: /dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol06591ee2acf1e6419
        inUseByPool: true
        name: blockdevice-8e782e6b47ed896a325870fc436e65f9
    - blockDevice:
      - deviceID: /dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol09dfbbc322ed0647d
        inUseByPool: true
        name: blockdevice-99596b3a4eb2396b8e45b334daeefccc
  
  cstorpool.openebs.io/cstor-pool-yfn3 edited
  ```
  After editing the CSP successfully cross verify whether changes have been saved using following command and it show output as updated blockdevice name
  ```sh
  $ kubectl get csp cstor-pool-yfn3 -o yaml | grep blockdevice
        name: blockdevice-8e782e6b47ed896a325870fc436e65f9
        name: blockdevice-99596b3a4eb2396b8e45b334daeefccc
  ```

- Update the blockdevice entry on SPC. This step is required to make it intact with CSP since we manually updated the blockdevice name in CSP.
  ```sh
  $ kubectl edit spc cstor-pool
  apiVersion: openebs.io/v1alpha1
  kind: StoragePoolClaim
  metadata:
    annotations:
      reconcile.openebs.io/disable: "true"
    creationTimestamp: "2020-08-03T10:45:22Z"
    finalizers:
    - storagepoolclaim.openebs.io/finalizer
    .....
    .....
  spec:
    blockDevices:
      blockDeviceList:
      - blockdevice-8e782e6b47ed896a325870fc436e65f9
      - blockdevice-99596b3a4eb2396b8e45b334daeefccc
  ```
  After editing the SPC we are making sure that CSP and SPC are pointing to the same blockdevices and cStor pool is also running on top of mentioned devices. Verify whether it saved successfully or not using following command
  ```sh
  $ kubectl get spc cstor-pool -o yaml | grep blockdevice-
      - blockdevice-8e782e6b47ed896a325870fc436e65f9
      - blockdevice-99596b3a4eb2396b8e45b334daeefccc
  ```

- Enable the reconciliation on SPC. This step is required to enable back the reconciliation on SPC which was stopped by
adding `reconcile.openebs.io/disable annotation` on SPC. Enabling can be achieved by removing annotation on SPC using `kubectl edit` command. Then remove the annotation `reconcile.openebs.io/disable` which was added in above steps. This will help to reconcile further changes in SPC if made. After removing the annotation there shouldn’t be any more events generated for SPC saying disabled reconciliation.

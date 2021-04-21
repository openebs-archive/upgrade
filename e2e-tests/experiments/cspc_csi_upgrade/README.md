## Test Case to upgrade the cspc csi components

## Description
   - This test case is capable of upgrading the OpenEBS CSPC pool and cStor-csi volume components.
   - First it will verify the CSPC CSI components version to upgrade.
   - This test case upgrades the cstor operator, csi provisioner , cspc pool and csi volumes 
     into new desired version. And verify the status of each components.
   - This test constitutes the below files.
     - cstor-csi-volume-upgrade-job.j2 - CSI volume upgrade job template which has to be populated with the given variables.
     - cstor-cspc-upgrade-job.j2 - CSPC pool upgrade job template which has to be populated with the given variables.
     - test_vars.yml - This test_vars file has the list of test specific variables used in LitmusBook.
     - test.yml - Playbook where the test logic is built to Upgrade the CSPC CSI components.
     - cstor_replica_version_check.yml - This file includes the task to check the cstor volume replicas are upgraded to desired version.
   - This test case should be provided with the parameters in form of job environmental variables in run_litmus_test.yml

## Litmusbook Environment Variables

| Parameters              | Description                                                |
| ----------------------- | ---------------------------------------------------------- |
| FROM_VERSION            | Old version which needs to be upgraded                     |
| TO_VERSION              | New version to upgrade                                     |
| OPERATOR_NS             | Namespace where the openebs is deployed                    |
| CSI_NAMESPACE           | Namespace where OpenEBS csi operator is deployed           |
| UPGRADE_IMAGE_TAG       | Image tag for upgrade job                                  |
| CSPC_POOL_UPGRADE       | Set the value as `true` to Upgrade cspc pool               |
| CSPC_VOLUME_UPGRADE     | Set the value as `true` to Upgrade csi volumes             |
| CSTOR_OPERATOR_UPGRADE  | Set the value as `true` to upgrade the cstor opertor and csi provisioner |

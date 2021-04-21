## Test Case to Migrate the spc non-csi components into cspc csi components

## Description
   - This test case is capable of Migrating the OpenEBS SPC pool and cStor volume components into CSPC pool and CSI volume
   - Before migrating the pool we have to provision the cStor opertor. 
   - For Migrate the volumes applications need to be scaled down
   - This test constitutes the below files.
     - cstor-volume-migration-job.j2 - Volume migration job template which has to be populated with the given variables.
     - cstor-spc-migration-job.j2 - Pool migration job template which has to be populated with the given variables.
     - test_vars.yml - This test_vars file has the list of test specific variables used in LitmusBook.
     - test.yml - Playbook where the test logic is built to migrate the CSPC CSI components.
     - application_scale.yml - This file includes the task to scaledown and scaleup the application.
   - This test case should be provided with the parameters in form of job environmental variables in run_litmus_test.yml

## Litmusbook Environment Variables

| Parameters              | Description                                                |
| ----------------------- | ---------------------------------------------------------- |
| OPERATOR_NS             | Namespace where the openebs is deployed                    |
| Migration_IMAGE_TAG     | Image tag for migration job                                |
| MIGRATE_SPC_POOL        | Set the value as `true` to Migrate spc pool                |
| MIGRATE_CSTOR_VOLUME    | Set the value as `true` to Migrate non-csi volumes         |

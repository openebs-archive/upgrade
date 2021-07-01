# Test Case to deploy the OpenEBS cStor operator.

## Description
   - This test case is capable of setting up OpenEBS cStor cspc,cvc operator and OpenEBS CSI control plane components

   - This test constitutes the below files. 

### run_e2e_test.yml
   - This includes the e2e job which triggers the test execution. The pod includes several environmental variables such as 
        - RELEASE_VERSION: Version Tag for the cstor operator
        - ACTION: Values should be provision for deploy, to remove the operator value should be deprovision.
        - WEBHOOK_FAILURE_POLICY: value for the webhook failure policy.

### test_vars.yml
   - This test_vars file has the list of test specific variables used in E2E test.

### test.yml
   - test.yml is the playbook where the test logic is built to deploy OpenEBS cStor CSPC, CVC operator and OpenEBS CSI control plane components.

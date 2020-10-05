# Upgrade

[![Build Status](https://travis-ci.org/openebs/upgrade.svg?branch=master)](https://travis-ci.org/openebs/upgrade)
[![Go Report](https://goreportcard.com/badge/github.com/openebs/upgrade)](https://goreportcard.com/report/github.com/openebs/upgrade)
[![codecov](https://codecov.io/gh/openebs/upgrade/branch/master/graph/badge.svg)](https://codecov.io/gh/openebs/upgrade)
[![Slack](https://img.shields.io/badge/chat!!!-slack-ff1493.svg?style=flat-square)](https://kubernetes.slack.com/messages/openebs)


Contains components that help with OpenEBS data engine upgrades


## [Upgrading OpenEBS reources](https://github.com/openebs/upgrade/blob/master/docs/upgrade.md)
Below are the steps for upgrading the OpenEBS reources:
- [CSPC pools](https://github.com/openebs/upgrade/blob/master/docs/upgrade.md#cspc-pools)
- [cStor CSI volumes](https://github.com/openebs/upgrade/blob/master/docs/upgrade.md#cstor-csi-volumes)

**Note:** 
 - If current version of ndm-operator is 1.12.0 or below and using virtual disks as blockdevices for provisioning cStor pool please refer this [doc](https://github.com/openebs/upgrade/blob/master/docs/virtual-disk-troubleshoot.md) before proceeding.

## [Migrating cStor pools and volumes from SPC to CSPC](https://github.com/openebs/upgrade/blob/master/docs/migration.md)
Below are the steps for migrating the OpenEBS cStor custom reources:
- [SPC pools to CSPC pools](https://github.com/openebs/upgrade/blob/master/docs/migration.md#spc-pools-to-cspc-pools)
- [cStor External Provisioned volumes to cStor CSI volumes](https://github.com/openebs/upgrade/blob/master/docs/migration.md#cstor-external-provisioned-volumes-to-cstor-csi-volumes)

**Note:** 
 - If the Kubernetes cluster is on rancher and iscsi is running inside the kubelet container then it is mandatory to install iscsi service on the nodes and add extra binds to the kubelet container as mentioned [here](https://github.com/openebs/cstor-operators/blob/master/docs/troubleshooting/rancher_prerequisite.md).
 - Minimum version of Kubernetes to migrate to CSPC pools / CSI volumes is 1.17.0.
 - If using virtual disks as blockdevices for provisioning cStorpool please refer this [doc](https://github.com/openebs/upgrade/blob/master/docs/virtual-disk-troubleshoot.md) before proceeding.


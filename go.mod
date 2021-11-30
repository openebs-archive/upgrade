module github.com/openebs/upgrade

go 1.14

require (
	github.com/google/go-cmp v0.5.4
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.0.0
	github.com/openebs/api/v3 v3.0.0-20211116062351-ecd9a8a61d3e
	github.com/openebs/jiva-operator v1.12.2-0.20211126122511-b8b205d44bfa
	github.com/openebs/maya v1.12.1-0.20210308113344-5c43ada4c9e2
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.1
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.20.2
	sigs.k8s.io/controller-runtime v0.8.2
)

replace k8s.io/client-go => k8s.io/client-go v0.20.2

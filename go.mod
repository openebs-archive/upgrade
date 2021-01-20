module github.com/openebs/upgrade

go 1.14

require (
	github.com/google/go-cmp v0.4.0
	github.com/kubernetes-csi/external-snapshotter/v2 v2.1.1
	github.com/openebs/api/v2 v2.1.0
	github.com/openebs/maya v0.0.0-20200602143918-71415115098d
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v0.17.3
	k8s.io/klog v1.0.0
)


// This is just to test the api PR
// Will remove this after it is merged
replace github.com/openebs/api/v2 => github.com/shubham14bajpai/api/v2 v2.0.0-20210120130135-994b05fdecdf

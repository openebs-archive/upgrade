ns=$1
if [[ $ns == "" ]]; then
    ns="openebs"
fi 
cspList=$(kubectl get csp -o jsonpath='{.items[*].metadata.name}')
bdList=$(kubectl -n $ns get blockdevices -o jsonpath='{.items[*].metadata.name}')

for csp in $cspList
do
        echo "Verifying blockdevices on $csp"
        pod=$(kubectl -n $ns get pods -l openebs.io/cstor-pool=$csp -o jsonpath='{.items[*].metadata.name}')
        devlinks=$(kubectl -n $ns exec -it $pod -c cstor-pool -- zpool status -P | grep dev | awk '{print $1}')
        for devlink in $devlinks
        do
                oldbd=""
                newbd=""
                for bd in $bdList
                do
                        links=$(kubectl -n $ns get blockdevices $bd -o jsonpath="{.spec.devlinks[?(@.kind=='by-id')].links}")
                        links=${links:1:-1}
                        for link in $links
                        do
                                if [[ "$devlink" == *"$link"* ]]; then
                                        state=$(kubectl -n $ns get blockdevices $bd -o jsonpath="{.status.state}")
                                        claimState=$(kubectl -n $ns get blockdevices $bd -o jsonpath="{.status.claimState}")
                                        if [[ $state == "Inactive" && $claimState == "Claimed" ]]; then
                                                oldbd=$bd
                                        elif [[ $state == "Active" && $claimState == "Unclaimed" ]]; then
                                                newbd=$bd
                                        fi
                                fi
                        done
                done
                if [[ $oldbd != "" ]]; then
                        echo "Please update $csp blockdevice from $oldbd --> $newbd by follwoing doc link"
                fi
        done
done

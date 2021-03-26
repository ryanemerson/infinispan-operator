#Change the OCP_HOST variable to point t your cluster
#!/bin/bash -x
#oc login -u kubeadmin -p acSMe-YKYhE-sSUN3-BgAiE  https://${OCP_HOST}:6443
OCP_HOST=upi-0.infinispan.lab.pnq2.cee.redhat.com 
for pvcName in $(oc get pvc --selector="app=infinispan-pod" -o=jsonpath='{.items[?(@.status.phase!="Bound")].metadata.name}')
do
        echo Deleting PVC $pvcName
        oc delete pvc $pvcName
done
for pvName in $(oc get pv --selector="created=ispndev" -o=jsonpath='{.items[?(@.status.phase!="Bound")].metadata.name}')
do
        echo Deleting PV $pvName
        oc delete pv $pvName
done
ssh -i ~/.ssh/quicklab.key -o "UserKnownHostsFile /dev/null" -o "StrictHostKeyChecking no" quicklab@${OCP_HOST} 'sudo firewall-cmd --permanent --add-service=nfs && \
sudo firewall-cmd --permanent --add-service=mountd && \
sudo firewall-cmd --permanent --add-service=rpc-bind && \
sudo firewall-cmd --reload'
ssh -i ~/.ssh/quicklab.key -o "UserKnownHostsFile /dev/null" -o "StrictHostKeyChecking no" quicklab@${OCP_HOST} 'sudo rm -rf /opt/nfs/*'
ssh -i ~/.ssh/quicklab.key -o "UserKnownHostsFile /dev/null" -o "StrictHostKeyChecking no" quicklab@${OCP_HOST} 'sudo bash -c "for i in {1..29} ; do mkdir -p /opt/nfs/pv00\$i ; done"'
ssh -i ~/.ssh/quicklab.key -o "UserKnownHostsFile /dev/null" -o "StrictHostKeyChecking no" quicklab@${OCP_HOST} 'sudo bash -c "rm -f /etc/exports; for i in {1..29} ; do echo /opt/nfs/pv00\$i \*\(no_root_squash,rw,sync\) >> /etc/exports; done"'
ssh -i ~/.ssh/quicklab.key -o "UserKnownHostsFile /dev/null" -o "StrictHostKeyChecking no" quicklab@${OCP_HOST} 'sudo chmod -R 777 /opt/nfs/*'
ssh -i ~/.ssh/quicklab.key -o "UserKnownHostsFile /dev/null" -o "StrictHostKeyChecking no" quicklab@${OCP_HOST} 'sudo bash -c "sudo service nfs restart"'
for i in {1..29}
do
        cat  <<EOF | oc create -f -
apiVersion: v1
kind: PersistentVolume
metadata:
  name: example-$i
  labels:
     created: ispndev
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 5Gi
  nfs:
    path: /opt/nfs/pv00$i
    server: ${OCP_HOST}
  persistentVolumeReclaimPolicy: Retain
  volumeMode: Filesystem
EOF
done


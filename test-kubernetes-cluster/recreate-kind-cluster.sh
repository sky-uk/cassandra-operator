#!/usr/bin/env bash
set -e

scriptDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
reg_name="kind-registry"
reg_port=5000

function setup_volume_on_node() {
    local node=$1
    local zone=$2

    pv_path="/mnt/pv-zone-${zone}"
    kubectl --context kind label --overwrite node ${node} failure-domain.beta.kubernetes.io/zone=eu-west-1${zone}
    docker exec ${node} mkdir -p /data/vol ${pv_path}/bindmount
    docker exec ${node} mount -o bind /data/vol ${pv_path}/bindmount
}

function create_storage_class() {
    # based on https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner/blob/master/provisioner/deployment/kubernetes/example/default_example_storageclass.yaml
    local zone=$1
    cat <<EOF | kubectl --context kind apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: standard-zone-${zone}
provisioner: kubernetes.io/no-provisioner
reclaimPolicy: Delete
EOF
}

function create_test_namespace() {
    local namespace=$1
    kubectl --context kind create ns ${namespace}
    ${scriptDir}/../hack/retry.sh 6 10 kubectl --context kind -n ${namespace} get sa default
}

function deploy_local_volume_provisioner() {
    # based on https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner/blob/master/provisioner/deployment/kubernetes/example/default_example_provisioner_generated.yaml
    local zone=$1
    cat <<EOF | kubectl --context kind apply -f -
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-provisioner-config-${zone}
  namespace: local-volume-provisioning
data:
  storageClassMap: |
    standard-zone-${zone}:
       hostDir: /mnt/pv-zone-${zone}
       mountDir: /mnt/pv-zone-${zone}
       volumeMode: Filesystem
       fsType: ext4
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: local-volume-provisioner-${zone}
  namespace: local-volume-provisioning
spec:
  selector:
    matchLabels:
      app: local-volume-provisioner-${zone}
  template:
    metadata:
      labels:
        app: local-volume-provisioner-${zone}
    spec:
      serviceAccountName: local-storage-admin
      containers:
        - image: "quay.io/external_storage/local-volume-provisioner:v2.3.3"
          name: provisioner
          securityContext:
            privileged: true
          env:
          - name: MY_NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
          - name: MY_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          volumeMounts:
            - mountPath: /etc/provisioner/config
              name: provisioner-config
              readOnly: true
            - mountPath: /mnt/pv-zone-${zone}
              name: pv-zone-${zone}
              mountPropagation: "HostToContainer"
      nodeSelector:
        failure-domain.beta.kubernetes.io/zone: eu-west-1${zone}
      volumes:
        - name: provisioner-config
          configMap:
            name: local-provisioner-config-${zone}
        - name: pv-zone-${zone}
          hostPath:
            path: /mnt/pv-zone-${zone}
EOF
}

function run_local_registry() {
    # local registry so we can build images locally
    runningRegistry=$(docker ps --filter=name="${reg_name}" --format="{{.Names}}")
    if [[ "$runningRegistry" == "" ]]; then
        echo "Running local registry on port ${reg_port}"
        docker run -d --name=${reg_name} --restart=always -p ${reg_port}:${reg_port} registry:2
        echo "Started registry: ${reg_name}"
    fi

    # connect the registry to the cluster network
    # (the network may already be connected)
    docker network connect "kind" "${reg_name}" || true
}

function verify_pod_security_policy_restrictions {
    local context=$1
    local namespace=$2

    trap "kubectl --context ${context} -n ${namespace} delete job policy-test --ignore-not-found" RETURN

    echo "Verifying restricted pod security policy has been applied"
    kubectl --context ${context} -n ${namespace} delete job policy-test --ignore-not-found
    kubectl --context ${context} -n ${namespace} apply -f ${scriptDir}/policy-test-job.yml

    ${scriptDir}/../hack/retry.sh 6 10 ${scriptDir}/expect-container-status.sh ${context} ${namespace} job-name=policy-test CreateContainerConfigError
}

function apply_pod_security_policy_restrictions {
    local context=$1

    docker exec kind-control-plane sed -i "s/--enable-admission-plugins=NodeRestriction/--enable-admission-plugins=NodeRestriction,PodSecurityPolicy/" /etc/kubernetes/manifests/kube-apiserver.yaml
    # give long enough for the change to be noticed and trigger the restart
    sleep 5
    verify_apiserver_accessible

    kubectl --context ${context} apply -f ${scriptDir}/psp-policies.yml
    kubectl --context ${context} apply -f ${scriptDir}/psp-rbac.yml
}

function verify_apiserver_accessible {
    echo "Verifying API server is accessible ..."
    ${scriptDir}/../hack/retry.sh 6 10 kubectl --context kind get ns
}

# pre-requisite
if ! [ -x "$(command -v kind)" ]; then
  echo 'Error: kind is not installed.' >&2
  exit 1
fi

# clean previous cluster if present
kind delete cluster || true
kubectl config delete-context kind || true

# create a cluster
tmpDir=$(mktemp -d)
trap '{ CODE=$?; rm -rf ${tmpDir} ; exit ${CODE}; }' EXIT
cat << EOF > ${tmpDir}/kind-cluster.yml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_name}:${reg_port}"]
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker
  - role: worker
EOF

# node image officially supported for v0.8.1 - see https://github.com/kubernetes-sigs/kind/releases/tag/v0.8.0 for list of supported
KIND_NODE_IMAGE=${KIND_NODE_IMAGE:-"kindest/node:v1.12.10@sha256:faeb82453af2f9373447bb63f50bae02b8020968e0889c7fa308e19b348916cb"}
kind create cluster -v=1 --config ${tmpDir}/kind-cluster.yml --image ${KIND_NODE_IMAGE}
kubectl config rename-context "kind-kind" "kind"

verify_apiserver_accessible

# create test namespace
NAMESPACE=${NAMESPACE:-"test-cassandra-operator"}
create_test_namespace ${NAMESPACE}
create_test_namespace local-volume-provisioning

apply_pod_security_policy_restrictions kind
verify_pod_security_policy_restrictions kind ${NAMESPACE}

# setup local volume provisioning
setup_volume_on_node kind-worker a
setup_volume_on_node kind-worker2 a
setup_volume_on_node kind-worker3 b
setup_volume_on_node kind-worker4 b
create_storage_class a
create_storage_class b

kubectl --context kind apply -f ${scriptDir}/local-volume-provisioner-credentials.yml
deploy_local_volume_provisioner a
deploy_local_volume_provisioner b

# create a local registry that can be used by kind nodes
run_local_registry

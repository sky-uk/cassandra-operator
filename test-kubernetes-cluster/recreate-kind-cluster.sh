#!/usr/bin/env bash
set -e
scriptDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

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
    cat <<EOF | kubectl --context kind apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
EOF
}

function deploy_local_volume_provisioner_credentials() {
    # based on https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner/blob/master/provisioner/deployment/kubernetes/example/default_example_provisioner_generated.yaml
    cat <<EOF | kubectl --context kind apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: local-storage-admin
  namespace: local-volume-provisioning
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: local-storage-provisioner-pv-binding
  namespace: local-volume-provisioning
subjects:
- kind: ServiceAccount
  name: local-storage-admin
  namespace: local-volume-provisioning
roleRef:
  kind: ClusterRole
  name: system:persistent-volume-provisioner
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: local-storage-provisioner-node-clusterrole
  namespace: local-volume-provisioning
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: local-storage-provisioner-node-binding
  namespace: local-volume-provisioning
subjects:
- kind: ServiceAccount
  name: local-storage-admin
  namespace: local-volume-provisioning
roleRef:
  kind: ClusterRole
  name: local-storage-provisioner-node-clusterrole
  apiGroup: rbac.authorization.k8s.io
EOF
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
       blockCleanerCommand:
         - "/scripts/shred.sh"
         - "2"
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
        - image: "quay.io/external_storage/local-volume-provisioner:v2.2.0"
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
    runningRegistry=$(docker ps --filter=name="kind-registry" --format="{{.Names}}")
    if [[ "$runningRegistry" == "" ]]; then
        echo "Running local registry on port 5000"
        docker run -d --name=kind-registry --restart=always -p 5000:5000 registry:2
    fi

    create_registry_proxy
}

# tell kind nodes to proxy TCP requests on port 5000 to the docker host on the same port
# this is so that kind nodes can pull the "local" image from the docker host
# based on https://github.com/kubernetes-sigs/kubeadm-dind-cluster/issues/56#issuecomment-387463386
function create_registry_proxy {
    cat <<EOF | kubectl --context kind apply -f -
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: registry-proxy
  namespace: kube-system
spec:
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app: registry-proxy
  template:
    metadata:
      labels:
        app: registry-proxy
    spec:
      hostNetwork: true
      containers:
        - image: "tecnativa/tcp-proxy"
          name: tcp-proxy
          command: ["/bin/sh", "-c"]
          args:
             - export TALK=\$(/sbin/ip route |  awk '/default/ { print \$3 ":5000"}');
               export LISTEN=:5000;
               /magic-entrypoint /docker-entrypoint.sh haproxy -f /usr/local/etc/haproxy/haproxy.cfg;
          ports:
            - containerPort: 5000
EOF
}

# pre-requisite
if ! [ -x "$(command -v kind)" ]; then
  echo 'Error: kind is not installed.' >&2
  exit 1
fi

# clean previous cluster if present
kind delete cluster || true

# create a cluster
tmpDir=$(mktemp -d)
trap '{ CODE=$?; rm -rf ${tmpDir} ; exit ${CODE}; }' EXIT
cat << EOF > ${tmpDir}/kind-cluster.yml
kind: Cluster
apiVersion: kind.sigs.k8s.io/v1alpha3
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker
  - role: worker
EOF

# node image officially supported for v0.5.1 - see https://github.com/kubernetes-sigs/kind/releases/tag/v0.5.1
KIND_NODE_IMAGE=${KIND_NODE_IMAGE:-"kindest/node:v1.11.10@sha256:bb22258625199ba5e47fb17a8a8a7601e536cd03456b42c1ee32672302b1f909"}
kind create cluster --config ${tmpDir}/kind-cluster.yml --image ${KIND_NODE_IMAGE}
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
kubectl config rename-context "kubernetes-admin@kind" "kind"

# create test namespace
NAMESPACE=${NAMESPACE:-"test-cassandra-operator"}
create_test_namespace ${NAMESPACE}
create_test_namespace local-volume-provisioning

# setup local volume provisioning
setup_volume_on_node kind-worker a
setup_volume_on_node kind-worker2 a
setup_volume_on_node kind-worker3 b
setup_volume_on_node kind-worker4 b
create_storage_class a
create_storage_class b
deploy_local_volume_provisioner_credentials
deploy_local_volume_provisioner a
deploy_local_volume_provisioner b

# create a local registry that can be used by kind nodes
run_local_registry

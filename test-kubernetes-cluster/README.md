# Test Kubernetes cluster

Creates a multi-node Kubernetes cluster based on [Kind](https://kind.sigs.k8s.io/).
The nodes are spread across multiple availability zones: `a` and `b`.
Support for persistent volumes is achieved via local volumes that are lazily created by a static [local provisioner](https://github.com/kubernetes-incubator/external-storage/tree/master/local-volume/provisioner).
Persistent volume claims can target volumes in specific AZ by using the corresponding storage class - e.g. `standard-zone-a`

This Kubernetes cluster is used in our end-to-end tests during CI, but is also very handy when testing locally.   

A local registry is started on port 5000 and Kind setup to allow nodes to pull images from the host.
 
## Usage

```
./recreate-kind-cluster.sh
```

# Changes

## 0.74.0-alpha

- [FEATURE] [Document CRD fields #194](#194) (@sebbonnet)
- [FEATURE] [Default component image version to the operator version](#193) (@sebbonnet)

## 0.73.0-alpha

**Note: This version is not backwards-compatible with previous Cassandra clusters deployed by the operator.**

- [FEATURE] [Allow updates and removal of emptyDir storage](#192) (@sebbonnet) 
- [FEATURE] [Allow multiple rack storages](#190) (@sebbonnet)
- [FEATURE] [Remove `RetentionPolicy.Enabled` field](#189) (@alangibson01)
- [ENHANCEMENT] [Api builders](#188) (@sebbonnet)
- [FEATURE] [Removing UseEmptyDir from Cassandra spec](#185) (@sebbonnet)
- [FEATURE] [Flexible storage type per rack](#184) (@sebbonnet)
- [BUGFIX] [Fix pod cpu resources removal by updating rather than patching StatefulSet](#182) (@sebbonnet)
- [FEATURE] [Change CassandraPod spec to use ResourceRequirements](#178) (@sebbonnet)

## 0.72.0-alpha

This release brings 2 main features, bug fixes and several enhancements.

**Note: This version is not backwards-compatible with previous Cassandra clusters deployed by the operator.
It includes one API change that is a not backward-compatible (#61).**

Main features:
- Reconcile cluster state based on its definition at regular intervals (#53)
- Cassandra sidecar exposing node status endpoints (#30)

Changelog:
- [ENHANCEMENT] [Switch from dind to kind](#176) (@sebbonnet)
- [ENHANCEMENT] [Reduce main.go to bare minimum](#175) (@sebbonnet)
- [FEATURE] [Reconcile partial and external modification](#171) (@sebbonnet)
- [BUGFIX] [Double the number of events per Cassandra](#174) (@sebbonnet)
- [FEATURE] [Expose reconciliation syncPeriod as cmd line arg](#172) (@sebbonnet)
- [FEATURE] [Reconcile by determining current state](#170) (@sebbonnet)
- [ENHANCEMENT] [Use cmd.Equal to compare CassandraSpec](#166) (@sebbonnet)
- [BUGFIX] [Stop creating configmap volume or mount if exists](#165) (@sebbonnet)
- [BUGFIX] [Fix affinity rules for emptydir](#168) (@sebbonnet)
- [BUGFIX] [Generate all client code using code-gen](#163) (@sebbonnet)
- [FEATURE] [controller-runtime based controller](#150) (@kragniz)
- [FEATURE] [Move update validation to the validation package](#156) (@wallrj)
- [ENHANCEMENT] [Upgrade controller tools](#159) (@wallrj)
- [ENHANCEMENT] [A comparison of webhook libraries](#144) (@wallrj)
- [BUGFIX] [Fix intermittent licence check failures](#154) (@sebbonnet)
- [ENHANCEMENT] [Scripts to check and update go.mod files in all go sub-projects](#153) (@wallrj)
- [FEATURE] [Move validation to apis package and use apimachinery Validation helpers](#139) (@wallrj)
- [ENHANCEMENT] [Add caching of gradle dependencies](#142) (@wallrj)
- [ENHANCEMENT] [Use describe table to shorten tests](#140) (@sebbonnet)
- [BUGFIX] [Do not attempt updating cluster on snapshot change](#137) (@sebbonnet)
- [BUGFIX] [Use dynamic informer to handle unmarshalling errors](#125) (@kragniz)
- [FEATURE] [Make Snapshot.RetentionPolicy.Enabled an optional field](#126) (@wallrj)
- [BUGFIX] [Log max 2 events per statefulset](#129) (@sebbonnet)
- [BUGFIX] [Prevent mutating objects we get back from watchers](#127) (@sebbonnet)
- [BUGFIX] [Fix panic when comparing snapshots](#120) (@sebbonnet)
- [FEATURE] [Optional probe fields](#117) (@wallrj)
- [FEATURE] [Cassandra sidecar which responds to liveness and readiness probes](#116) (@wallrj)
- [ENHANCEMENT] [Generate code using controller-gen](#112) (@kragniz)
- [FEATURE] [Add ownerReferences to created resources](#100) (@kragniz)
- [ENHANCEMENT] [Run tests in parallel](#102) (@wallrj)
- [ENHANCEMENT] [Run all static checks and compilation steps before launching the e2e tests](#99) (@wallrj)
- [ENHANCEMENT] [Generate CRD using controller-tools](#94) (@kragniz)
- [FEATURE] [Use pointers for optional fields](#93) (@kragniz)
- [FEATURE] [types: add omitempty for optional fields](#88) (@kragniz)
- [ENHANCEMENT] [Migrate cassandra-operator from dep to go modules #60 ](#86) (@kragniz)
- [FEATURE] [Rename the DC field to Datacenter and make it a string pointer](#61) (@wallrj)
- [ENHANCEMENT] [Add a make check-style target](#68) (@wallrj)

## 0.70.1-alpha
- [BUGFIX] [Cassandra node unable to find seed provider ](https://github.com/sky-uk/cassandra-operator/issues/50)

## 0.70.0-alpha
- [ANNOUNCEMENT] This is the first release available to the general public
  
  The operator is alpha-status software and can be used in development environments.
  It is not yet recommended for use in production environments.      
  
- [FEATURE] New properties to specify docker images for snapshot and bootstrapping
  
  Adds 2 new properties to the cassandra cluster definition: 
  - `pod.bootstrapperImage`: docker image used to bootstrap cassandra pods - see [cassandra-bootstrapper](cassandra-bootstrapper/README.md) 
  - `snapshot.image`: docker image used to trigger snapshot creation and cleanup - see [cassandra-snapshot](cassandra-snapshot/README.md)
  
  These properties default to the latest version of each image.
  They are exposed for transparency and flexibility and mostly intended to be used during upgrades 
  and to allow testing the operator against custom docker images version
  while working on features or improvements.   

## 0.67.0
- [FEATURE] [Scheduled backups of cluster data](https://github.com/sky-uk/cassandra-operator/issues/20)
 
  This change introduces snapshot creation and cleanup by means of cronjobs, 
  and so requires the Operator to have additional permissions to list, create, modify and delete cronjobs.
  For more details refer to [cassandra-snapshot](cassandra-snapshot/README.md)

## 0.66.0
- [BUGFIX] [Wait indefinitely for rack changes](https://github.com/sky-uk/cassandra-operator/issues/19)

  Rather than waiting a fixed time for rack changes to be applied before proceeding to apply changes to subsequent racks,
  the operator will now wait indefinitely for a change to complete. This ensures that for scaling-up operations on
  heavily-loaded clusters, new nodes will not be added until all previously-added nodes have completed initalisation.
  
  This change requires the operator's service account to have the `patch` permission on the `events` resource.

## 0.65.0
- [FEATURE] [Make it possible to upgrade configurer without destroying clusters](https://github.com/sky-uk/cassandra-operator/issues/18)

  **Note: This version is not backwards-compatible with previous Cassandra clusters deployed by the operator.**
   
  This version introduces a major rework in the way that configuration is injected into Cassandra pods. Where previously
  we provided custom-built Cassandra Docker images which baked in code for configuration injection, this is now done by
  a pair of init-containers, which is a more scalable and future-proof means of doing this.
  
  The upshot of this is that the [official Cassandra Docker images](https://hub.docker.com/_/cassandra/) version 3.x can
  be used instead.

## 0.56.0
- [BUGFIX] [Scaling up a rack causes a full rolling update of the underlying StatefulSet](https://github.com/sky-uk/cassandra-operator/issues/17)

## 0.54.0
- [FEATURE] [Allow configurable readiness and liveness check parameters](https://github.com/sky-uk/cassandra-operator/issues/16)

  Note: This change is backwards incompatible with previous configurations which used the `PodLivenessProbeTimeoutSeconds` or `PodReadinessProbeTimeoutSeconds` pod settings.
  These have been removed and replaced with `PodLivenessProbe` and `PodReadinessProbe` which are nested objects which allow the following properties to be set:
  - `FailureThreshold`
  - `InitialDelaySeconds`
  - `PeriodSeconds`
  - `SuccessThreshold`
  - `TimeoutSeconds`
  
  These properties mirror those in Kubernetes' [Probe V1](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#probe-v1-core) and are all optional values.

## 0.48.0
- [FEATURE] [Split CRD bootstrapping out of operator](https://github.com/sky-uk/cassandra-operator/issues/14)

## 0.47.0
- [BUGFIX] [Operator internal state left inconsistent if cluster definition is invalid](https://github.com/sky-uk/cassandra-operator/issues/13)

## 0.42.0
- [BUGFIX] [Changing pod memory updates requests.memory but not limits.memory](https://github.com/sky-uk/cassandra-operator/issues/12)

## 0.41.0
- [BUGFIX] [Remove cluster check functionality from operator](https://github.com/sky-uk/cassandra-operator/issues/21)

## 0.37.0
- [FEATURE] [Apply updates on custom configuration changes](https://github.com/sky-uk/cassandra-operator/issues/5)

## 0.36.0
- [BUGFIX] [Liveness and readiness probes timeout after 1s](https://github.com/sky-uk/cassandra-operator/issues/10)
- [BUGFIX] [Metrics gathering should not attempt to access Jolokia on a pod which doesn't have an IP address](https://github.com/sky-uk/cassandra-operator/22)

## 0.33.0
- [FEATURE] [Apply updates on configuration changes](https://github.com/sky-uk/cassandra-operator/issues/8)

  The following updates will be applied by the operator:
  - Changes to PodMemory and PodCPU values
  - Increase in the number of racks
  - Increase in the number of replicas in a rack

## 0.29.0
- [FEATURE] Introduce `Cassandra` custom resource and change the operator from listening to changes on specially-labelled configmaps 
to listening to changes on a dedicated custom resource, `core.sky.uk/cassandras`, currently set to version `v1alpha1`.

## 0.28.0
- [FEATURE] Allow the Cassandra docker image used in a cluster to be specified in the cluster ConfigMap.

## 0.24.0
- [BUGFIX] [Fetch metrics from a random cluster node each time](https://github.com/sky-uk/cassandra-operator/issues/3)
Fetch node metrics from a random node in the cluster rather than requesting via the service name, 
so we avoid reporting inaccurate status when the target node is unavailable.
      
## 0.23.0
- [BUGFIX] [Remove empty racks](https://github.com/sky-uk/cassandra-operator/issues/2)
So we avoid creating unnecessary StatefulSet and invalid seed nodes.
   
## 0.21.0
- [FEATURE] Added `--allow-empty-dir` flag to store Cassandra data on emptyDir volumes      (intended for test use only)

## 0.18.0
- [BUGFIX] [Metrics collection does not work for multi-clusters](https://github.com/sky-uk/cassandra-operator/issues/1)
Fix to support multiple clusters 

## 0.13.0
- Pre-alpha release that creates a cluster based on a ConfigMap with a limited set of properties 
to configure the number of racks, name and size of the cluster.

package apis

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"k8s.io/apimachinery/pkg/api/resource"

	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CassandraBuilder struct {
	name      string
	namespace string
	spec      *CassandraSpecBuilder
}

type CassandraSpecBuilder struct {
	datacenter *string
	racks      []*RackSpecBuilder
	pod        *PodSpecBuilder
	snapshot   *SnapshotSpecBuilder
}

type PodSpecBuilder struct {
	resources                               *coreV1.ResourceRequirements
	sidecar                                 *v1alpha1.Sidecar
	image                                   *string
	bootstrapperImage                       *string
	cassandraReadinessPeriod                *int32
	cassandraReadinessTimeout               *int32
	cassandraInitialDelay                   *int32
	cassandraReadinessProbeFailureThreshold *int32
	cassandraReadinessProbeSuccessThreshold *int32
	cassandraLivenessPeriod                 *int32
	cassandraLivenessTimeout                *int32
	cassandraLivenessProbeFailureThreshold  *int32
	cassandraLivenessProbeSuccessThreshold  *int32
	env                                     *[]v1alpha1.CassEnvVar
}

type SnapshotSpecBuilder struct {
	retentionPolicy *RetentionPolicySpecBuilder
	timeoutSeconds  *int32
	keyspaces       []string
	schedule        string
	image           *string
	resources       coreV1.ResourceRequirements
}

type RetentionPolicySpecBuilder struct {
	timeoutSeconds      *int32
	cleanupSchedule     string
	retentionPeriodDays *int32
	resources           coreV1.ResourceRequirements
}

type SidecarSpecBuilder struct {
	image     *string
	resources *coreV1.ResourceRequirements
}

type RackSpecBuilder struct {
	rackName string
	replicas int32
	zone     string
	storages []StorageBuilder
}

type EmptyDirStorageBuilder struct {
	path *string
}

type PersistentVolumeStorageBuilder struct {
	path         *string
	storageClass string
	storageSize  string
}

//
// CassandraSpecBuilder
//

func ACassandra() *CassandraBuilder {
	return &CassandraBuilder{}
}

func (cass *CassandraBuilder) WithDefaults() *CassandraBuilder {
	return cass.
		WithName("my-cluster").
		WithNamespace("my-namespace").
		WithSpec(ACassandraSpec().WithDefaults())
}

func (cass *CassandraBuilder) WithSpec(spec *CassandraSpecBuilder) *CassandraBuilder {
	cass.spec = spec
	return cass
}

func (cass *CassandraBuilder) WithName(name string) *CassandraBuilder {
	cass.name = name
	return cass
}

func (cass *CassandraBuilder) WithNamespace(name string) *CassandraBuilder {
	cass.namespace = name
	return cass
}

func (cass *CassandraBuilder) Build() *v1alpha1.Cassandra {
	return &v1alpha1.Cassandra{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      cass.name,
			Namespace: cass.namespace,
		},
		Spec: *cass.spec.Build(),
	}
}

//
// CassandraSpecBuilder
//

func ACassandraSpec() *CassandraSpecBuilder {
	return &CassandraSpecBuilder{}
}

func (cs *CassandraSpecBuilder) WithDatacenter(datacenter string) *CassandraSpecBuilder {
	cs.datacenter = ptr.String(datacenter)
	return cs
}

func (cs *CassandraSpecBuilder) WithPod(pod *PodSpecBuilder) *CassandraSpecBuilder {
	cs.pod = pod
	return cs
}

func (cs *CassandraSpecBuilder) WithSnapshot(snapshot *SnapshotSpecBuilder) *CassandraSpecBuilder {
	cs.snapshot = snapshot
	return cs
}

func (cs *CassandraSpecBuilder) WithRack(rack *RackSpecBuilder) *CassandraSpecBuilder {
	cs.racks = append(cs.racks, rack)
	return cs
}

func (cs *CassandraSpecBuilder) WithRacks(racks ...*RackSpecBuilder) *CassandraSpecBuilder {
	cs.racks = racks
	return cs
}

func (cs *CassandraSpecBuilder) WithNoSnapshot() *CassandraSpecBuilder {
	cs.snapshot = nil
	return cs
}

func (cs *CassandraSpecBuilder) WithDefaults() *CassandraSpecBuilder {
	return cs.WithDatacenter("my-dc").
		WithPod(APod().WithDefaults()).
		WithRack(ARack("a", 1).WithDefaults()).
		WithRack(ARack("b", 1).WithDefaults()).
		WithSnapshot(ASnapshot().WithDefaults())
}

func (cs *CassandraSpecBuilder) Build() *v1alpha1.CassandraSpec {
	var racks []v1alpha1.Rack
	for _, rackBuilder := range cs.racks {
		racks = append(racks, rackBuilder.Build())
	}
	var snapshot *v1alpha1.Snapshot
	if cs.snapshot != nil {
		snapshot = cs.snapshot.Build()
	}
	return &v1alpha1.CassandraSpec{
		Datacenter: cs.datacenter,
		Racks:      racks,
		Pod:        *cs.pod.Build(),
		Snapshot:   snapshot,
	}
}

//
// RetentionPolicySpecBuilder
//

func ARetentionPolicy() *RetentionPolicySpecBuilder {
	return &RetentionPolicySpecBuilder{}
}

func (rp *RetentionPolicySpecBuilder) WithDefaults() *RetentionPolicySpecBuilder {
	return rp.
		WithRetentionPeriodDays(1).
		WithTimeoutSeconds(0).
		WithCleanupScheduled("1 23 * * *").
		WithResources(coreV1.ResourceRequirements{
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("55Mi"),
				coreV1.ResourceCPU:    resource.MustParse("0"),
			},
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("55Mi"),
			},
		})

}

func (rp *RetentionPolicySpecBuilder) WithResources(resources coreV1.ResourceRequirements) *RetentionPolicySpecBuilder {
	rp.resources = resources
	return rp
}

func (rp *RetentionPolicySpecBuilder) WithRetentionPeriodDays(days int32) *RetentionPolicySpecBuilder {
	rp.retentionPeriodDays = ptr.Int32(days)
	return rp
}

func (rp *RetentionPolicySpecBuilder) WithCleanupScheduled(schedule string) *RetentionPolicySpecBuilder {
	rp.cleanupSchedule = schedule
	return rp
}

func (rp *RetentionPolicySpecBuilder) WithTimeoutSeconds(seconds int32) *RetentionPolicySpecBuilder {
	rp.timeoutSeconds = ptr.Int32(seconds)
	return rp
}

func (rp *RetentionPolicySpecBuilder) Build() *v1alpha1.RetentionPolicy {
	return &v1alpha1.RetentionPolicy{
		RetentionPeriodDays:   rp.retentionPeriodDays,
		CleanupTimeoutSeconds: rp.timeoutSeconds,
		CleanupSchedule:       rp.cleanupSchedule,
		Resources:             rp.resources,
	}
}

//
// SnapshotSpecBuilder
//

func ASnapshot() *SnapshotSpecBuilder {
	return &SnapshotSpecBuilder{}
}

func (snap *SnapshotSpecBuilder) WithDefaults() *SnapshotSpecBuilder {
	return snap.
		WithImage("skyuk/cassandra-snapshot:latest").
		WithSchedule("1 23 * * *").
		WithKeyspaces([]string{"keyspace1", "keyspace2"}).
		WithTimeoutSeconds(1).
		WithResources(coreV1.ResourceRequirements{
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("50Mi"),
				coreV1.ResourceCPU:    resource.MustParse("0"),
			},
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("50Mi"),
			},
		})
}

func (snap *SnapshotSpecBuilder) WithResources(resources coreV1.ResourceRequirements) *SnapshotSpecBuilder {
	snap.resources = resources
	return snap
}

func (snap *SnapshotSpecBuilder) WithRetentionPolicy(policy *RetentionPolicySpecBuilder) *SnapshotSpecBuilder {
	snap.retentionPolicy = policy
	return snap
}

func (snap *SnapshotSpecBuilder) WithKeyspaces(keyspaces []string) *SnapshotSpecBuilder {
	snap.keyspaces = keyspaces
	return snap
}

func (snap *SnapshotSpecBuilder) WithSchedule(schedule string) *SnapshotSpecBuilder {
	snap.schedule = schedule
	return snap
}

func (snap *SnapshotSpecBuilder) WithTimeoutSeconds(seconds int32) *SnapshotSpecBuilder {
	snap.timeoutSeconds = ptr.Int32(seconds)
	return snap
}

func (snap *SnapshotSpecBuilder) WithImage(snapshotImage string) *SnapshotSpecBuilder {
	snap.image = ptr.String(snapshotImage)
	return snap
}

func (snap *SnapshotSpecBuilder) Build() *v1alpha1.Snapshot {
	var retentionPolicy *v1alpha1.RetentionPolicy
	if snap.retentionPolicy != nil {
		retentionPolicy = snap.retentionPolicy.Build()
	}
	return &v1alpha1.Snapshot{
		Image:           snap.image,
		Schedule:        snap.schedule,
		Keyspaces:       snap.keyspaces,
		TimeoutSeconds:  snap.timeoutSeconds,
		RetentionPolicy: retentionPolicy,
		Resources:       snap.resources,
	}
}

//
// PodSpecBuilder
//

func APod() *PodSpecBuilder {
	return &PodSpecBuilder{}
}

func (p *PodSpecBuilder) WithDefaults() *PodSpecBuilder {

	var sidecar = ASidecar().WithDefaults().Build()

	return p.
		WithImageName(ptr.String("cassandra:3.11")).
		WithBootstrapperImageName(ptr.String("BootstrapperImage")).
		WithSidecar(sidecar).
		WithResources(&coreV1.ResourceRequirements{
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("1Gi"),
				coreV1.ResourceCPU:    resource.MustParse("0"),
			},
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}).
		WithCassandraLivenessProbeSuccessThreshold(1).
		WithCassandraLivenessPeriod(2).
		WithCassandraInitialDelay(3).
		WithCassandraLivenessProbeFailureThreshold(4).
		WithCassandraLivenessTimeout(4).
		WithCassandraReadinessProbeSuccessThreshold(1).
		WithCassandraReadinessPeriod(2).
		WithCassandraReadinessProbeFailureThreshold(4).
		WithCassandraReadinessTimeout(5).
		WithEnv(&[]v1alpha1.CassEnvVar{
			v1alpha1.CassEnvVar{
				Name:  "defaultEnvName",
				Value: "defaultEnvValue",
			},
			v1alpha1.CassEnvVar{
				Name:  "NODETOOL_ARGS",
				Value: "-DnodetoolArg=testArg",
			},
		})
}

func (p *PodSpecBuilder) WithResources(podResources *coreV1.ResourceRequirements) *PodSpecBuilder {
	p.resources = podResources
	return p
}

func (p *PodSpecBuilder) WithSidecar(sidecar *v1alpha1.Sidecar) *PodSpecBuilder {
	p.sidecar = sidecar
	return p
}

func (p *PodSpecBuilder) WithImageName(image *string) *PodSpecBuilder {
	p.image = image
	return p
}

func (p *PodSpecBuilder) WithBootstrapperImageName(image *string) *PodSpecBuilder {
	p.bootstrapperImage = image
	return p
}

func (p *PodSpecBuilder) WithoutBootstrapperImageName() *PodSpecBuilder {
	return p.WithBootstrapperImageName(nil)
}

func (p *PodSpecBuilder) WithCassandraInitialDelay(delay int32) *PodSpecBuilder {
	p.cassandraInitialDelay = ptr.Int32(delay)
	return p
}

func (p *PodSpecBuilder) WithCassandraLivenessPeriod(period int32) *PodSpecBuilder {
	p.cassandraLivenessPeriod = ptr.Int32(period)
	return p
}

func (p *PodSpecBuilder) WithCassandraLivenessTimeout(timeout int32) *PodSpecBuilder {
	p.cassandraLivenessTimeout = ptr.Int32(timeout)
	return p
}

func (p *PodSpecBuilder) WithCassandraLivenessProbeFailureThreshold(threshold int32) *PodSpecBuilder {
	p.cassandraLivenessProbeFailureThreshold = ptr.Int32(threshold)
	return p
}

func (p *PodSpecBuilder) WithCassandraLivenessProbeSuccessThreshold(threshold int32) *PodSpecBuilder {
	p.cassandraLivenessProbeSuccessThreshold = ptr.Int32(threshold)
	return p
}

func (p *PodSpecBuilder) WithCassandraReadinessPeriod(period int32) *PodSpecBuilder {
	p.cassandraReadinessPeriod = ptr.Int32(period)
	return p
}

func (p *PodSpecBuilder) WithCassandraReadinessTimeout(timeout int32) *PodSpecBuilder {
	p.cassandraReadinessTimeout = ptr.Int32(timeout)
	return p
}

func (p *PodSpecBuilder) WithCassandraReadinessProbeFailureThreshold(threshold int32) *PodSpecBuilder {
	p.cassandraReadinessProbeFailureThreshold = ptr.Int32(threshold)
	return p
}

func (p *PodSpecBuilder) WithCassandraReadinessProbeSuccessThreshold(threshold int32) *PodSpecBuilder {
	p.cassandraReadinessProbeSuccessThreshold = ptr.Int32(threshold)
	return p
}

func (p *PodSpecBuilder) WithEnv(envVars *[]v1alpha1.CassEnvVar) *PodSpecBuilder {
	p.env = envVars
	return p
}

func (p *PodSpecBuilder) Build() *v1alpha1.Pod {
	return &v1alpha1.Pod{
		BootstrapperImage: p.bootstrapperImage,
		Sidecar:           p.sidecar,
		Image:             p.image,
		Resources:         *p.resources,
		LivenessProbe: &v1alpha1.Probe{
			SuccessThreshold:    p.cassandraLivenessProbeSuccessThreshold,
			FailureThreshold:    p.cassandraLivenessProbeFailureThreshold,
			InitialDelaySeconds: p.cassandraInitialDelay,
			PeriodSeconds:       p.cassandraLivenessPeriod,
			TimeoutSeconds:      p.cassandraLivenessTimeout,
		},
		ReadinessProbe: &v1alpha1.Probe{
			SuccessThreshold:    p.cassandraReadinessProbeSuccessThreshold,
			FailureThreshold:    p.cassandraReadinessProbeFailureThreshold,
			InitialDelaySeconds: p.cassandraInitialDelay,
			PeriodSeconds:       p.cassandraReadinessPeriod,
			TimeoutSeconds:      p.cassandraReadinessTimeout,
		},
		Env: p.env,
	}
}

//
// SidecarBuilder
//

func ASidecar() *SidecarSpecBuilder {
	return &SidecarSpecBuilder{}
}

func (s *SidecarSpecBuilder) WithDefaults() *SidecarSpecBuilder {
	s.image = ptr.String("SidecarImage")
	s.resources = &coreV1.ResourceRequirements{
		Limits: coreV1.ResourceList{
			coreV1.ResourceMemory: resource.MustParse("50Mi"),
		},
		Requests: coreV1.ResourceList{
			coreV1.ResourceMemory: resource.MustParse("50Mi"),
			coreV1.ResourceCPU:    resource.MustParse("0"),
		},
	}
	return s
}

func (s *SidecarSpecBuilder) WithSidecarImageName(image *string) *SidecarSpecBuilder {
	s.image = image
	return s
}

func (s *SidecarSpecBuilder) WithResources(resources *coreV1.ResourceRequirements) *SidecarSpecBuilder {
	s.resources = resources
	return s
}

func (s *SidecarSpecBuilder) WithoutSidecarImageName() *SidecarSpecBuilder {
	return s.WithSidecarImageName(nil)
}

func (s *SidecarSpecBuilder) Build() *v1alpha1.Sidecar {
	return &v1alpha1.Sidecar{
		Image:     s.image,
		Resources: *s.resources,
	}
}

//
// StorageBuilder
//

type StorageBuilder interface {
	Build() v1alpha1.Storage
}

//
// PersistentVolumeStorageBuilder
//

func APersistentVolume() *PersistentVolumeStorageBuilder {
	return &PersistentVolumeStorageBuilder{}
}

func (pv *PersistentVolumeStorageBuilder) AtPath(path string) *PersistentVolumeStorageBuilder {
	pv.path = ptr.String(path)
	return pv
}

func (pv *PersistentVolumeStorageBuilder) WithStorageClass(storageClass string) *PersistentVolumeStorageBuilder {
	pv.storageClass = storageClass
	return pv
}

func (pv *PersistentVolumeStorageBuilder) OfSize(storageSize string) *PersistentVolumeStorageBuilder {
	pv.storageSize = storageSize
	return pv
}

func (pv *PersistentVolumeStorageBuilder) WithDefaults() *PersistentVolumeStorageBuilder {
	return pv.AtPath("/var/lib/cassandra").
		OfSize("1Gi")
}

func (pv *PersistentVolumeStorageBuilder) Build() v1alpha1.Storage {
	return v1alpha1.Storage{
		Path: pv.path,
		StorageSource: v1alpha1.StorageSource{
			PersistentVolumeClaim: &coreV1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.String(pv.storageClass),
				Resources: coreV1.ResourceRequirements{
					Requests: coreV1.ResourceList{
						coreV1.ResourceStorage: resource.MustParse(pv.storageSize),
					},
				},
			},
		},
	}
}

//
// EmptyDirStorageBuilder
//

func AnEmptyDir() *EmptyDirStorageBuilder {
	return &EmptyDirStorageBuilder{}
}

func (e *EmptyDirStorageBuilder) AtPath(path string) *EmptyDirStorageBuilder {
	e.path = ptr.String(path)
	return e
}

func (e *EmptyDirStorageBuilder) WithDefaults() *EmptyDirStorageBuilder {
	return e
}

func (e *EmptyDirStorageBuilder) Build() v1alpha1.Storage {
	return v1alpha1.Storage{
		Path: e.path,
		StorageSource: v1alpha1.StorageSource{
			EmptyDir: &coreV1.EmptyDirVolumeSource{},
		},
	}
}

//
// RackSpecBuilder
//

func ARack(name string, replicas int32) *RackSpecBuilder {
	return &RackSpecBuilder{rackName: name, replicas: replicas}
}

func (r *RackSpecBuilder) WithDefaults() *RackSpecBuilder {
	return r.
		WithZone(fmt.Sprintf("%s%s", "eu-west-1", r.rackName)).
		WithStorage(
			APersistentVolume().
				WithDefaults().
				WithStorageClass(fmt.Sprintf("%s%s", "standard-zone-", r.rackName)))
}

func (r *RackSpecBuilder) WithEmptyDir() *RackSpecBuilder {
	return r.WithStorage(AnEmptyDir().WithDefaults())
}

func (r *RackSpecBuilder) WithPersistentVolume() *RackSpecBuilder {
	return r.WithStorage(APersistentVolume().WithDefaults())
}

func (r *RackSpecBuilder) WithZone(zone string) *RackSpecBuilder {
	r.zone = zone
	return r
}

func (r *RackSpecBuilder) WithStorage(builder StorageBuilder) *RackSpecBuilder {
	r.storages = append(r.storages, builder)
	return r
}

func (r *RackSpecBuilder) WithStorages(builders ...StorageBuilder) *RackSpecBuilder {
	r.storages = builders
	return r
}

func (r *RackSpecBuilder) Build() v1alpha1.Rack {
	var storages []v1alpha1.Storage
	for _, builder := range r.storages {
		storages = append(storages, builder.Build())
	}
	return v1alpha1.Rack{
		Name:     r.rackName,
		Replicas: r.replicas,
		Zone:     r.zone,
		Storage:  storages,
	}
}

package validation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/robfig/cron"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
)

// ValidateCassandra checks that all required fields are supplied and that they have valid values
// NB ObjectMeta is not validated here;
// apiVersion, kind and metadata, are all validated by the API server implicitly.
// See https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#specifying-a-structural-schema
func ValidateCassandra(c *v1alpha1.Cassandra) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = append(allErrs, validateCassandraSpec(c, field.NewPath("spec"))...)
	return allErrs
}

func validateCassandraSpec(c *v1alpha1.Cassandra, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = append(allErrs, validateRacks(c, fldPath.Child("Racks"))...)
	allErrs = append(allErrs, validatePodResources(c, fldPath.Child("Pod"))...)
	allErrs = append(allErrs, validateSnapshot(c, fldPath.Child("Snapshot"))...)
	return allErrs
}

func validateRacks(c *v1alpha1.Cassandra, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if len(c.Spec.Racks) == 0 {
		allErrs = append(
			allErrs,
			field.Required(
				fldPath,
				"",
			),
		)
		return allErrs
	}

	for _, rack := range c.Spec.Racks {
		rackFieldPath := fldPath.Child(rack.Name)
		allErrs = validateUnsignedInt(allErrs, rackFieldPath.Child("Replicas"), rack.Replicas, 1)
		if len(rack.Storage) == 0 {
			allErrs = append(
				allErrs,
				field.Required(
					rackFieldPath.Child("Storage"),
					"at least one storage is required",
				),
			)
		}

		rackStoragePaths := make(map[string]bool)
		for i := range rack.Storage {
			storage := rack.Storage[i]
			storageFieldPath := rackFieldPath.Child("Storage").Index(i)
			if storage.Path == nil {
				allErrs = append(
					allErrs,
					field.Required(
						storageFieldPath.Child("Path"),
						"a volume path is required",
					),
				)
			} else {
				if v1alpha1helpers.IsAReservedVolumePath(*storage.Path) {
					allErrs = append(
						allErrs,
						field.Forbidden(
							storageFieldPath,
							fmt.Sprintf("Storage path '%s' is reserved for the operator", *storage.Path),
						),
					)
				}
				if _, ok := rackStoragePaths[*storage.Path]; ok {
					allErrs = append(
						allErrs,
						field.Forbidden(
							rackFieldPath.Child("Storage"),
							fmt.Sprintf("multiple storages have the same path '%s'", *storage.Path),
						),
					)
				} else {
					rackStoragePaths[*storage.Path] = true
				}
			}

			if storage.PersistentVolumeClaim != nil && storage.EmptyDir != nil {
				allErrs = append(
					allErrs,
					field.Forbidden(
						storageFieldPath,
						"only one storage source per storage is allowed",
					),
				)
			}
			if storage.PersistentVolumeClaim == nil && storage.EmptyDir == nil {
				allErrs = append(
					allErrs,
					field.Required(
						storageFieldPath,
						"one storage source is required",
					),
				)
			}
			if storage.PersistentVolumeClaim != nil {
				if _, ok := storage.PersistentVolumeClaim.Resources.Requests[coreV1.ResourceStorage]; !ok {
					allErrs = append(
						allErrs,
						field.Required(
							storageFieldPath.Child("persistentVolumeClaim.resources[storage]"),
							"a storage size is required",
						),
					)
				}
			}
		}
	}
	return allErrs
}

func validatePodResources(c *v1alpha1.Cassandra, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if c.Spec.Pod.Resources.Requests.Memory().IsZero() {
		allErrs = append(
			allErrs,
			field.Invalid(
				fldPath.Child("Resources.Requests.Memory"),
				c.Spec.Pod.Resources.Requests.Memory().String(),
				"must be > 0",
			),
		)
	}
	if c.Spec.Pod.Resources.Limits.Memory().IsZero() {
		allErrs = append(
			allErrs,
			field.Invalid(
				fldPath.Child("Resources.Limits.Memory"),
				c.Spec.Pod.Resources.Limits.Memory().String(),
				"must be > 0",
			),
		)
	}
	if isRequestGreaterThanLimit(c.Spec.Pod.Resources, coreV1.ResourceMemory) {
		allErrs = append(
			allErrs,
			field.Invalid(
				fldPath.Child("Resources.Requests.Memory"),
				c.Spec.Pod.Resources.Requests.Memory().String(),
				fmt.Sprintf("must not be greater than spec.Pod.Resources.Limits.Memory (%s)", c.Spec.Pod.Resources.Limits.Memory().String()),
			),
		)
	}
	if isRequestGreaterThanLimit(c.Spec.Pod.Resources, coreV1.ResourceCPU) {
		allErrs = append(
			allErrs,
			field.Invalid(
				fldPath.Child("Resources.Requests.Cpu"),
				c.Spec.Pod.Resources.Requests.Cpu().String(),
				fmt.Sprintf("must not be greater than spec.Pod.Resources.Limits.Cpu (%s)", c.Spec.Pod.Resources.Limits.Cpu().String()),
			),
		)
	}

	allErrs = append(
		allErrs,
		validateLivenessProbe(c.Spec.Pod.LivenessProbe, fldPath.Child("LivenessProbe"))...,
	)
	allErrs = append(
		allErrs,
		validateReadinessProbe(c.Spec.Pod.ReadinessProbe, fldPath.Child("ReadinessProbe"))...,
	)
	return allErrs
}

func isRequestGreaterThanLimit(resources coreV1.ResourceRequirements, resourceName coreV1.ResourceName) bool {
	if request, ok := resources.Requests[resourceName]; ok {
		if limit, ok := resources.Limits[resourceName]; ok {
			return request.Cmp(limit) > 0
		}
	}
	return false
}

func validateUnsignedInt(allErrs field.ErrorList, fldPath *field.Path, value int32, min int32) field.ErrorList {
	if value < min {
		allErrs = append(
			allErrs,
			field.Invalid(
				fldPath,
				value,
				fmt.Sprintf("must be >= %d", min),
			),
		)
	}
	return allErrs
}

// validateLivenessProbe wraps `validateProbe` and filters out the results for `SuccessThreshold`,
// instead performing a LivenessProbe specific check, to ensure that the value is always 1.
// This is explained in the Kubernetes API docs as follows:
//   Minimum consecutive successes for the probe to be considered successful after having failed.
//   Defaults to 1. Must be 1 for liveness. Minimum value is 1.
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#probe-v1-core
func validateLivenessProbe(probe *v1alpha1.Probe, fldPath *field.Path) field.ErrorList {
	allErrs := validateProbe(probe, fldPath)
	successThresholdFieldPath := fldPath.Child("SuccessThreshold")
	allErrs = allErrs.Filter(func(e error) bool {
		fieldErr, ok := e.(*field.Error)
		if ok && fieldErr.Field == successThresholdFieldPath.String() {
			return true
		}
		return false
	})
	if *probe.SuccessThreshold != 1 {
		allErrs = append(
			allErrs,
			field.Invalid(
				successThresholdFieldPath,
				*probe.SuccessThreshold,
				"must be 1",
			),
		)
	}
	return allErrs
}

func validateReadinessProbe(probe *v1alpha1.Probe, fldPath *field.Path) field.ErrorList {
	return validateProbe(probe, fldPath)
}

func validateProbe(probe *v1alpha1.Probe, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = validateUnsignedInt(allErrs, fldPath.Child("FailureThreshold"), *probe.FailureThreshold, 1)
	allErrs = validateUnsignedInt(allErrs, fldPath.Child("InitialDelaySeconds"), *probe.InitialDelaySeconds, 0)
	allErrs = validateUnsignedInt(allErrs, fldPath.Child("PeriodSeconds"), *probe.PeriodSeconds, 1)
	allErrs = validateUnsignedInt(allErrs, fldPath.Child("SuccessThreshold"), *probe.SuccessThreshold, 1)
	allErrs = validateUnsignedInt(allErrs, fldPath.Child("TimeoutSeconds"), *probe.TimeoutSeconds, 1)
	return allErrs
}

func validateSnapshot(c *v1alpha1.Cassandra, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if c.Spec.Snapshot == nil {
		return allErrs
	}
	if _, err := cron.Parse(c.Spec.Snapshot.Schedule); err != nil {
		allErrs = append(
			allErrs,
			field.Invalid(
				fldPath.Child("Schedule"),
				c.Spec.Snapshot.Schedule,
				fmt.Sprintf(
					"is not a valid cron expression (%s)",
					err,
				),
			),
		)
	}
	allErrs = validateUnsignedInt(allErrs, fldPath.Child("TimeoutSeconds"), *c.Spec.Snapshot.TimeoutSeconds, 1)
	if c.Spec.Snapshot.RetentionPolicy != nil {
		allErrs = append(
			allErrs,
			validateSnapshotRetentionPolicy(c, fldPath.Child("RetentionPolicy"))...,
		)
	}
	return allErrs
}

func validateSnapshotRetentionPolicy(c *v1alpha1.Cassandra, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = validateUnsignedInt(allErrs, fldPath.Child("RetentionPeriodDays"), *c.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays, 0)
	allErrs = validateUnsignedInt(allErrs, fldPath.Child("CleanupTimeoutSeconds"), *c.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds, 0)
	if _, err := cron.Parse(c.Spec.Snapshot.RetentionPolicy.CleanupSchedule); err != nil {
		allErrs = append(
			allErrs,
			field.Invalid(
				fldPath.Child("CleanupSchedule"),
				c.Spec.Snapshot.RetentionPolicy.CleanupSchedule,
				fmt.Sprintf(
					"is not a valid cron expression (%s)",
					err,
				),
			),
		)
	}
	return allErrs
}

// ValidateCassandraUpdate checks that only supported changes have been made to a Cassandra resource.
// Calls ValidateCassandra to perform structural validation of the new Cassandra object first,
// to ensure that all fields are have compatible values.
func ValidateCassandraUpdate(old, new *v1alpha1.Cassandra) field.ErrorList {
	allErrs := ValidateCassandra(new)
	if err := allErrs.ToAggregate(); err != nil {
		return allErrs
	}
	fldPath := field.NewPath("spec")

	if *old.Spec.Datacenter != *new.Spec.Datacenter {
		allErrs = append(
			allErrs,
			field.Forbidden(
				fldPath.Child("Datacenter"),
				fmt.Sprintf(
					"This field can not be changed: current: %q, new: %q",
					*old.Spec.Datacenter,
					*new.Spec.Datacenter,
				),
			),
		)
	}

	allErrs = append(
		allErrs,
		validatePodUpdate(fldPath.Child("Pod"), &old.Spec.Pod, &new.Spec.Pod)...,
	)

	_, matchedRacks, removedRacks := v1alpha1helpers.MatchRacks(&old.Spec, &new.Spec)

	removedRackNames := sets.NewString()
	for _, r := range removedRacks {
		removedRackNames.Insert(r.Name)
	}
	if removedRackNames.Len() > 0 {
		allErrs = append(
			allErrs,
			field.Forbidden(
				fldPath.Child("Racks"),
				fmt.Sprintf(
					"Rack deletion is not supported. Missing Racks: %s",
					strings.Join(removedRackNames.List(), ", "),
				),
			),
		)
	}

	for _, matchedRack := range matchedRacks {
		allErrs = append(
			allErrs,
			validateRackUpdate(
				fldPath.Child("Racks", matchedRack.Old.Name),
				&matchedRack.Old,
				&matchedRack.New,
			)...,
		)
	}

	return allErrs
}

func validatePodUpdate(fldPath *field.Path, old, new *v1alpha1.Pod) field.ErrorList {
	var allErrs field.ErrorList
	if *old.Image != *new.Image {
		allErrs = append(
			allErrs,
			field.Forbidden(
				fldPath.Child("Image"),
				fmt.Sprintf(
					"This field can not be changed: current: %q, new: %q",
					*old.Image,
					*new.Image,
				),
			),
		)
	}
	return allErrs
}

func validateRackUpdate(fldPath *field.Path, old, new *v1alpha1.Rack) field.ErrorList {
	var allErrs field.ErrorList
	matchedStorages, removedStorages := matchStorages(old, new)
	removedStoragePaths := sets.NewString()
	for _, s := range removedStorages {
		removedStoragePaths.Insert(*s.Path)
	}
	if len(removedStorages) > 0 {
		allErrs = append(
			allErrs,
			field.Forbidden(
				fldPath.Child("Storage"),
				fmt.Sprintf(
					"Storage deletion is not supported. Missing Storage at path(s): %s",
					strings.Join(removedStoragePaths.List(), ", "),
				),
			),
		)
	}

	for _, matchedStorage := range matchedStorages {
		// reject all storage updates until we know better
		if !reflect.DeepEqual(matchedStorage.Old, matchedStorage.New) {
			allErrs = append(
				allErrs,
				field.Forbidden(
					fldPath.Child("Storage"),
					fmt.Sprintf(
						"This field can not be changed: current: %v, new: %v",
						matchedStorage.Old,
						matchedStorage.New,
					),
				),
			)
		}
	}

	if new.Zone != old.Zone {
		allErrs = append(
			allErrs,
			field.Forbidden(
				fldPath.Child("Zone"),
				fmt.Sprintf(
					"This field can not be changed: current: %v, new: %v",
					old.Zone,
					new.Zone,
				),
			),
		)
	}

	if new.Replicas < old.Replicas {
		allErrs = append(
			allErrs,
			field.Forbidden(
				fldPath.Child("Replicas"),
				fmt.Sprintf(
					"This field can not be decremented (scale-in is not yet supported): current: %d, new: %d",
					old.Replicas,
					new.Replicas,
				),
			),
		)
	}

	return allErrs
}

type matchedStorage struct {
	Old v1alpha1.Storage
	New v1alpha1.Storage
}

func matchStorages(oldRack, newRack *v1alpha1.Rack) (matchedStorages []matchedStorage, removedStorages []v1alpha1.Storage) {
	for _, oldStorage := range oldRack.Storage {
		if foundStorage, ok := findStorage(&oldStorage, newRack.Storage); ok {
			matchedStorages = append(matchedStorages, matchedStorage{oldStorage, *foundStorage})
		} else {
			removedStorages = append(removedStorages, oldStorage)
		}
	}
	return matchedStorages, removedStorages
}

func findStorage(storageToFind *v1alpha1.Storage, storages []v1alpha1.Storage) (*v1alpha1.Storage, bool) {
	for _, storage := range storages {
		if *storage.Path == *storageToFind.Path {
			return &storage, true
		}
	}
	return nil, false
}

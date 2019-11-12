package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations/adjuster"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
)

type statefulSetAccessor struct {
	clusterAccessor cluster.Accessor
}

func (h *statefulSetAccessor) registerStatefulSets(c *cluster.Cluster, configMap *v1.ConfigMap) error {
	for _, rack := range c.Racks() {
		if err := h.registerStatefulSet(c, &rack, configMap); err != nil {
			return err
		}
	}

	return nil
}

func (h *statefulSetAccessor) registerStatefulSet(c *cluster.Cluster, rack *v1alpha1.Rack, customConfigMap *v1.ConfigMap) error {
	statefulSet, err := h.clusterAccessor.CreateStatefulSetForRack(c, rack, customConfigMap)
	if err != nil {
		return fmt.Errorf("error while creating stateful set rack %s for cluster %s.%s: %v", rack.Name, c.Namespace(), c.Name(), err)
	}
	log.Infof("Stateful set created for cluster : %s in rack: %s", c.QualifiedName(), rack.Name)

	if err = h.clusterAccessor.WaitUntilRackChangeApplied(c, statefulSet); err != nil {
		if err == cluster.ErrReconciliationInterrupted {
			return err
		}

		log.Warnf("%v: subsequent stateful sets will still be created but some pods may restart", err)
	}

	return nil
}

func (h *statefulSetAccessor) updateStatefulSet(c *cluster.Cluster, customConfigMap *v1.ConfigMap, rack *v1alpha1.Rack, doUpdate func(*v1beta2.StatefulSet, *v1alpha1.Rack, *v1.ConfigMap)) error {
	log.Infof("Applying update for rack %s in cluster %s", rack.Name, c.QualifiedName())
	statefulSet, err := h.clusterAccessor.GetStatefulSetForRack(c, rack)
	if err != nil {
		return fmt.Errorf("unable to retrieve statefulSet for rack %s: %v. Other racks will not be updated", rack.Name, err)
	}

	doUpdate(statefulSet, rack, customConfigMap)

	updatedStatefulSet, err := h.clusterAccessor.UpdateStatefulSet(c, statefulSet)
	if err != nil {
		return fmt.Errorf("unable to update statefulSet for rack %s: %v. Other racks will not be updated", rack.Name, err)
	}

	err = h.clusterAccessor.WaitUntilRackChangeApplied(c, updatedStatefulSet)
	if err == cluster.ErrReconciliationInterrupted {
		return err
	}

	if err != nil {
		return fmt.Errorf("%v: other racks will not be updated", err)
	}

	return nil
}

func (h *statefulSetAccessor) patchStatefulSet(c *cluster.Cluster, clusterChange *adjuster.ClusterChange) error {
	log.Infof("Applying patch for rack %s in cluster %s: %s", clusterChange.Rack.Name, c.QualifiedName(), clusterChange.Patch)

	updatedStatefulSet, err := h.clusterAccessor.PatchStatefulSet(c, &clusterChange.Rack, clusterChange.Patch)
	if err == cluster.ErrReconciliationInterrupted {
		return err
	}

	if err != nil {
		return fmt.Errorf("unable to update rack %s: %v. Other racks will not be updated", clusterChange.Rack.Name, err)
	}
	return h.clusterAccessor.WaitUntilRackChangeApplied(c, updatedStatefulSet)
}

package cluster

import (
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/batch/v1beta1"
)

type objectReferenceFactory interface {
	newStatefulSetList() *v1beta2.StatefulSetList
	newCronJob() *v1beta1.CronJob
}

type defaultReferenceFactory struct {
}

func (r *defaultReferenceFactory) newStatefulSetList() *v1beta2.StatefulSetList {
	return &v1beta2.StatefulSetList{}
}
func (r *defaultReferenceFactory) newCronJob() *v1beta1.CronJob {
	return &v1beta1.CronJob{}
}

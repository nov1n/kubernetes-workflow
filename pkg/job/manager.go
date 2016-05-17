package job

import (
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sClUnv "k8s.io/kubernetes/pkg/client/unversioned"
)

type JobManager interface {
	AddJob() error
	RemoveJob() error
}

type Manager struct {
	Client    *k8sClUnv.Client
	Namespace string
}

func (man *Manager) AddJob(jobConfig *k8sBatch.Job) (job *k8sBatch.Job, err error) {
	return man.Client.Extensions().Jobs(man.Namespace).Create(jobConfig)
}

func (man *Manager) RemoveJob(jobConfig *k8sBatch.Job) (err error) {
	deleteOptions := &k8sApi.DeleteOptions{}
	return man.Client.Extensions().Jobs(man.Namespace).Delete(jobConfig.GetName(), deleteOptions)
}

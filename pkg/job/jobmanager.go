package job

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

type JobManager interface {
	AddJob() error
	RemoveJob() error
}

type Manager struct {
	Client    *client.Client
	Namespace string
}

func (man *Manager) AddJob(jobConfig *batch.Job) (job *batch.Job, err error) {
	return man.Client.Extensions().Jobs(man.Namespace).Create(jobConfig)
}

func (man *Manager) RemoveJob(jobConfig *batch.Job) (err error) {
	deleteOptions := &api.DeleteOptions{}
	return man.Client.Extensions().Jobs(man.Namespace).Delete(jobConfig.GetName(), deleteOptions)
}

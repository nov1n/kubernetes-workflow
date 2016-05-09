package workflow

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

type JobManagerInterface interface {
	AddJob() error
	RemoveJob() error
}

type JobManager struct {
	Client    *client.Client
	Namespace string
}

func (jm *JobManager) AddJob(jobConfig *batch.Job) (job *batch.Job, err error) {
	return jm.Client.Extensions().Jobs(jm.Namespace).Create(jobConfig)
}

func (jm *JobManager) RemoveJob(jobConfig *batch.Job) (err error) {
	deleteOptions := &api.DeleteOptions{}
	return jm.Client.Extensions().Jobs(jm.Namespace).Delete(jobConfig.GetName(), deleteOptions)
}

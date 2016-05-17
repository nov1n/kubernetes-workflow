/*
Copyright 2016 Nerdalize BV All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

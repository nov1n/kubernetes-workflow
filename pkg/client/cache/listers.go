/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package cache

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sCache "k8s.io/kubernetes/pkg/client/cache"
	k8sLabels "k8s.io/kubernetes/pkg/labels"
)

// StoreToWorkflowLister gives a store List and Exists methods. The store must contain only Workflows.
type StoreToWorkflowLister struct {
	k8sCache.Store
}

func (s *StoreToWorkflowLister) Exists(workflow *api.Workflow) (bool, error) {
	_, exists, err := s.Store.Get(workflow)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// StoreToWorkflowLister lists all workflows in the store.
func (s *StoreToWorkflowLister) List() (workflows api.WorkflowList, err error) {
	for _, c := range s.Store.List() {
		workflows.Items = append(workflows.Items, *(c.(*api.Workflow)))
	}
	return workflows, nil
}

// GetJobWorkflow
func (s *StoreToWorkflowLister) GetJobWorkflows(job *k8sBatch.Job) (workflows []api.Workflow, err error) {
	var selector k8sLabels.Selector
	var workflow api.Workflow

	if len(job.Labels) == 0 {
		err = fmt.Errorf("no workflows found for job %v because it has no labels", job.Name)
		return
	}
	for _, m := range s.Store.List() {
		workflow = *m.(*api.Workflow)
		if workflow.Namespace != job.Namespace {
			continue
		}
		selector, _ = k8sApiUnv.LabelSelectorAsSelector(workflow.Spec.JobsSelector)
		glog.V(3).Infof("WF %v has selector %#v", workflow.Name, selector)
		if selector.Matches(k8sLabels.Set(job.Labels)) {
			workflows = append(workflows, workflow)
			glog.V(3).Infof("Found workflow %v retrieving from job %v", workflow.Name, job.Name)
		}
	}
	if len(workflows) == 0 {
		err = fmt.Errorf("could not find workflows for job %s in namespace %s with labels: %v", job.Name, job.Namespace, job.Labels)
	}
	return
}

// StoreToJobLister gives a store List and Exists methods. The store must contain only Jobs.
type StoreToJobLister struct {
	k8sCache.Store
}

// Exists checks if the given job exists in the store.
func (s *StoreToJobLister) Exists(job *k8sBatch.Job) (bool, error) {
	_, exists, err := s.Store.Get(job)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// StoreToJobLister lists all jobs in the store.
func (s *StoreToJobLister) List() (jobs k8sBatch.JobList, err error) {
	for _, c := range s.Store.List() {
		jobs.Items = append(jobs.Items, *(c.(*k8sBatch.Job)))
	}
	return jobs, nil
}

// Jobs will look like the api pkg/client
func (s *StoreToJobLister) Jobs(namespace string) storeJobsNamespacer {
	return storeJobsNamespacer{s.Store, namespace}
}

type storeJobsNamespacer struct {
	store     k8sCache.Store
	namespace string
}

// List returns a list of jobs fromt he store
func (s storeJobsNamespacer) List(selector k8sLabels.Selector) (jobs k8sBatch.JobList, err error) {
	list := k8sBatch.JobList{}
	for _, m := range s.store.List() {
		job := m.(*k8sBatch.Job)
		if s.namespace == k8sApi.NamespaceAll || s.namespace == job.Namespace {
			if selector.Matches(k8sLabels.Set(job.Labels)) {
				list.Items = append(list.Items, *job)
			}
		}
	}
	return list, nil
}

// GetPodJobs returns a list of jobs managing a pod. Returns an error only if no matching jobs are found.
func (s *StoreToJobLister) GetPodJobs(pod *k8sApi.Pod) (jobs []k8sBatch.Job, err error) {
	var selector k8sLabels.Selector
	var job k8sBatch.Job

	if len(pod.Labels) == 0 {
		err = fmt.Errorf("no jobs found for pod %v because it has no labels", pod.Name)
		return
	}

	for _, m := range s.Store.List() {
		job = *m.(*k8sBatch.Job)
		if job.Namespace != pod.Namespace {
			continue
		}

		selector, _ = k8sApiUnv.LabelSelectorAsSelector(job.Spec.Selector)
		if !selector.Matches(k8sLabels.Set(pod.Labels)) {
			continue
		}
		jobs = append(jobs, job)
	}
	if len(jobs) == 0 {
		err = fmt.Errorf("could not find jobs for pod %s in namespace %s with labels: %v", pod.Name, pod.Namespace, pod.Labels)
	}
	return
}

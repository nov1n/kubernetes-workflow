/*
Copyright 2016 Nerdalize B.V. All rights reserved.
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
		glog.V(4).Infof("Found workflow %v retrieving from job %v", workflow.Name, job.Name)
		if workflow.Namespace != job.Namespace {
			continue
		}
		selector, _ = k8sApiUnv.LabelSelectorAsSelector(job.Spec.Selector)
		if selector.Matches(k8sLabels.Set(job.Labels)) {
			workflows = append(workflows, workflow)
		}
	}
	if len(workflows) == 0 {
		err = fmt.Errorf("could not find workflows for job %s in namespace %s with labels: %v", job.Name, job.Namespace, job.Labels)
	}
	return
}

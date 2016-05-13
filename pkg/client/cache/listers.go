package cache

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	k8sCache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"
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
func (s *StoreToWorkflowLister) GetJobWorkflows(job *extensions.Job) (workflows []api.Workflow, err error) {
	var selector labels.Selector
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
		selector, _ = unversioned.LabelSelectorAsSelector(job.Spec.Selector)
		if selector.Matches(labels.Set(job.Labels)) {
			workflows = append(workflows, workflow)
		}
	}
	if len(workflows) == 0 {
		err = fmt.Errorf("could not find workflows for job %s in namespace %s with labels: %v", job.Name, job.Namespace, job.Labels)
	}
	return
}

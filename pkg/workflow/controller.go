package workflow

import "github.com/nov1n/kubernetes-workflow/pkg/client"
import "github.com/nov1n/kubernetes-workflow/pkg/job"
import "github.com/nov1n/kubernetes-workflow/pkg/api"

const (
	labelKey = "workflow"
)

type WorkflowController struct {
	kubeClient client.ThirdPartyClient
	jobManager job.Manager

	workflow api.Workflow
}

// String returns a string representation for the given WorkflowController
func (wc *WorkflowController) String() (res string) {
	// TODO: String representation of the workflow

	return
}

// StartWorkflow starts a workflow finding schedulable steps and using the
// JobManager to schedule them.
func (wc *WorkflowController) StartWorkflow() (err error) {
	// 1 Validate spec
	// 2 startJobsWithoutDependencies()
	// 3 go watchJobs()

	return
}

// NewWorkflowController creates an instance of a WorkflowController.
func NewWorkflowController(wf api.Workflow, cl *client.ThirdPartyClient, jm *job.Manager) (wfc WorkflowController, err error) {

	return
}

// startJobsWithoutDependencies starts jobs without dependencies.
func (wc *WorkflowController) startJobsWithoutDependencies() (err error) {

	return
}

// validateWorkflow validates a workflow returning an error in case it is invalid.
func validateWorkflow(wf *api.Workflow) (err error) {

	return
}

// watchJobs watches jobs for status change.
func watchJobs(wf *api.Workflow) {
	// for in range
	// select
	// case added --> handler1
	// case modified --> handler2
	// case deleted --> handler3

}
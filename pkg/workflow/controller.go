package workflow

import "github.com/nov1n/kubernetes-workflow/client"
import "github.com/nov1n/kubernetes-workflow/job"

const (
	labelKey = "workflow"
)

type WorkflowController struct {
	kubeClient client.ThirdpartyClient
	jobManager job.Manager

	workflow types.Workflow
}

// String returns a string representation for the given WorkflowController
func (wc *WorkflowController) String() (res string) {
	// TODO: String representation of the workflow
}

// StartWorkflow starts a workflow finding schedulable steps and using the
// JobManager to schedule them.
func (wc *WorkflowController) StartWorkflow() err {
	// 1 Validate spec
	// 2 startJobsWithoutDependencies()
	// 3 go watchJobs()

}

// NewWorkflowController creates an instance of a WorkflowController.
func NewWorkflowController(wf Workflow, cl *client.ThirdpartyClient, jm *job.Manager) (wfc WorkflowController, err error) {

	return
}

// startJobsWithoutDependencies starts jobs without dependencies.
func (wc *WorkflowController) startJobsWithoutDependencies() (err error) {

}

// validateWorkflow validates a workflow returning an error in case it is invalid.
func validateWorkflow(wf *Workflow) (err error) {

}

// watchJobs watches jobs for status change.
func watchJobs(wf *Workflow) {
	// for in range
	// select
	// case added --> handler1
	// case modified --> handler2
	// case deleted --> handler3

}

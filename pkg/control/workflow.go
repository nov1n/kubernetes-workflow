package control

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sRec "k8s.io/kubernetes/pkg/client/record"
)

// WorkflowInterface defines methods for Workflow
type WorkflowInterface interface {
	UpdateWorkflow(*api.Workflow) error
}

// Workflow is the workflow implementation of ControlInterface
type Workflow struct {
	Client   *client.ThirdParty
	Recorder k8sRec.EventRecorder
}

var _ WorkflowInterface = &Workflow{}

// updateWorkflowStatus will try to update a workflow to the server.
// updateWorkflowStatus will retry 'retryOnStatusConflict' times when an update fails.
func (w *Workflow) UpdateWorkflow(workflow *api.Workflow) error {
	for i, rv := 0, workflow.ResourceVersion; ; i++ {
		workflow.ResourceVersion = rv
		_, updateErr := w.Client.Workflows(workflow.Namespace).Update(workflow)
		if updateErr == nil {
			glog.V(2).Infof("Updated status of wf %v successfully", workflow.Name)
			return nil
		}
		if i >= retryOnStatusConflict {
			return fmt.Errorf("tried to update status of wf %v, but amount of retries (%d) exceeded", workflow.Name, retryOnStatusConflict)
		}
		statusErr, ok := updateErr.(*k8sApiErr.StatusError)
		if !ok {
			return fmt.Errorf("tried to update status of wf %v in retry %d/%d, but got error: %v", workflow.Name, i, retryOnStatusConflict, updateErr)
		}
		getWorkflow, getErr := w.Client.Workflows(workflow.Namespace).Get(workflow.Name)
		if getErr != nil {
			return fmt.Errorf("tried to update status of wf %v in retry %d/%d, but got error: %v", workflow.Name, i, retryOnStatusConflict, getErr)
		}
		rv = getWorkflow.ResourceVersion
		glog.V(2).Infof("Tried to update status of wf %v in retry %d/%d, but encountered status error (%v), retrying", workflow.Name, i, retryOnStatusConflict, statusErr)
	}
}

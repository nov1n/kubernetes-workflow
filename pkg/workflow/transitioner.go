package workflow

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/client/cache"
	"github.com/nov1n/kubernetes-workflow/pkg/job"
	"github.com/nov1n/kubernetes-workflow/pkg/validation"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiErr "k8s.io/kubernetes/pkg/api/errors"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sClSetUnv "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/unversioned"
	k8sRec "k8s.io/kubernetes/pkg/client/record"
	k8sCtl "k8s.io/kubernetes/pkg/controller"
	k8sUtRunt "k8s.io/kubernetes/pkg/util/runtime"
)

const (
	workflowValidLabel             = api.Group + "/valid"
	workflowPauseLabel             = api.Group + "/pause"
	recorderComponent              = "workflow-controller"
	requeueAfterStatusConflictTime = 500 * time.Millisecond
	requeueJobstoreNotSyncedTime   = 100 * time.Millisecond
	retryOnStatusConflict          = 3
	falseString                    = "false"
	trueString                     = "true"
)

// Transitioner is responsible for transitioning a workflow from its current
// state towards a desired state.
type Transitioner struct {
	// tpClient is a client for accessing ThirdParty resources.
	tpClient *client.ThirdPartyClient

	// jobControl can be used to Create and Delete jobs in the upstream store.
	jobControl job.ControlInterface

	// To allow injection of updateWorkflowStatus for testing.
	updateHandler func(workflow *api.Workflow) error

	// jobStoreSynced returns true if the jod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	jobStoreSynced func() bool

	// A TTLCache of job creates/deletes
	expectations *WorkflowStepExpectations

	// A store of workflow, populated by the frameworkController
	workflowStore *cache.StoreToWorkflowLister

	// Store of job
	jobStore *cache.StoreToJobLister

	// Recorder records client events
	recorder k8sRec.EventRecorder

	transition func(string) (bool, time.Duration, error)
}

// NewTransitionerFor returns a new Transitioner given a Manager.
func NewTransitionerFor(m *Manager) *Transitioner {
	eventBroadcaster := k8sRec.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	// TODO: remove the wrapper when every clients have moved to use the clientset.
	eventBroadcaster.StartRecordingToSink(&k8sClSetUnv.EventSinkImpl{Interface: m.kubeClient.Core().Events("")})

	t := &Transitioner{
		tpClient: m.tpClient,
		jobControl: job.Control{
			KubeClient: m.kubeClient,
			Recorder:   eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: recorderComponent}),
		},
		jobStoreSynced: m.jobStoreSynced,
		workflowStore:  &m.workflowStore,
		jobStore:       &m.jobStore,
		expectations:   m.expectations,
		recorder:       eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: recorderComponent}),
	}
	t.updateHandler = t.updateWorkflowStatus
	t.transition = t.transitionWorkflow
	return t
}

// pastActiveDeadline checks if workflow has ActiveDeadlineSeconds field set and if it is exceeded.
func pastActiveDeadline(workflow *api.Workflow) bool {
	return false
}

// isWorkflowValid validates a given workflow
func isWorkflowValid(wf *api.Workflow) bool {
	validationErrs := validation.ValidateWorkflow(wf)
	if len(validationErrs) > 0 {
		glog.Errorf("Workflow %v invalid: %v", wf.Name, validationErrs)
		return false
	}
	return true
}

// updateWorkflowStatus will try to update a workflow to the server.
// updateWorkflowStatus will retry 'retryOnStatusConflict' times when an update fails.
func (t *Transitioner) updateWorkflowStatus(workflow *api.Workflow) error {
	for i, rv := 0, workflow.ResourceVersion; ; i++ {
		workflow.ResourceVersion = rv
		_, updateErr := t.tpClient.Workflows(workflow.Namespace).Update(workflow)
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
		getWorkflow, getErr := t.tpClient.Workflows(workflow.Namespace).Get(workflow.Name)
		if getErr != nil {
			return fmt.Errorf("tried to update status of wf %v in retry %d/%d, but got error: %v", workflow.Name, i, retryOnStatusConflict, getErr)
		}
		rv = getWorkflow.ResourceVersion
		glog.V(2).Infof("Tried to update status of wf %v in retry %d/%d, but encountered status error (%v), retrying", workflow.Name, i, retryOnStatusConflict, statusErr)
	}
}

func (t *Transitioner) preconditionsMet(workflow *api.Workflow) bool {
	// Check if the jobStore is synced yet (initialized)
	if !t.jobStoreSynced() {
		glog.V(3).Infof("Waiting for job controller to sync, requeuing workflow %v", workflow.Name)
		return false
	}

	// If the workflow is finished we don't have to do anything
	if workflow.IsFinished() {
		glog.V(3).Infof("Workflow %v is finished, no requeueuing.", workflow.Name)
		return false
	}

	// If a workflow is requested to be paused, don't process it.
	if pause, ok := workflow.Labels[workflowPauseLabel]; ok && pause == trueString {
		glog.V(3).Infof("Workflow %v is paused, no requeueuing.", workflow.Name)
		return false
	}

	// all precondition met
	return true
}

func (t *Transitioner) workflowByKey(key string) (*api.Workflow, bool) {
	// Obtain the workflow object from store by key
	obj, exists, err := t.workflowStore.Store.GetByKey(key)
	if !exists {
		return nil, false
	}
	if err != nil {
		glog.Errorf("Unable to retrieve workflow %v from store: %v. THIS SHOULD NOT BE POSSIBLE.", key, err)
		return nil, false
	}

	workflow := *obj.(*api.Workflow)
	return &workflow, true
}

// Transition transitions a workflow from its current state towards a desired state.
// It's given a key created by k8sController.KeyFunc.
func (t *Transitioner) transitionWorkflow(key string) (requeue bool, requeueAfter time.Duration, err error) {
	glog.V(3).Infoln("Syncing: " + key)
	startTime := time.Now()
	defer func() {
		glog.V(3).Infof("Finished syncing workflow %q (%v)", key, time.Now().Sub(startTime))
	}()

	workflow, exists := t.workflowByKey(key)

	// The workflow is deleted, so delete its expectations.
	if !exists {
		glog.V(3).Infof("Workflow has been deleted: %v", key)
		t.expectations.DeleteExpectations(key)
		return false, 0, nil
	}

	// Check if this workflow can be processed.
	if !t.preconditionsMet(workflow) {
		// Requeue controller when precondition failed because jobStore was not synced.
		if !t.jobStoreSynced() {
			return true, requeueJobstoreNotSyncedTime, nil
		}
		return false, 0, nil
	}

	// Try to schedule suitable steps
	if t.process(workflow) {
		if err := t.updateHandler(workflow); err != nil {
			return true, requeueAfterStatusConflictTime, fmt.Errorf("failed to update workflow %v, requeuing after %v.  Error: %v", workflow.Name, requeueAfterStatusConflictTime, err)
		}
	}

	return false, 0, nil
}

// process a workflow and return whether a status update is needed.
// This method set the defaults for a workflow, validate the workflow and
// process its steps.
func (t *Transitioner) process(workflow *api.Workflow) bool {
	glog.V(3).Infof("Manage workflow %v", workflow.Name)

	// Set defaults for workflow
	if _, ok := workflow.Labels[api.WorkflowUIDLabel]; !ok {
		glog.V(3).Infof("Setting status and UID label for workflow %v", workflow.Name)
		workflow.SetDefaults()
		return true
	}

	needsStatusUpdate := false

	// Check if workflow has been validated or was labeled invalid before.
	if wasValid, exists := workflow.Labels[workflowValidLabel]; !exists || wasValid == falseString {
		isValid := isWorkflowValid(workflow)
		workflow.Labels[workflowValidLabel] = strconv.FormatBool(isValid)

		if !isValid {
			// If the workflow is not valid, the workflow shouldn't be processed so
			// return. If this is the first time the workflow is validated it should
			// be updated to the server, otherwise nothing changed so don't update.
			firstTimeValidated := !exists
			return firstTimeValidated
		}

		// Workflow is valid, continue processing
		needsStatusUpdate = true
	}
	return t.processSteps(workflow) || needsStatusUpdate
}

// processSteps processes the steps for a given workflow and returns whether
// a status update is needed.
func (t *Transitioner) processSteps(workflow *api.Workflow) bool {
	needsStatusUpdate := false
	// Check if workflow has completed
	workflowComplete := true
	for stepName, step := range workflow.Spec.Steps {
		if stepStatus, ok := workflow.Status.Statuses[stepName]; ok && stepStatus.Complete {
			continue // step completed nothing to do
		}
		workflowComplete = false
		switch {
		case step.JobTemplate != nil: // Job step
			needsStatusUpdate = t.processJobStep(workflow, stepName, &step) || needsStatusUpdate
		case step.ExternalRef != nil: // external object reference
			needsStatusUpdate = t.processExternalReferenceStep(workflow, stepName, &step) || needsStatusUpdate
		}
	}
	if workflowComplete {
		glog.V(3).Infof("Setting workflow complete status for workflow %v", workflow.Name)
		workflow.AddCompleteCondition()
		needsStatusUpdate = true
	}
	return needsStatusUpdate
}

// processJobStep processes a job that holds a reference to a job.
// This method will create a new job or update a running job's status, given that
// its dependencies are satisfied.
func (t *Transitioner) processJobStep(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	if !step.DependenciesResolved(workflow) {
		return false
	}

	// all dependencies satisfied (or missing) need action: update or create step
	glog.V(3).Infof("Dependencies satisfied for %v", stepName)

	// fetch job by labelSelector and step
	jobSelector := job.CreateWorkflowJobLabelSelector(workflow, workflow.Spec.Steps[stepName].JobTemplate, stepName)
	glog.V(3).Infof("Selecting jobs using selector %v", jobSelector)
	jobList, err := t.jobStore.Jobs(workflow.Namespace).List(jobSelector)
	if err != nil {
		glog.Errorf("Listing jobs on jobStore returned an error. This should not be possible.")
		return false
	}

	glog.V(3).Infof("Listing jobs for step %v of wf %v resulted in %d items", stepName, workflow.Name, len(jobList.Items))
	switch len(jobList.Items) {
	case 0: // create job
		t.createJobForStep(workflow, stepName, step)
		return false
	case 1: // update status
		return t.updateStepStatus(workflow, stepName, jobList.Items[0])
	default: // reconciliate
		glog.Errorf("WorkflowController.manageWorkfloJob resulted in too many jobs for wf %v. Need reconciliation.", workflow.Name)
		return false
	}
}

// createJobForStep will create a job for a given step.
func (t *Transitioner) createJobForStep(workflow *api.Workflow, stepName string, step *api.WorkflowStep) {
	key, _ := k8sCtl.KeyFunc(workflow)
	// When expectations are not satisfied for this job, we should wait for it
	// to be observed by the informer.
	if !t.expectations.ExpectationsForStepSatisfied(key, stepName) {
		return
	}
	err := t.jobControl.CreateJob(workflow.Namespace, step.JobTemplate, workflow, stepName)
	if err != nil {
		glog.Errorf("Couldn't create job %v in step %v for wf %v", step.JobTemplate.Name, stepName, workflow.Name)
		k8sUtRunt.HandleError(err)
		return
	}
	t.expectations.ExpectCreation(key, stepName)
	glog.V(3).Infof("Created job %v in step %v for wf %v", step.JobTemplate.Name, stepName, workflow.Name)
}

// updateJobForStep will update the status of a step according to the status of
// the job it's holding.
func (t *Transitioner) updateStepStatus(workflow *api.Workflow, stepName string, curJob k8sBatch.Job) bool {
	reference, err := k8sApi.GetReference(&curJob)
	if err != nil || reference == nil {
		glog.Errorf("Unable to get reference from job %v in step %v of wf %v: %v", curJob.Name, stepName, workflow.Name, err)
		return false
	}
	oldStatus, exists := workflow.Status.Statuses[stepName]
	jobFinished := job.IsJobFinished(&curJob)
	if exists && jobFinished == oldStatus.Complete {
		return false
	}
	workflow.Status.Statuses[stepName] = api.WorkflowStepStatus{
		Complete:  jobFinished,
		Reference: *reference,
	}
	glog.V(3).Infof("Updated step status from %v to %v for job %v in step %v for wf %v", oldStatus.Complete,
		workflow.Status.Statuses[stepName].Complete, curJob.Name, stepName, workflow.Name)
	return true

}

// processReference processes as sub dag.
// TODO: Implement this.
func (t *Transitioner) processExternalReferenceStep(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	return false
}

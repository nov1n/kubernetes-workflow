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
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sClSetUnv "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/unversioned"
	k8sRec "k8s.io/kubernetes/pkg/client/record"
)

const (
	workflowValidLabel             = "valid"
	recorderComponent              = "workflow-controller"
	requeueAfterStatusConflictTime = 500 * time.Millisecond
	requeueJobstoreNotSyncedTime   = 100 * time.Millisecond
	retryOnStatusConflict          = 3
	falseString                    = "false"
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

// transitionWorkflow returns a copy of the supplied workflow transitioned
// towards the desired state. It does not carry out the updates.
// This method sets defaults, does validation and processes its steps.
func (t *Transitioner) transitionWorkflow(wf api.Workflow) api.Workflow {
	glog.V(3).Infoln("Transitioning: " + wf.Name)

	startTime := time.Now()
	defer func() {
		glog.V(3).Infof("Finished workflow transition for: %q (%v)", key, time.Now().Sub(startTime))
	}()

	// Set defaults for workflow
	if _, ok := wf.Labels[api.WorkflowUIDLabel]; !ok {
		wf.SetUID()
	}

	// Create empty status map
	if wf.Status.Statuses == nil {
		glog.V(3).Infof("Setting status for workflow %v", wf.Name)
		wf.Status.Statuses = make(map[string]api.WorkflowStepStatus, len(wf.Spec.Steps))
		now := k8sApiUnv.Now()
		wf.Status.StartTime = &now
	}

	// Check if workflow has been validated
	if wasValid, ok := wf.Labels[workflowValidLabel]; !ok || wasValid == falseString {
		// Workflow has either not yet been validated or was invalid

		// Check if workflow is valid
		isValid := strconv.FormatBool(isWorkflowValid(workflow))
		wf.Labels[workflowValidLabel] = isValid

		stillInvalid := wasValid == falseString && isValid == falseString
		if stillInvalid {
			// Workflow is still invalid, no state to update
			// return as we do not want to process an invalid workflow
			return nil, fmt.Errorf("workflow %v is invalid", wf.Name)
		}

		turnedInvalid := isValid == falseString
		if turnedInvalid {
			// Workflow is validated for the first time and
			// is invalid, update status on the server
			return wf, nil
		}
	}

	// Check if workflow has completed
	workflowComplete := true
	for stepName, step := range wf.Spec.Steps {
		if stepStatus, ok := wf.Status.Statuses[stepName]; ok && stepStatus.Complete {
			continue // step completed nothing to do
		}
		workflowComplete = false
		switch {
		case step.JobTemplate != nil: // Job step
			t.processJobStep(&workflow, stepName, &step) // TODO: fix this
			if err != nil {
				return nil, err
			}
		case step.ExternalRef != nil: // external object reference
			t.processExternalReferenceStep(&workflow, stepName, &step)
		}
	}

	if workflowComplete {
		glog.V(3).Infof("Setting workflow complete status for workflow %v", wf.Name)
		now := k8sApiUnv.Now()
		condition := api.WorkflowCondition{
			Type:               api.WorkflowComplete,
			Status:             k8sApi.ConditionTrue,
			LastProbeTime:      now,
			LastTransitionTime: now,
		}
		wf.Status.Conditions = append(wf.Status.Conditions, condition)
		wf.Status.CompletionTime = &now
	}

	return wf, nil
}

// processJobStep processes a step that holds a reference to a job.
// The given workflow object is updated and returned
func (t *Transitioner) processJobStep(wf *api.Workflow, stepName string, step *api.WorkflowStep) error {
	for _, dependencyName := range step.Dependencies {
		dependencyStatus, ok := wf.Status.Statuses[dependencyName]
		if !ok || !dependencyStatus.Complete {
			// Not all dependencies are satisfied
			return
		}
	}

	// all dependencies satisfied (or missing) need action: update or create step
	glog.V(3).Infof("Dependencies satisfied for %v", stepName)

	// fetch job by labelSelector and step
	jobSelector := job.CreateWorkflowJobLabelSelector(workflow, wf.Spec.Steps[stepName].JobTemplate, stepName)
	glog.V(3).Infof("Selecting jobs using selector %v", jobSelector)
	jobList, err := t.jobStore.Jobs(wf.Namespace).List(jobSelector)
	if err != nil {
		//panic("Listing jobs on jobStore returned an error. This should not be possible.")
		return fmt.Errorf("error listing jobs: %v", err)
	}

	glog.V(3).Infof("Listing jobs for step %v of wf %v resulted in %d items", stepName, wf.Name, len(jobList.Items))

	switch len(jobList.Items) {
	case 0: // Add condition running to step
		condition := api.WorkflowStepCondition{
			Type:               api.WorkflowStepRunning,
			Status:             k8sApi.ConditionTrue,
			LastProbeTime:      now,
			LastTransitionTime: now,
		}
		append(wf.Status.Statuses[stepName].Conditions, condition)
		glog.V(3).Infof("Updated workflowstep status to running %v in step %v for wf %v",
			step.JobTemplate.Name, stepName, wf.Name)
	case 1: // update workflowstep conditions
		curJob := jobList.Items[0]
		reference, err := k8sApi.GetReference(&curJob)
		if err != nil || reference == nil {
			glog.Errorf("Unable to get reference from job %v in step %v of wf %v: %v", curJob.Name, stepName, wf.Name, err)
			return
		}

		oldStatus, exists := wf.Status.Statuses[stepName]
		jobFinished := job.IsJobFinished(&curJob)
		if exists && jobFinished == oldStatus.Complete {
			return
		}

		condition := api.WorkflowStepCondition{
			Type:               api.WorkflowStepFinished,
			Status:             k8sApi.ConditionTrue,
			LastProbeTime:      now,
			LastTransitionTime: now,
		}

		append(wf.Status.Statuses[stepName].Conditions, condition)

		glog.V(3).Infof("Updated job status from %v to %v for job %v in step %v for wf %v", oldStatus.Complete,
			wf.Status.Statuses[stepName].Complete, curJob.Name, stepName, wf.Name)
	default: // reconciliate
		glog.Errorf("WorkflowController.manageWorkfloJob resulted in too many jobs for wf %v. Need reconciliation.", wf.Name)
		return
	}
}

// processReference processes as sub dag.
// TODO: Implement this.
func (t *Transitioner) processExternalReferenceStep(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	return false
}

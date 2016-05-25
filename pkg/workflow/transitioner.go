package workflow

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/client/cache"
	"github.com/nov1n/kubernetes-workflow/pkg/controller"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiErr "k8s.io/kubernetes/pkg/api/errors"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sClSet "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	k8sClSetUnv "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/unversioned"
	k8sRec "k8s.io/kubernetes/pkg/client/record"
	k8sCtl "k8s.io/kubernetes/pkg/controller"
	k8sUtRunt "k8s.io/kubernetes/pkg/util/runtime"
)

const (
	workflowUID                    = "workflow-uid"
	requeueAfterStatusConflictTime = 500 * time.Millisecond
	requeueJobstoreNotSyncedTime   = 100 * time.Millisecond
)

type Transitioner struct {
	tpClient   *client.ThirdPartyClient
	jobControl controller.JobControlInterface

	// jobStoreSynced returns true if the jod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	jobStoreSynced func() bool

	// A store of workflow, populated by the frameworkController
	workflowStore *cache.StoreToWorkflowLister

	// Store of job
	jobStore *cache.StoreToJobLister

	// Recorder records client events
	recorder k8sRec.EventRecorder
}

// NewTransitioner returns a new Transitioner
func NewTransitioner(tpClient *client.ThirdPartyClient, kubeClient k8sClSet.Interface,
	jobStoreSynced func() bool, workflowStore *cache.StoreToWorkflowLister, jobStore *cache.StoreToJobLister) *Transitioner {
	eventBroadcaster := k8sRec.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	// TODO: remove the wrapper when every clients have moved to use the clientset.
	eventBroadcaster.StartRecordingToSink(&k8sClSetUnv.EventSinkImpl{Interface: kubeClient.Core().Events("")})

	return &Transitioner{
		tpClient: tpClient,
		jobControl: controller.WorkflowJobControl{
			KubeClient: kubeClient,
			Recorder:   eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: "workflow-controller"}),
		},
		jobStoreSynced: jobStoreSynced,
		workflowStore:  workflowStore,
		jobStore:       jobStore,
		recorder:       eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: "workflow-controller"}),
	}
}

func (t *Transitioner) Transition(key string) (requeue bool, requeueAfter time.Duration, err error) {
	glog.V(3).Infoln("Syncing: " + key)

	startTime := time.Now()
	defer func() {
		glog.V(3).Infof("Finished syncing workflow %q (%v)", key, time.Now().Sub(startTime))
	}()

	// Check if the jobStore is synced yet (initialized)
	if !t.jobStoreSynced() {
		glog.V(3).Infof("Waiting for job controller to sync, requeuing workflow %v", key)
		return true, requeueJobstoreNotSyncedTime, nil
	}

	// Obtain the workflow object from store by key
	obj, exists, err := t.workflowStore.Store.GetByKey(key)
	if !exists {
		glog.V(3).Infof("Workflow has been deleted: %v", key)
		return false, 0, nil
	}
	if err != nil {
		glog.Errorf("Unable to retrieve workflow %v from store: %v", key, err)
		// TODO: Do we really want it to requeue immediately?
		return true, 0, err
	}
	workflow := *obj.(*api.Workflow)
	// Set defaults for workflow
	if _, ok := workflow.Labels[workflowUID]; !ok {
		newWorkflow, err := t.setLabels(&workflow)
		if err != nil {
			serr, ok := err.(*k8sApiErr.StatusError)
			if !ok {
				return true, 0, fmt.Errorf("couldn't set labels on workflow %v: %v", key, err)
			}
			if serr.Status().Code == http.StatusConflict {
				glog.V(3).Infof("encountered status conflict on workflow label update (%v), requeuing after %v", workflow.Name, requeueAfterStatusConflictTime)
				return true, requeueAfterStatusConflictTime, nil
			}
			// TODO We probably want this to be larger than 0.
			return true, 0, serr
		}
		workflow = *newWorkflow
	}

	// If this is the first time syncWorkflow is called and
	// the statuses map is empty, create it
	if workflow.Status.Statuses == nil {
		glog.V(3).Infof("Setting status for workflow %v", workflow.Name)
		workflow.Status.Statuses = make(map[string]api.WorkflowStepStatus, len(workflow.Spec.Steps))
		now := k8sApiUnv.Now()
		workflow.Status.StartTime = &now
	}

	// Try to schedule suitable steps
	if t.manageWorkflow(&workflow) {
		if err := t.updateWorkflow(&workflow); err != nil {
			serr, ok := err.(*k8sApiErr.StatusError)
			if !ok {
				return true, 0, fmt.Errorf("failed to update workflow %v, requeuing.  Error: %v", workflow.Name, err)
			}
			if serr.Status().Code == http.StatusConflict {
				return true, requeueAfterStatusConflictTime, fmt.Errorf("encountered status conflict on workflow update (%v), requeuing after %v", workflow.Name, requeueAfterStatusConflictTime)
			}
			// TODO We probably want this to be larger than 0.
			return true, 0, serr
		}
	}
	return false, 0, nil
}

// TODO: We might want to move this to another file. Maybe utils?
func (t *Transitioner) setLabels(workflow *api.Workflow) (newWorkflow *api.Workflow, err error) {
	glog.V(3).Infof("Setting labels on wf %v", workflow.Name)
	if workflow.Labels == nil {
		workflow.Labels = make(map[string]string)
	}
	if workflow.Spec.JobsSelector == nil {
		workflow.Spec.JobsSelector = &k8sApiUnv.LabelSelector{
			MatchLabels: make(map[string]string),
		}
	}
	workflow.Labels[workflowUID] = string(workflow.UID)
	workflow.Spec.JobsSelector.MatchLabels[workflowUID] = string(workflow.UID)
	newWorkflow, err = t.tpClient.Workflows(workflow.Namespace).Update(workflow)
	return
}

// pastActiveDeadline checks if workflow has ActiveDeadlineSeconds field set and if it is exceeded.
func pastActiveDeadline(workflow *api.Workflow) bool {
	return false
}

func (t *Transitioner) updateWorkflow(workflow *api.Workflow) error {
	_, err := t.tpClient.Workflows(workflow.Namespace).Update(workflow)
	return err
}

func (t *Transitioner) manageWorkflow(workflow *api.Workflow) bool {
	needsStatusUpdate := false
	glog.V(3).Infof("Manage workflow %v", workflow.Name)
	workflowComplete := true
	for stepName, step := range workflow.Spec.Steps {
		if stepStatus, ok := workflow.Status.Statuses[stepName]; ok && stepStatus.Complete {
			continue // step completed nothing to do
		}
		workflowComplete = false
		switch {
		case step.JobTemplate != nil: // Job step
			needsStatusUpdate = t.manageWorkflowJob(workflow, stepName, &step) || needsStatusUpdate
		case step.ExternalRef != nil: // external object reference
			needsStatusUpdate = t.manageWorkflowReference(workflow, stepName, &step) || needsStatusUpdate
		}
	}

	if workflowComplete {
		glog.V(3).Infof("Setting workflow complete status for workflow %v", workflow.Name)
		now := k8sApiUnv.Now()
		condition := api.WorkflowCondition{
			Type:               api.WorkflowComplete,
			Status:             k8sApi.ConditionTrue,
			LastProbeTime:      now,
			LastTransitionTime: now,
		}
		workflow.Status.Conditions = append(workflow.Status.Conditions, condition)
		workflow.Status.CompletionTime = &now
		needsStatusUpdate = true
	}

	return needsStatusUpdate
}

// TODO: I think this method should return an error (instead of logging the error).
//       hen an error is returned we probably want to requeue the workflow.
// manageWorkflowJob manages a Job that is in a step of a workflot.
func (t *Transitioner) manageWorkflowJob(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	for _, dependencyName := range step.Dependencies {
		dependencyStatus, ok := workflow.Status.Statuses[dependencyName]
		if !ok || !dependencyStatus.Complete {
			return false
		}
	}

	// all dependencies satisfied (or missing) need action: update or create step
	glog.V(3).Infof("Dependencies satisfied for %v", stepName)

	key, err := k8sCtl.KeyFunc(workflow)
	if err != nil {
		glog.Errorf("Couldn't get key for workflow %#v: %v", workflow, err)
		return false
	}
	// fetch job by labelSelector and step
	jobSelector := controller.CreateWorkflowJobLabelSelector(workflow, workflow.Spec.Steps[stepName].JobTemplate, stepName)
	jobList, err := t.jobStore.Jobs(workflow.Namespace).List(jobSelector)
	if err != nil {
		glog.Errorf("Error getting jobs for workflow %q: %v", key, err)
		// TODO: We requeued the workflow at this point (before refactoring to Transitioner). Is that really necessary?
		// t.enqueueController(workflow)
		return false
	}

	glog.V(3).Infof("Listing jobs for step %v of wf %v resulted in %d items", stepName, workflow.Name, len(jobList.Items))

	switch len(jobList.Items) {
	case 0: // create job
		err := t.jobControl.CreateJob(workflow.Namespace, step.JobTemplate, workflow, stepName)
		if err != nil {
			glog.Errorf("Couldn't create job %v in step %v for wf %v", step.JobTemplate.Name, stepName, workflow.Name)
			defer k8sUtRunt.HandleError(err)
		} else {
			glog.V(3).Infof("Created job %v in step %v for wf %v", step.JobTemplate.Name, stepName, workflow.Name)
		}
	case 1: // update status
		job := jobList.Items[0]
		reference, err := k8sApi.GetReference(&job)
		if err != nil || reference == nil {
			glog.Errorf("Unable to get reference from job %v in step %v of wf %v: %v", job.Name, stepName, workflow.Name, err)
			return false
		}
		oldStatus := workflow.Status.Statuses[stepName]
		jobFinished := controller.IsJobFinished(&job)
		workflow.Status.Statuses[stepName] = api.WorkflowStepStatus{
			Complete:  jobFinished,
			Reference: *reference}
		glog.V(3).Infof("Updated job status from %v to %v for job %v in step %v for wf %v", oldStatus.Complete,
			workflow.Status.Statuses[stepName].Complete, job.Name, stepName, workflow.Name)
	default: // reconciliate
		glog.Errorf("WorkflowController.manageWorkfloJob resulted in too many jobs for wf %v. Need reconciliation.", workflow.Name)
		return false
	}
	return true
}

func (t *Transitioner) manageWorkflowReference(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	return false
}

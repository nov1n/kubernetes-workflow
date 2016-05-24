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
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sCache "k8s.io/kubernetes/pkg/client/cache"
	k8sClSet "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	k8sClSetUnv "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/unversioned"
	k8sRec "k8s.io/kubernetes/pkg/client/record"
	k8sCl "k8s.io/kubernetes/pkg/client/unversioned"
	k8sCtl "k8s.io/kubernetes/pkg/controller"
	k8sFrwk "k8s.io/kubernetes/pkg/controller/framework"
	k8sRepli "k8s.io/kubernetes/pkg/controller/replication"
	k8sKubectl "k8s.io/kubernetes/pkg/kubectl"
	k8sRunt "k8s.io/kubernetes/pkg/runtime"
	k8sUtRunt "k8s.io/kubernetes/pkg/util/runtime"
	k8sWait "k8s.io/kubernetes/pkg/util/wait"
	k8sWq "k8s.io/kubernetes/pkg/util/workqueue"
	k8sWatch "k8s.io/kubernetes/pkg/watch"
)

const (
	workflowUID                    = "workflow-uid"
	requeueAfterStatusConflictTime = 500 * time.Millisecond
	requeueJobstoreNotSyncedTime   = 100 * time.Millisecond
)

type WorkflowManager struct {
	oldKubeClient k8sCl.Interface
	kubeClient    k8sClSet.Interface
	tpClient      *client.ThirdPartyClient

	jobControl controller.JobControlInterface

	// To allow injection of updateWorkflowStatus for testing.
	updateHandler func(workflow *api.Workflow) error
	syncHandler   func(workflowKey string) error

	// jobStoreSynced returns true if the jod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	jobStoreSynced func() bool

	// A TTLCache of job creates/deletes each rc expects to see
	expectations k8sCtl.ControllerExpectationsInterface

	// A store of workflow, populated by the frameworkController
	workflowStore cache.StoreToWorkflowLister
	// Watches changes to all workflows
	workflowController *k8sFrwk.Controller

	// Store of job
	jobStore cache.StoreToJobLister

	// Watches changes to all jobs
	jobController *k8sFrwk.Controller

	// Workflows that need to be updated
	queue *k8sWq.Type

	recorder k8sRec.EventRecorder
}

func NewWorkflowManager(oldClient k8sCl.Interface, kubeClient k8sClSet.Interface, tpClient *client.ThirdPartyClient, resyncPeriod k8sCtl.ResyncPeriodFunc) *WorkflowManager {
	eventBroadcaster := k8sRec.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	// TODO: remove the wrapper when every clients have moved to use the clientset.
	eventBroadcaster.StartRecordingToSink(&k8sClSetUnv.EventSinkImpl{Interface: kubeClient.Core().Events("")})

	wc := &WorkflowManager{
		oldKubeClient: oldClient,
		kubeClient:    kubeClient,
		tpClient:      tpClient,
		jobControl: controller.WorkflowJobControl{
			KubeClient: kubeClient,
			Recorder:   eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: "workflow-controller"}),
		},
		expectations: k8sCtl.NewControllerExpectations(),
		queue:        k8sWq.New(),
		recorder:     eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: "workflow-controller"}),
	}

	wc.workflowStore.Store, wc.workflowController = k8sFrwk.NewInformer(
		&k8sCache.ListWatch{
			ListFunc: func(options k8sApi.ListOptions) (k8sRunt.Object, error) {
				return wc.tpClient.Workflows(k8sApi.NamespaceAll).List(options)
			},
			WatchFunc: func(options k8sApi.ListOptions) (k8sWatch.Interface, error) {
				return wc.tpClient.Workflows(k8sApi.NamespaceAll).Watch(options)
			},
		},
		&api.Workflow{},
		k8sRepli.FullControllerResyncPeriod,
		k8sFrwk.ResourceEventHandlerFuncs{
			AddFunc: wc.enqueueController,
			UpdateFunc: func(old, cur interface{}) {
				if workflow := cur.(*api.Workflow); !isWorkflowFinished(workflow) {
					// wc.enqueueController(workflow)
					// fmt.Println("UPDATE")
				}
				glog.V(3).Infof("Update WF old=%v, cur=%v", old.(*api.Workflow), cur.(*api.Workflow))
			},
			DeleteFunc: wc.deleteWorkflow,
		},
	)

	wc.jobStore.Store, wc.jobController = k8sFrwk.NewInformer(
		&k8sCache.ListWatch{
			ListFunc: func(options k8sApi.ListOptions) (k8sRunt.Object, error) {
				return wc.oldKubeClient.Batch().Jobs(k8sApi.NamespaceAll).List(options)
			},
			WatchFunc: func(options k8sApi.ListOptions) (k8sWatch.Interface, error) {
				return wc.oldKubeClient.Batch().Jobs(k8sApi.NamespaceAll).Watch(options)
			},
		},
		&k8sBatch.Job{},
		k8sRepli.FullControllerResyncPeriod,
		k8sFrwk.ResourceEventHandlerFuncs{
			AddFunc:    wc.addJob,
			UpdateFunc: wc.updateJob,
			DeleteFunc: wc.deleteJob,
		},
	)

	wc.updateHandler = wc.updateWorkflowStatus
	wc.syncHandler = wc.syncWorkflow
	wc.jobStoreSynced = wc.jobController.HasSynced
	return wc
}

// Run the main goroutine responsible for watching and syncing workflows.
func (w *WorkflowManager) Run(workers int, stopCh <-chan struct{}) {
	defer k8sUtRunt.HandleCrash()
	go w.workflowController.Run(stopCh)
	go w.jobController.Run(stopCh)
	for i := 0; i < workers; i++ {
		go k8sWait.Until(w.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.V(3).Infof("Shutting down Workflow Controller")
	w.queue.ShutDown()
}

// getJobWorkflow return the workflow managing the given job
func (w *WorkflowManager) getJobWorkflow(job *k8sBatch.Job) *api.Workflow {
	workflows, err := w.workflowStore.GetJobWorkflows(job)
	if err != nil {
		glog.V(3).Infof("No workflows found for job %v: %v", job.Name, err)
		return nil
	}
	if len(workflows) > 1 {
		glog.Errorf("more than one workflow found for job %v with labels: %+v", job.Name, job.Labels)
		//sort.Sort(byCreationTimestamp(jobs))
	}
	return &workflows[0]
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (w *WorkflowManager) worker() {
	for {
		func() {
			key, quit := w.queue.Get()
			glog.V(3).Infof("Worker got key from queue: %v\n", key)
			if quit {
				return
			}
			defer w.queue.Done(key)
			err := w.syncHandler(key.(string))
			if err != nil {
				glog.Errorf("Error syncing workflow: %v", err)
			}
		}()
	}
}

func (w *WorkflowManager) syncWorkflow(key string) error {
	glog.V(3).Infoln("Syncing: " + key)

	startTime := time.Now()
	defer func() {
		glog.V(3).Infof("Finished syncing workflow %q (%v)", key, time.Now().Sub(startTime))
	}()

	// Check if the jobStore is synced yet (initialized)
	if !w.jobStoreSynced() {
		glog.V(3).Infof("Waiting for job controller to sync, requeuing workflow %v", key)
		w.enqueueKeyAfter(key, requeueJobstoreNotSyncedTime)
		return nil
	}

	// Obtain the workflow object from store by key
	obj, exists, err := w.workflowStore.Store.GetByKey(key)
	if !exists {

		glog.V(3).Infof("Workflow has been deleted: %v", key)
		return nil
	}
	if err != nil {
		glog.Errorf("Unable to retrieve workflow %v from store: %v", key, err)
		w.queue.Add(key)
		return err
	}
	workflow := *obj.(*api.Workflow)
	// Set defaults for workflow
	if _, ok := workflow.Labels[workflowUID]; !ok {
		newWorkflow, err := w.setLabels(&workflow)
		if err != nil {
			serr, ok := err.(*k8sApiErr.StatusError)
			if !ok {
				glog.Errorf("Couldn't set labels on workflow %v: %v", key, err)
				w.enqueueController(workflow)
				return nil
			}
			if serr.Status().Code == http.StatusConflict {
				glog.V(3).Infof("encountered status conflict on workflow label update (%v), requeuing after %v", workflow.Name, requeueAfterStatusConflictTime)
				w.enqueueAfter(&workflow, requeueAfterStatusConflictTime)
			}
			return nil
		}
		workflow = *newWorkflow
	}

	// workflowKey, err := controller.KeyFunc(&workflow)
	// if err != nil {
	// 	glog.Errorf("Couldn't get key for workflow %#v: %v", workflow, err)
	// 	return err
	// }

	// If this is the first time syncWorkflow is called and
	// the statuses map is empty, create it
	if workflow.Status.Statuses == nil {
		glog.V(3).Infof("Setting status for workflow %v", workflow.Name)
		workflow.Status.Statuses = make(map[string]api.WorkflowStepStatus, len(workflow.Spec.Steps))
		now := k8sApiUnv.Now()
		workflow.Status.StartTime = &now
	}

	// If expectations are not met ???
	// workflowNeedsSync := w.expectations.SatisfiedExpectations(workflowKey)
	// 	if !workflowNeedsSync {
	// 		glog.V(3).Infof("Workflow %v doesn't need synch", workflow.Name)
	// 		return nil
	// 	}

	// If the workflow is finished we don't have to do anything
	// if isWorkflowFinished(&workflow) {
	// 	return nil
	// }

	// If the the deadline has passed we add a condition, set completion time and
	// fire an event
	// if pastActiveDeadline(&workflow) {
	// 	// @sdminonne: TODO delete jobs & write error for the ExternalReference
	// 	now := unversioned.Now()
	// 	condition := extensions.WorkflowCondition{
	// 		Type:               extensions.WorkflowFailed,
	// 		Status:             api.ConditionTrue,
	// 		LastProbeTime:      now,
	// 		LastTransitionTime: now,
	// 		Reason:             "DeadlineExceeded",
	// 		Message:            "Workflow was active longer than specified deadline",
	// 	}
	// 	workflow.Status.Conditions = append(workflow.Status.Conditions, condition)
	// 	workflow.Status.CompletionTime = &now
	// 	w.recorder.Event(&workflow, api.EventTypeNormal, "DeadlineExceeded", "Workflow was active longer than specified deadline")
	// 	if err := w.updateHandler(&workflow); err != nil {
	// 		glog.Errorf("Failed to update workflow %v, requeuing.  Error: %v", workflow.Name, err)
	// 		w.enqueueController(&workflow)
	// 	}
	// 	return nil
	// }
	//
	// if w.manageWorkflow(&workflow) {
	// 	if err := w.updateHandler(&workflow); err != nil {
	// 		glog.Errorf("Failed to update workflow %v, requeuing.  Error: %v", workflow.Name, err)
	// 		w.enqueueController(&workflow)
	// 	}
	// }

	// Try to schedule suitable steps
	if w.manageWorkflow(&workflow) {
		if err := w.updateHandler(&workflow); err != nil {
			serr, ok := err.(*k8sApiErr.StatusError)
			if !ok {
				glog.Errorf("Failed to update workflow %v, requeuing.  Error: %v", workflow.Name, err)
				w.enqueueController(&workflow)
				return nil
			}
			if serr.Status().Code == http.StatusConflict {
				glog.V(2).Infof("encountered status conflict on workflow update (%v), requeuing after %v", workflow.Name, requeueAfterStatusConflictTime)
				w.enqueueAfter(&workflow, requeueAfterStatusConflictTime)
			}
			return nil
		}
	}

	return nil
}

func (w *WorkflowManager) setLabels(workflow *api.Workflow) (newWorkflow *api.Workflow, err error) {
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
	newWorkflow, err = w.tpClient.Workflows(workflow.Namespace).Update(workflow)
	return
}

// pastActiveDeadline checks if workflow has ActiveDeadlineSeconds field set and if it is exceeded.
func pastActiveDeadline(workflow *api.Workflow) bool {
	return false
}

func (w *WorkflowManager) updateWorkflowStatus(workflow *api.Workflow) error {
	_, err := w.tpClient.Workflows(workflow.Namespace).UpdateStatus(workflow)
	return err
}

func isWorkflowFinished(w *api.Workflow) bool {
	return false
}

func (w *WorkflowManager) enqueueController(obj interface{}) {
	key, err := k8sCtl.KeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	w.queue.Add(key)
}

// enqueueAfter enqueues a workflow after a given time.
// enqueueAfter is non-blocking.
func (w *WorkflowManager) enqueueAfter(obj interface{}, d time.Duration) {
	go func() {
		time.Sleep(d)
		w.enqueueController(obj)
	}()
	return
}

// enqueueKeyAfter enqueues a workflow key after a given time.
// enqueueKeyAfter is non-blocking.
func (w *WorkflowManager) enqueueKeyAfter(key string, d time.Duration) {
	go func() {
		time.Sleep(d)
		w.queue.Add(key)
	}()
	return
}

func (w *WorkflowManager) addJob(obj interface{}) {
	// type safety enforced by Informer
	job := obj.(*k8sBatch.Job)
	glog.V(3).Infof("addJob %v", job.Name)
	if workflow := w.getJobWorkflow(job); workflow != nil {
		glog.V(3).Infof("enqueueing controller for %v", job.Name)
		w.enqueueController(workflow)
	}
}

func (w *WorkflowManager) updateJob(old, cur interface{}) {
	// type safety enforced by Informer
	oldJob := old.(*k8sBatch.Job)
	curJob := cur.(*k8sBatch.Job)
	glog.V(3).Infof("updateJob old=%v, cur=%v ", oldJob.Name, curJob.Name)
	if k8sApi.Semantic.DeepEqual(old, cur) {
		glog.V(3).Infof("\t nothing to update")
		return
	}
	if workflow := w.getJobWorkflow(curJob); workflow != nil {
		glog.V(3).Infof("enqueueing controller for %v", curJob.Name)
		w.enqueueController(workflow)
	}
}

func (w *WorkflowManager) deleteJob(obj interface{}) {
	// type safety enforced by Informer
	job, ok := obj.(*k8sBatch.Job)
	if !ok {
		tombstone, ok := obj.(k8sCache.DeletedFinalStateUnknown)
		if !ok {
			glog.Errorf("DeleteJob: Tombstone not found for obj %v", obj)
			return
		}
		glog.V(3).Infof("DeleteJob: Tombstone found %v", tombstone)
		job, ok = tombstone.Obj.(*k8sBatch.Job)
		if !ok {
			glog.Errorf("DeleteJob: Tombstone contained object that is not a job %+v", tombstone)
			return
		}
		glog.V(3).Infof("DeleteJob: Job found in tombstone: %v", job)
	}
	glog.V(3).Infof("DeleteJob old=%v", job.Name)
	if workflow := w.getJobWorkflow(job); workflow != nil {
		glog.V(3).Infof("enqueueing controller for %v", job.Name)
		w.enqueueController(workflow)
	}
}

func (m *WorkflowManager) deleteWorkflow(obj interface{}) {
	// type safety enforced by Informer
	workflow, ok := obj.(*api.Workflow)
	if !ok {
		tombstone, ok := obj.(k8sCache.DeletedFinalStateUnknown)
		if !ok {
			glog.Errorf("DeleteWorkflow: Tombstone not found for obj %v", obj)
			return
		}
		glog.V(3).Infof("DeleteWorkflow: Tombstone found %v", tombstone)
		workflow, ok = tombstone.Obj.(*api.Workflow)
		if !ok {
			glog.Errorf("DeleteWorkflow: Tombstone contained object that is not a workflow %+v", tombstone)
			return
		}
		glog.V(3).Infof("DeleteWorkflow: Workflow found in tombstone: %v", workflow)
	}

	errs := m.deleteAllJobs(workflow)
	if len(errs) > 0 {
		glog.Errorf("Deleting all jobs on workflow %v returned the following errors: %v", workflow.Name, errs)
		return
	}
	glog.V(3).Infof("Deleted all jobs on workflow %v succesfully.", workflow.Name)
}

func (m *WorkflowManager) getAllJobs(workflow *api.Workflow) (jobs k8sBatch.JobList, err error) {
	selector, err := k8sApiUnv.LabelSelectorAsSelector(workflow.Spec.JobsSelector)
	if err != nil {
		return k8sBatch.JobList{}, fmt.Errorf("Could not convert workflow jobsSelector to k8sApiUnv.Selector for workflow %v", workflow.Name)
	}
	return m.jobStore.Jobs(workflow.Namespace).List(selector)
}

func (m *WorkflowManager) deleteAllJobs(workflow *api.Workflow) (errs []error) {
	reaper, err := k8sKubectl.ReaperFor(k8sBatch.Kind("Job"), m.oldKubeClient)
	if err != nil {
		return []error{fmt.Errorf("couldn't create job reaper: %v", err)}
	}
	glog.V(3).Infof("Deleting all jobs for workflow %v", workflow.Name)
	jobs, err := m.getAllJobs(workflow)
	if err != nil {
		return []error{fmt.Errorf("couldn't list jobs: %v", err)}
	}
	for _, job := range jobs.Items {
		glog.V(3).Infof("Deleting job %v/%v for workflow %v", job.Namespace, job.Name, workflow.Name)
		errs = append(errs, reaper.Stop(job.Namespace, job.Name, 0, nil))
	}
	return
}

func (w *WorkflowManager) manageWorkflow(workflow *api.Workflow) bool {
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
			needsStatusUpdate = w.manageWorkflowJob(workflow, stepName, &step) || needsStatusUpdate
		case step.ExternalRef != nil: // external object reference
			needsStatusUpdate = w.manageWorkflowReference(workflow, stepName, &step) || needsStatusUpdate
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

// manageWorkflowJob manages a Job that is in a step of a workflow.
func (w *WorkflowManager) manageWorkflowJob(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
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
	jobList, err := w.jobStore.Jobs(workflow.Namespace).List(jobSelector)
	if err != nil {
		glog.Errorf("Error getting jobs for workflow %q: %v", key, err)
		w.enqueueController(workflow)
		return false
	}

	glog.V(3).Infof("Listing jobs for step %v of wf %v resulted in %d items", stepName, workflow.Name, len(jobList.Items))

	switch len(jobList.Items) {
	case 0: // create job
		err := w.jobControl.CreateJob(workflow.Namespace, step.JobTemplate, workflow, stepName)
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

func (w *WorkflowManager) manageWorkflowReference(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	return false
}

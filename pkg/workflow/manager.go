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

package workflow

import (
	"time"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/client/cache"
	"github.com/nov1n/kubernetes-workflow/pkg/controller"
	k8sApi "k8s.io/kubernetes/pkg/api"
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
	k8sRunt "k8s.io/kubernetes/pkg/runtime"
	k8sUtRunt "k8s.io/kubernetes/pkg/util/runtime"
	k8sWait "k8s.io/kubernetes/pkg/util/wait"
	k8sWq "k8s.io/kubernetes/pkg/util/workqueue"
	k8sWatch "k8s.io/kubernetes/pkg/watch"
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
	jobStore k8sCache.StoreToJobLister

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
				// @borismattijssen TODO: allow different namespaces
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
			},
			DeleteFunc: wc.enqueueController,
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
	glog.Infof("Shutting down Workflow Controller")
	w.queue.ShutDown()
}

// getJobWorkflow return the workflow managing the given job
func (w *WorkflowManager) getJobWorkflow(job *k8sBatch.Job) *api.Workflow {
	return nil
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (w *WorkflowManager) worker() {
	for {
		func() {
			key, quit := w.queue.Get()
			glog.Infof("Worker got key from queue: %v\n", key)
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
	glog.Infoln("Syncing: " + key)

	startTime := time.Now()
	defer func() {
		glog.V(4).Infof("Finished syncing workflow %q (%v)", key, time.Now().Sub(startTime))
	}()

	// Check if the jobStore is synced yet (initialized)
	if !w.jobStoreSynced() {
		time.Sleep(100 * time.Millisecond) // @sdminonne: TODO remove hard coded value
		glog.Infof("Waiting for job controller to sync, requeuing workflow %v", key)
		w.queue.Add(key)
		return nil
	}

	// Obtain the workflow object from store by key
	obj, exists, err := w.workflowStore.Store.GetByKey(key)
	if !exists {
		glog.V(4).Infof("Workflow has been deleted: %v", key)
		w.expectations.DeleteExpectations(key)
		return nil
	}
	if err != nil {
		glog.Errorf("Unable to retrieve workflow %v from store: %v", key, err)
		w.queue.Add(key)
		return err
	}
	workflow := *obj.(*api.Workflow)
	// workflowKey, err := controller.KeyFunc(&workflow)
	// if err != nil {
	// 	glog.Errorf("Couldn't get key for workflow %#v: %v", workflow, err)
	// 	return err
	// }

	// If this is the first time syncWorkflow is called and
	// the statuses map is empty, create it
	if workflow.Status.Statuses == nil {
		workflow.Status.Statuses = make(map[string]api.WorkflowStepStatus, len(workflow.Spec.Steps))
		now := k8sApiUnv.Now()
		workflow.Status.StartTime = &now
	}

	// If expectations are not met ???
	// workflowNeedsSync := w.expectations.SatisfiedExpectations(workflowKey)
	// 	if !workflowNeedsSync {
	// 		glog.V(4).Infof("Workflow %v doesn't need synch", workflow.Name)
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
			glog.Errorf("Failed to update workflow %v, requeuing.  Error: %v", workflow.Name, err)
			w.enqueueController(&workflow)
		}
	}

	return nil
}

// pastActiveDeadline checks if workflow has ActiveDeadlineSeconds field set and if it is exceeded.
func pastActiveDeadline(workflow *api.Workflow) bool {
	return false
}

func (w *WorkflowManager) updateWorkflowStatus(workflow *api.Workflow) error {
	return nil
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

func (w *WorkflowManager) addJob(obj interface{}) {
}

func (w *WorkflowManager) updateJob(old, cur interface{}) {

}

func (w *WorkflowManager) deleteJob(obj interface{}) {

}

func (w *WorkflowManager) manageWorkflow(workflow *api.Workflow) bool {
	for stepName, step := range workflow.Spec.Steps {
		err := w.jobControl.CreateJob(workflow.Namespace, step.JobTemplate, workflow, stepName)
		if err != nil {
			glog.Errorf("Error creating job: %v\n", err)
		}
		// job := &batch.Job{
		// 	ObjectMeta: k8sApi.ObjectMeta{
		// 		GenerateName: "aa-",
		// 	},
		// 	Spec: step.JobTemplate.Spec,
		// }
		//
		// if _, err := w.kubeClient.Batch().Jobs(workflow.Namespace).Create(job); err != nil {
		// 	fmt.Printf("unable to create job: %v", err)
		// }
	}
	return false
}

func (w *WorkflowManager) manageWorkflowJob(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	return false
}

func (w *WorkflowManager) manageWorkflowReference(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	return false
}

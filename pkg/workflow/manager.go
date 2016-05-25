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
	"time"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/client/cache"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sCache "k8s.io/kubernetes/pkg/client/cache"
	k8sClSet "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
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

	// jobStoreSynced returns true if the jod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	jobStoreSynced func() bool

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

	transitioner *Transitioner
}

func NewWorkflowManager(oldClient k8sCl.Interface, kubeClient k8sClSet.Interface, tpClient *client.ThirdPartyClient, resyncPeriod k8sCtl.ResyncPeriodFunc) *WorkflowManager {
	wc := &WorkflowManager{
		oldKubeClient: oldClient,
		kubeClient:    kubeClient,
		tpClient:      tpClient,
		queue:         k8sWq.New(),
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
			AddFunc: wc.enqueueWorkflow,
			UpdateFunc: func(old, cur interface{}) {
				if workflow := cur.(*api.Workflow); !isWorkflowFinished(workflow) {
					wc.enqueueWorkflow(workflow)
				}
				glog.V(3).Infof("Update WF old=%v, cur=%v", old.(*api.Workflow), cur.(*api.Workflow))
			},
			DeleteFunc: wc.enqueueWorkflow,
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

	wc.transitioner = NewTransitioner(tpClient, kubeClient, wc.jobController.HasSynced, &wc.workflowStore, &wc.jobStore)
	wc.jobStoreSynced = wc.jobController.HasSynced
	return wc
}

func isWorkflowFinished(w *api.Workflow) bool {
	return false
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
			requeue, requeueAfter, err := w.transitioner.Transition(key.(string))
			if err != nil {
				glog.Errorf("Error syncing workflow: %v", err)
			}
			if requeue {
				w.enqueueAfter(key.(string), requeueAfter)
			}
		}()
	}
}

func (w *WorkflowManager) enqueueWorkflow(obj interface{}) {
	key, err := k8sCtl.KeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	w.queue.Add(key)
}

// enqueueAfter enqueues a workflow after a given time.
// enqueueAfter is non-blocking.
func (w *WorkflowManager) enqueueAfter(key string, d time.Duration) {
	if d > 0 {
		go func() {
			time.Sleep(d)
			w.queue.Add(key)
		}()
	} else {
		w.queue.Add(key)
	}
	return
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

func (w *WorkflowManager) addJob(obj interface{}) {
	// type safety enforced by Informer
	job := obj.(*k8sBatch.Job)
	glog.V(3).Infof("addJob %v", job.Name)
	if workflow := w.getJobWorkflow(job); workflow != nil {
		glog.V(3).Infof("enqueueing controller for %v", job.Name)
		w.enqueueWorkflow(workflow)
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
		w.enqueueWorkflow(workflow)
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
		w.enqueueWorkflow(workflow)
	}
}

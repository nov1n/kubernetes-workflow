package workflow

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/client/cache"
	"github.com/nov1n/kubernetes-workflow/pkg/controller"
	k8sApi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	k8sCache "k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	unversionedcore "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/unversioned"
	"k8s.io/kubernetes/pkg/client/record"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
	k8sController "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	replicationcontroller "k8s.io/kubernetes/pkg/controller/replication"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"
)

type WorkflowManager struct {
	oldKubeClient k8sClient.Interface
	kubeClient    clientset.Interface
	tpClient      *client.ThirdPartyClient

	jobControl controller.JobControlInterface

	// To allow injection of updateWorkflowStatus for testing.
	updateHandler func(workflow *api.Workflow) error
	syncHandler   func(workflowKey string) error

	// jobStoreSynced returns true if the jod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	jobStoreSynced func() bool

	// A TTLCache of job creates/deletes each rc expects to see
	expectations k8sController.ControllerExpectationsInterface

	// A store of workflow, populated by the frameworkController
	workflowStore cache.StoreToWorkflowLister
	// Watches changes to all workflows
	workflowController *framework.Controller

	// Store of job
	jobStore k8sCache.StoreToJobLister

	// Watches changes to all jobs
	jobController *framework.Controller

	// Workflows that need to be updated
	queue *workqueue.Type

	recorder record.EventRecorder
}

func NewWorkflowManager(oldClient k8sClient.Interface, kubeClient clientset.Interface, tpClient *client.ThirdPartyClient, resyncPeriod k8sController.ResyncPeriodFunc) *WorkflowManager {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	// TODO: remove the wrapper when every clients have moved to use the clientset.
	eventBroadcaster.StartRecordingToSink(&unversionedcore.EventSinkImpl{Interface: kubeClient.Core().Events("")})

	wc := &WorkflowManager{
		oldKubeClient: oldClient,
		kubeClient:    kubeClient,
		tpClient:      tpClient,
		jobControl: controller.WorkflowJobControl{
			KubeClient: kubeClient,
			Recorder:   eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: "workflow-controller"}),
		},
		expectations: k8sController.NewControllerExpectations(),
		queue:        workqueue.New(),
		recorder:     eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: "workflow-controller"}),
	}

	wc.workflowStore.Store, wc.workflowController = framework.NewInformer(
		&k8sCache.ListWatch{
			ListFunc: func(options k8sApi.ListOptions) (runtime.Object, error) {
				// @borismattijssen TODO: allow different namespaces
				return wc.tpClient.Workflows(k8sApi.NamespaceAll).List(options)
			},
			WatchFunc: func(options k8sApi.ListOptions) (watch.Interface, error) {
				return wc.tpClient.Workflows(k8sApi.NamespaceAll).Watch(options)
			},
		},
		&api.Workflow{},
		replicationcontroller.FullControllerResyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: wc.enqueueController,
			UpdateFunc: func(old, cur interface{}) {
				if workflow := cur.(*api.Workflow); !isWorkflowFinished(workflow) {
					wc.enqueueController(workflow)
				}
			},
			DeleteFunc: wc.enqueueController,
		},
	)

	wc.jobStore.Store, wc.jobController = framework.NewInformer(
		&k8sCache.ListWatch{
			ListFunc: func(options k8sApi.ListOptions) (runtime.Object, error) {
				return wc.oldKubeClient.Batch().Jobs(k8sApi.NamespaceAll).List(options)
			},
			WatchFunc: func(options k8sApi.ListOptions) (watch.Interface, error) {
				return wc.oldKubeClient.Batch().Jobs(k8sApi.NamespaceAll).Watch(options)
			},
		},
		&batch.Job{},
		replicationcontroller.FullControllerResyncPeriod,
		framework.ResourceEventHandlerFuncs{
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
	defer utilruntime.HandleCrash()
	go w.workflowController.Run(stopCh)
	go w.jobController.Run(stopCh)
	for i := 0; i < workers; i++ {
		go wait.Until(w.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.Infof("Shutting down Workflow Controller")
	w.queue.ShutDown()
}

// getJobWorkflow return the workflow managing the given job
func (w *WorkflowManager) getJobWorkflow(job *batch.Job) *api.Workflow {
	return nil
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (w *WorkflowManager) worker() {
	for {
		func() {
			fmt.Println("Worker started")
			key, quit := w.queue.Get()
			fmt.Printf("Worker got key from queue: %v\n", key)
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
	fmt.Println("Syncing: " + key)

	// Check if the jobStore is synced yet (initialized)

	// Obtain the workflow object from store by key

	// If this is the first time syncWorkflow is called and
	// the statuses map is empty, create it

	// If expectations are not met ???

	// If the workflow is finished we don't have to do anything

	// If the the deadline has passed we add a condition, set completion time and
	// fire an event

	// Try to schedule suitable steps

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
	key, err := k8sController.KeyFunc(obj)
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
	return false
}

func (w *WorkflowManager) manageWorkflowJob(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	return false
}

func (w *WorkflowManager) manageWorkflowReference(workflow *api.Workflow, stepName string, step *api.WorkflowStep) bool {
	return false
}

package workflow

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/control"
)

type Syncer struct {
	jobControl      control.JobInterface
	workflowControl control.WorkflowInterface
}

func NewSyncer(kubeClient k8sClSet.Interface, tpClient *client.ThirdPartyClient) *Syncer {
	eventBroadcaster := k8sRec.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	// TODO: remove the wrapper when every clients have moved to use the clientset.
	eventBroadcaster.StartRecordingToSink(&k8sClSetUnv.EventSinkImpl{Interface: m.kubeClient.Core().Events("")})
	return &Syncer{
		jobControl: control.Job{
			KubeClient: m.kubeClient,
			Recorder:   eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: recorderComponent}),
		},
		workflowControl: control.Workflow{
			KubeClient: m.tpClient,
			Recorder:   eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: recorderComponent}),
		},
	}
}

func (s *Syncer) sync(old *api.Workflow, new *api.Workflow) error {
	// check if a new job should be created
	for step, status := range new.Status.Statuses {
		runningOld, ok := old.Status.Statuses[step].Conditions[api.WorkflowStepRunning]
		runningNew, ok := status.Conditions[api.WorkflowStepRunning]
		if ok && running.Status == k8sApi.ConditionTrue {

		}
	}
	// something changed, we should update the workflow
	if !deepEquals(*old, *new) {
		err := s.workflowControl.UpdateWorkflow(new)
		if err != nil {
			return fmt.Errorf("Syncer couldn't update workflow: %v", err)
		}
	}
}

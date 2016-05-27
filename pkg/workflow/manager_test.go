package workflow

import (
	"testing"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/job"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sTestApi "k8s.io/kubernetes/pkg/api/testapi"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sClSet "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
	k8sCl "k8s.io/kubernetes/pkg/client/unversioned"
	k8sCtl "k8s.io/kubernetes/pkg/controller"
)

// utility function to create a JobTemplateSpec
func newJobTemplateSpec() *k8sBatch.JobTemplateSpec {
	return &k8sBatch.JobTemplateSpec{
		ObjectMeta: k8sApi.ObjectMeta{
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: k8sBatch.JobSpec{
			Template: k8sApi.PodTemplateSpec{
				ObjectMeta: k8sApi.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: k8sApi.PodSpec{
					Containers: []k8sApi.Container{
						{Image: "foo/bar"},
					},
				},
			},
		},
	}
}

func newJobTemplateStatus() api.WorkflowStepStatus {
	return api.WorkflowStepStatus{
		Complete: false,
		Reference: k8sApi.ObjectReference{
			Kind:      "Job",
			Name:      "foo",
			Namespace: k8sApi.NamespaceDefault,
		},
	}
}

func getKey(workflow *api.Workflow, t *testing.T) string {
	key, err := k8sCtl.KeyFunc(workflow)
	if err != nil {
		t.Errorf("Unexpected error getting key for workflow %v: %v", workflow.Name, err)
		return ""
	}
	return key
}

func TestControllerSyncWorkflow(t *testing.T) {
	testCases := map[string]struct {
		workflow           *api.Workflow
		jobs               []k8sBatch.Job
		checkWorkflow      func(testName string, workflow *api.Workflow, t *testing.T)
		expectedStartedJob int
	}{

		"workflow start": {
			workflow: &api.Workflow{
				ObjectMeta: k8sApi.ObjectMeta{
					Name:      "mydag",
					Namespace: k8sApi.NamespaceDefault,
				},
				Spec: api.WorkflowSpec{
					Steps: map[string]api.WorkflowStep{
						"myJob": {
							JobTemplate: newJobTemplateSpec(),
						},
					},
				},
			},
			jobs:               []k8sBatch.Job{},
			expectedStartedJob: 1,
		},
		"workflow status update": {
			workflow: &api.Workflow{
				ObjectMeta: k8sApi.ObjectMeta{
					Name:      "mydag",
					Namespace: k8sApi.NamespaceDefault,
				},
				Spec: api.WorkflowSpec{
					Steps: map[string]api.WorkflowStep{
						"myJob": {
							JobTemplate: newJobTemplateSpec(),
						},
					},
				},
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{},
					Statuses: map[string]api.WorkflowStepStatus{
						"myJob": newJobTemplateStatus(),
					},
				},
			},
			jobs: []k8sBatch.Job{
				{
					ObjectMeta: k8sApi.ObjectMeta{
						Name:      "foo",
						Namespace: k8sApi.NamespaceDefault,
						Labels: map[string]string{
							"foo": "bar",
							job.WorkflowStepLabelKey: "myJob",
						},
						SelfLink: "/apis/v1/jobs/foo",
					},
					Spec:   k8sBatch.JobSpec{},
					Status: k8sBatch.JobStatus{},
				},
			},
			checkWorkflow: func(testName string, workflow *api.Workflow, t *testing.T) {
				stepStatus, ok := workflow.Status.Statuses["myJob"]
				if !ok {
					t.Errorf("%s, Workflow step not updated", testName)
					return
				}
				if stepStatus.Complete {
					t.Errorf("%s, Workflow wrongly updated", testName)
				}
			},
			expectedStartedJob: 0,
		},
		"workflow step status update to complete": {
			workflow: &api.Workflow{
				ObjectMeta: k8sApi.ObjectMeta{
					Name:      "mydag",
					Namespace: k8sApi.NamespaceDefault,
				},
				Spec: api.WorkflowSpec{
					Steps: map[string]api.WorkflowStep{
						"myJob": {
							JobTemplate: newJobTemplateSpec(),
						},
					},
				},
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{},
					Statuses: map[string]api.WorkflowStepStatus{
						"myJob": newJobTemplateStatus(),
					},
				},
			},
			jobs: []k8sBatch.Job{
				{
					ObjectMeta: k8sApi.ObjectMeta{
						Name:      "foo",
						Namespace: k8sApi.NamespaceDefault,
						Labels: map[string]string{
							"foo": "bar",
							job.WorkflowStepLabelKey: "myJob",
						},
						SelfLink: "/apis/v1/jobs/foo",
					},
					Spec: k8sBatch.JobSpec{},
					Status: k8sBatch.JobStatus{
						Conditions: []k8sBatch.JobCondition{
							{
								Type:   k8sBatch.JobComplete,
								Status: k8sApi.ConditionTrue,
							},
						},
					},
				},
			},
			checkWorkflow: func(testName string, workflow *api.Workflow, t *testing.T) {
				stepStatus, ok := workflow.Status.Statuses["myJob"]
				if !ok {
					t.Errorf("%s, Workflow step not updated", testName)
					return
				}
				if !stepStatus.Complete {
					t.Errorf("%s, Workflow wrongly updated", testName)
				}
			},
			expectedStartedJob: 0,
		},
		"workflow status update to complete": {
			workflow: &api.Workflow{
				ObjectMeta: k8sApi.ObjectMeta{
					Name:      "mydag",
					Namespace: k8sApi.NamespaceDefault,
				},
				Spec: api.WorkflowSpec{
					Steps: map[string]api.WorkflowStep{
						"myJob": {
							JobTemplate: newJobTemplateSpec(),
						},
					},
				},
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{},
					Statuses: map[string]api.WorkflowStepStatus{
						"myJob": {
							Complete: true,
							Reference: k8sApi.ObjectReference{
								Kind:      "Job",
								Name:      "foo",
								Namespace: k8sApi.NamespaceDefault,
							},
						},
					},
				},
			},
			jobs: []k8sBatch.Job{}, // jobs no retrieved step only
			checkWorkflow: func(testName string, workflow *api.Workflow, t *testing.T) {
				if !isWorkflowFinished(workflow) {
					t.Errorf("%s, Workflow should be finished:\n %#v", testName, workflow)
				}
				if workflow.Status.CompletionTime == nil {
					t.Errorf("%s, CompletionTime not set", testName)
				}
			},
			expectedStartedJob: 0,
		},
		"workflow step dependency complete 3": {
			workflow: &api.Workflow{
				ObjectMeta: k8sApi.ObjectMeta{
					Name:      "mydag",
					Namespace: k8sApi.NamespaceDefault,
				},
				Spec: api.WorkflowSpec{
					Steps: map[string]api.WorkflowStep{
						"one": {
							JobTemplate: newJobTemplateSpec(),
						},
						"two": {
							JobTemplate:  newJobTemplateSpec(),
							Dependencies: []string{"one"},
						},
						"three": {
							JobTemplate:  newJobTemplateSpec(),
							Dependencies: []string{"one"},
						},
						"four": {
							JobTemplate:  newJobTemplateSpec(),
							Dependencies: []string{"one"},
						},
						"five": {
							JobTemplate:  newJobTemplateSpec(),
							Dependencies: []string{"two", "three", "four"},
						},
					},
				},
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{},
					Statuses: map[string]api.WorkflowStepStatus{
						"one": {
							Complete: true,
							Reference: k8sApi.ObjectReference{
								Kind:      "Job",
								Name:      "foo",
								Namespace: k8sApi.NamespaceDefault,
							},
						},
					},
				},
			},
			jobs: []k8sBatch.Job{
				{
					ObjectMeta: k8sApi.ObjectMeta{
						Name:      "foo",
						Namespace: k8sApi.NamespaceDefault,
						Labels: map[string]string{
							"foo": "bar",
							job.WorkflowStepLabelKey: "one",
						},
						SelfLink: "/apis/v1/jobs/foo",
					},
					Spec:   k8sBatch.JobSpec{},
					Status: k8sBatch.JobStatus{},
				},
			},
			checkWorkflow:      func(testName string, workflow *api.Workflow, t *testing.T) {},
			expectedStartedJob: 3,
		},
		"workflow step dependency not complete": {
			workflow: &api.Workflow{
				ObjectMeta: k8sApi.ObjectMeta{
					Name:      "mydag",
					Namespace: k8sApi.NamespaceDefault,
				},
				Spec: api.WorkflowSpec{
					Steps: map[string]api.WorkflowStep{
						"one": {
							JobTemplate: newJobTemplateSpec(),
						},
						"two": {
							JobTemplate:  newJobTemplateSpec(),
							Dependencies: []string{"one"},
						},
					},
				},
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{},
					Statuses: map[string]api.WorkflowStepStatus{
						"one": {
							Complete: false,
							Reference: k8sApi.ObjectReference{
								Kind:      "Job",
								Name:      "foo",
								Namespace: k8sApi.NamespaceDefault,
							},
						},
					},
				},
			},
			jobs: []k8sBatch.Job{
				{
					ObjectMeta: k8sApi.ObjectMeta{
						Name:      "foo",
						Namespace: k8sApi.NamespaceDefault,
						Labels: map[string]string{
							"foo": "bar",
							job.WorkflowStepLabelKey: "one",
						},
						SelfLink: "/apis/v1/jobs/foo",
					},
					Spec:   k8sBatch.JobSpec{},
					Status: k8sBatch.JobStatus{},
				},
			},
			checkWorkflow:      func(testName string, workflow *api.Workflow, t *testing.T) {},
			expectedStartedJob: 0,
		},
	}
	for name, tc := range testCases {
		clientConfig := &k8sRestCl.Config{Host: "", ContentConfig: k8sRestCl.ContentConfig{GroupVersion: k8sTestApi.Default.GroupVersion()}}
		clientset := k8sClSet.NewForConfigOrDie(clientConfig)
		oldClient := k8sCl.NewOrDie(clientConfig)
		thirdPartyClient := client.NewThirdPartyOrDie(k8sApiUnv.GroupVersion{
			Group:   "nerdalize.com",
			Version: "v1alpha1",
		}, *clientConfig)

		manager := NewManager(oldClient, clientset, thirdPartyClient)
		fakeJobControl := job.FakeControl{}
		manager.transitioner.jobControl = &fakeJobControl
		manager.transitioner.jobStoreSynced = func() bool { return true }
		var actual *api.Workflow
		manager.transitioner.updateHandler = func(workflow *api.Workflow) error {
			actual = workflow
			return nil
		}
		// setup workflow, jobs
		manager.workflowStore.Store.Add(tc.workflow)
		for _, job := range tc.jobs {
			manager.jobStore.Store.Add(&job)
		}
		_, _, err := manager.transitioner.Transition(getKey(tc.workflow, t))
		if err != nil {
			t.Errorf("%s: unexpected error syncing workflow %v", name, err)
			continue
		}
		if len(fakeJobControl.CreatedJobTemplates) != tc.expectedStartedJob {
			t.Errorf("%s: unexpected # of created jobs: expected %d got %d", name, tc.expectedStartedJob, len(fakeJobControl.CreatedJobTemplates))
			continue

		}
		if tc.checkWorkflow != nil {
			tc.checkWorkflow(name, actual, t)
		}
	}
}

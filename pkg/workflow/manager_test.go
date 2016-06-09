package workflow

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/job"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sTestApi "k8s.io/kubernetes/pkg/api/testapi"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sClSet "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	k8sFake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
	k8sCore "k8s.io/kubernetes/pkg/client/testing/core"
	k8sCl "k8s.io/kubernetes/pkg/client/unversioned"
	k8sCtl "k8s.io/kubernetes/pkg/controller"
	k8sWatch "k8s.io/kubernetes/pkg/watch"
)

var myV = flag.Int("myV", 0, "test")

func TestFlag(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*myV))
}

// utility function to create a JobTemplateSpec
func newJobTemplateSpec() *k8sBatch.JobTemplateSpec {
	return &k8sBatch.JobTemplateSpec{
		ObjectMeta: k8sApi.ObjectMeta{
			Name: "jobName",
			Labels: map[string]string{
				"foo":                "bar",
				workflowValidLabel:   "true",
				api.WorkflowUIDLabel: "123",
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

func newTestWorkflow() *api.Workflow {
	return &api.Workflow{
		ObjectMeta: k8sApi.ObjectMeta{
			Name:      "mydag",
			Namespace: k8sApi.NamespaceDefault,
			Labels:    map[string]string{api.WorkflowUIDLabel: "123"},
		},
		Spec: api.WorkflowSpec{
			JobsSelector: &k8sApiUnv.LabelSelector{
				MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
			},
			Steps: map[string]api.WorkflowStep{
				"myJob": {
					JobTemplate: newJobTemplateSpec(),
				},
			},
		},
	}
}

// getTestWorkflowList returns a list containing one test workflow.
func getTestWorkflowList(wf api.Workflow) api.WorkflowList {
	return api.WorkflowList{
		TypeMeta: k8sApiUnv.TypeMeta{
			Kind: "WorkflowList",
		},
		Items: []api.Workflow{wf},
	}
}

// getClient returns a ThirdPartyClient that always returns the given output string as a response
// to a request
func getClient(output string) (tpc *client.ThirdPartyClient, err error) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, output)
	}))
	tpc, err = client.NewThirdParty(k8sApiUnv.GroupVersion{
		Group:   api.Group,
		Version: api.Version,
	}, k8sRestCl.Config{
		Host: ts.URL,
	})
	return
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
					Labels:    map[string]string{api.WorkflowUIDLabel: "123"},
				},
				Spec: api.WorkflowSpec{
					JobsSelector: &k8sApiUnv.LabelSelector{
						MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
					},
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
					Labels:    map[string]string{api.WorkflowUIDLabel: "123"},
				},
				Spec: api.WorkflowSpec{
					JobsSelector: &k8sApiUnv.LabelSelector{
						MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
					},
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
							"foo":                    "bar",
							api.WorkflowUIDLabel:     "123",
							workflowValidLabel:       "true",
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
					Labels:    map[string]string{api.WorkflowUIDLabel: "123"},
				},
				Spec: api.WorkflowSpec{
					JobsSelector: &k8sApiUnv.LabelSelector{
						MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
					},
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
							workflowValidLabel:       "true",
							api.WorkflowUIDLabel:     "123",
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
					Labels:    map[string]string{api.WorkflowUIDLabel: "123"},
				},
				Spec: api.WorkflowSpec{
					JobsSelector: &k8sApiUnv.LabelSelector{
						MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
					},
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
				if !workflow.IsFinished() {
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
					Labels:    map[string]string{api.WorkflowUIDLabel: "123"},
				},
				Spec: api.WorkflowSpec{
					JobsSelector: &k8sApiUnv.LabelSelector{
						MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
					},
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
							workflowValidLabel:       "true",
							api.WorkflowUIDLabel:     "123",
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
					Labels:    map[string]string{api.WorkflowUIDLabel: "123"},
				},
				Spec: api.WorkflowSpec{
					JobsSelector: &k8sApiUnv.LabelSelector{
						MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
					},
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
							workflowValidLabel:       "true",
							api.WorkflowUIDLabel:     "123",
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
		"workflow pause": {
			workflow: &api.Workflow{
				ObjectMeta: k8sApi.ObjectMeta{
					Name:      "mydag",
					Namespace: k8sApi.NamespaceDefault,
					Labels: map[string]string{
						api.WorkflowUIDLabel: "123",
						workflowPauseLabel:   trueString,
					},
				},
				Spec: api.WorkflowSpec{
					JobsSelector: &k8sApiUnv.LabelSelector{
						MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
					},
					Steps: map[string]api.WorkflowStep{
						"one": {
							JobTemplate: newJobTemplateSpec(),
						},
					},
				},
			},
			jobs:               []k8sBatch.Job{},
			expectedStartedJob: 0,
		},
	}
	for name, tc := range testCases {
		clientConfig := &k8sRestCl.Config{Host: "", ContentConfig: k8sRestCl.ContentConfig{GroupVersion: k8sTestApi.Default.GroupVersion()}}
		clientset := k8sClSet.NewForConfigOrDie(clientConfig)
		oldClient := k8sCl.NewOrDie(clientConfig)
		thirdPartyClient := client.NewThirdPartyOrDie(k8sApiUnv.GroupVersion{
			Group:   api.Group,
			Version: api.Version,
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
		_, _, err := manager.transitioner.transition(getKey(tc.workflow, t))
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

func TestIsWorkflowFinished(t *testing.T) {
	cases := []struct {
		name     string
		finished bool
		workflow *api.Workflow
	}{
		{
			name:     "Complete and True",
			finished: true,
			workflow: &api.Workflow{
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{
						{
							Type:   api.WorkflowComplete,
							Status: k8sApi.ConditionTrue,
						},
					},
				},
			},
		},
		{
			name:     "Failed and True",
			finished: true,
			workflow: &api.Workflow{
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{
						{
							Type:   api.WorkflowFailed,
							Status: k8sApi.ConditionTrue,
						},
					},
				},
			},
		},
		{
			name:     "Complete and False",
			finished: false,
			workflow: &api.Workflow{
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{
						{
							Type:   api.WorkflowComplete,
							Status: k8sApi.ConditionFalse,
						},
					},
				},
			},
		},
		{
			name:     "Failed and False",
			finished: false,
			workflow: &api.Workflow{
				Status: api.WorkflowStatus{
					Conditions: []api.WorkflowCondition{
						{
							Type:   api.WorkflowComplete,
							Status: k8sApi.ConditionFalse,
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		if tc.workflow.IsFinished() != tc.finished {
			t.Errorf("%s - Expected %v got %v", tc.name, tc.finished, tc.workflow.IsFinished())
		}
	}
}

// TestWatchJobs the case when jobs are added and we get watch events from the server.
func TestWatchJobs(t *testing.T) {
	testWorkflow := newTestWorkflow()
	testList := getTestWorkflowList(*testWorkflow)
	json, err := json.Marshal(testList)
	if err != nil {
		t.Errorf("Error when trying to parse workflow list (%#v): %v", testList, err)
	}
	tpc, err := getClient(string(json))
	if err != nil {
		t.Error("Error while creating client")
	}

	clientConfig := &k8sRestCl.Config{Host: "", ContentConfig: k8sRestCl.ContentConfig{GroupVersion: k8sTestApi.Default.GroupVersion()}}
	oldClient := k8sCl.NewOrDie(clientConfig)
	clientset := k8sFake.NewSimpleClientset()
	fakeWatch := k8sWatch.NewFake()
	clientset.PrependWatchReactor("jobs", k8sCore.DefaultWatchReactor(fakeWatch, nil))
	manager := NewManager(oldClient, clientset, tpc)
	manager.jobStoreSynced = func() bool { return true }

	// Put one job and one pod into the store
	manager.workflowStore.Store.Add(testWorkflow)
	received := make(chan struct{})
	// The pod update sent through the fakeWatcher should figure out the managing job and
	// send it into the syncHandler.
	manager.transitioner.transition = func(key string) (bool, time.Duration, error) {
		obj, exists, err := manager.workflowStore.Store.GetByKey(key)
		if !exists || err != nil {
			t.Errorf("Expected to find workflow under key %v", key)
			close(received)
			return false, 0, nil
		}
		workflow, ok := obj.(*api.Workflow)
		if !ok {
			t.Errorf("unexpected type: %v %#v", reflect.TypeOf(obj), obj)
			close(received)
			return false, 0, nil
		}
		if !k8sApi.Semantic.DeepDerivative(workflow, testWorkflow) {
			t.Errorf("\nExpected %#v,\nbut got %#v", testWorkflow, workflow)
			close(received)
			return false, 0, nil
		}
		close(received)
		return false, 0, nil
	}
	// Start only the pod watcher and the workqueue, send a watch event,
	// and make sure it hits the sync method for the right job.
	stopCh := make(chan struct{})
	defer close(stopCh)
	go manager.Run(1, stopCh)

	jobTemplate := newJobTemplateSpec()
	j := &k8sBatch.Job{
		ObjectMeta: jobTemplate.ObjectMeta,
		Spec:       jobTemplate.Spec,
	}
	fakeWatch.Add(j)

	t.Log("Waiting for pod to reach syncHandler")
	<-received
}

func TestExpectations(t *testing.T) {
	clientConfig := &k8sRestCl.Config{Host: "", ContentConfig: k8sRestCl.ContentConfig{GroupVersion: k8sTestApi.Default.GroupVersion()}}
	clientset := k8sClSet.NewForConfigOrDie(clientConfig)
	oldClient := k8sCl.NewOrDie(clientConfig)
	thirdPartyClient := client.NewThirdPartyOrDie(k8sApiUnv.GroupVersion{
		Group:   api.Group,
		Version: api.Version,
	}, *clientConfig)

	manager := NewManager(oldClient, clientset, thirdPartyClient)
	fakeJobControl := job.FakeControl{}
	manager.transitioner.jobControl = &fakeJobControl
	manager.transitioner.jobStoreSynced = func() bool { return true }
	manager.transitioner.updateHandler = func(workflow *api.Workflow) error {
		return nil
	}

	workflow := &api.Workflow{
		ObjectMeta: k8sApi.ObjectMeta{
			Name:      "mydag",
			Namespace: k8sApi.NamespaceDefault,
			Labels:    map[string]string{api.WorkflowUIDLabel: "123"},
		},
		Spec: api.WorkflowSpec{
			JobsSelector: &k8sApiUnv.LabelSelector{
				MatchLabels: map[string]string{api.WorkflowUIDLabel: "123"},
			},
			Steps: map[string]api.WorkflowStep{
				"myJob": {
					JobTemplate: newJobTemplateSpec(),
				},
			},
		},
	}
	expectedStartedJob := 1
	noOfTransitionsCalls := 2
	// setup workflow
	manager.workflowStore.Store.Add(workflow)
	for i := 0; i < noOfTransitionsCalls-1; i++ {
		_, _, err := manager.transitioner.transition(getKey(workflow, t))
		if err != nil {
			t.Errorf("unexpected error syncing workflow %v", err)
			return
		}
	}
	if len(fakeJobControl.CreatedJobTemplates) != expectedStartedJob {
		t.Errorf("unexpected # of created jobs: expected %d got %d", expectedStartedJob, len(fakeJobControl.CreatedJobTemplates))
	}
}

// TestWatchJobs the case when jobs are added and we get watch events from the server.
// func TestWatchUpdateJobs(t *testing.T) {
// 	testWorkflow := newTestWorkflow()
// 	testList := getTestWorkflowList(*testWorkflow)
// 	json, err := json.Marshal(testList)
// 	if err != nil {
// 		t.Errorf("Error when trying to parse workflow list (%#v): %v", testList, err)
// 	}
// 	tpc, err := getClient(string(json))
// 	if err != nil {
// 		t.Error("Error while creating client")
// 	}
//
// 	clientConfig := &k8sRestCl.Config{Host: "", ContentConfig: k8sRestCl.ContentConfig{GroupVersion: k8sTestApi.Default.GroupVersion()}}
// 	oldClient := k8sCl.NewOrDie(clientConfig)
// 	clientset := k8sFake.NewSimpleClientset()
// 	fakeWatch := k8sWatch.NewFake()
// 	clientset.PrependWatchReactor("jobs", k8sCore.DefaultWatchReactor(fakeWatch, nil))
// 	manager := NewManager(oldClient, clientset, tpc)
// 	manager.jobStoreSynced = func() bool { return true }
//
// 	// Put one job and one pod into the store
// 	manager.workflowStore.Store.Add(testWorkflow)
// 	received := make(chan struct{})
// 	// The pod update sent through the fakeWatcher should figure out the managing job and
// 	// send it into the syncHandler.
// 	manager.transitioner.transition = func(key string) (bool, time.Duration, error) {
// 		obj, exists, err := manager.workflowStore.Store.GetByKey(key)
// 		if !exists || err != nil {
// 			t.Errorf("Expected to find workflow under key %v", key)
// 			close(received)
// 			return false, 0, nil
// 		}
// 		workflow, ok := obj.(*api.Workflow)
// 		if !ok {
// 			t.Errorf("unexpected type: %v %#v", reflect.TypeOf(obj), obj)
// 			close(received)
// 			return false, 0, nil
// 		}
// 		if !k8sApi.Semantic.DeepDerivative(workflow, testWorkflow) {
// 			t.Errorf("\nExpected %#v,\nbut got %#v", testWorkflow, workflow)
// 			close(received)
// 			return false, 0, nil
// 		}
// 		close(received)
// 		return false, 0, nil
// 	}
// 	// Start only the pod watcher and the workqueue, send a watch event,
// 	// and make sure it hits the sync method for the right job.
// 	stopCh := make(chan struct{})
// 	defer close(stopCh)
// 	go manager.Run(1, stopCh)
// 	jobTemplate := newJobTemplateSpec()
// 	j := &k8sBatch.Job{
// 		ObjectMeta: jobTemplate.ObjectMeta,
// 		Spec:       jobTemplate.Spec,
// 	}
//
// 	j.Labels["new"] = "test"
// 	fakeWatch.Modify(j)
//
// 	t.Log("Waiting for pod to reach syncHandler")
// 	<-received
// }

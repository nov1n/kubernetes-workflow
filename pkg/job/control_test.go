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

package job

import (
	"strings"
	"testing"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	k8sRec "k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/client/restclient"
	k8sLabels "k8s.io/kubernetes/pkg/labels"

	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
)

// TestGetJobsPrefix tests the GetJobsPrefix function
func TestGetJobsPrefix(t *testing.T) {
	controllerName := "foo"
	prefix := getJobsPrefix(controllerName)
	expected := "foo-"

	if prefix != expected {
		t.Errorf("Error in test getJobsPrefix: got '%v', expected '%v'", prefix, expected)
	}
}

// spec is a JobTemplateSpec used for testing purposes
var spec = &k8sBatch.JobTemplateSpec{
	ObjectMeta: k8sApi.ObjectMeta{
		Name: "job1",
	},
	Spec: k8sBatch.JobSpec{
		Parallelism: func(i int32) *int32 { return &i }(1),
		Template: k8sApi.PodTemplateSpec{
			ObjectMeta: k8sApi.ObjectMeta{
				Name: "pod1",
			},
			Spec: k8sApi.PodSpec{
				RestartPolicy: "OnFailure",
				Containers: []k8sApi.Container{
					k8sApi.Container{
						Name:  "ubuntu1",
						Image: "ubuntu",
						Command: []string{
							"/bin/sleep", "30",
						},
					},
				},
			},
		},
	},
}

// jobObj is a Job for testing purposes
var jobObj = &k8sBatch.Job{
	TypeMeta:   k8sApiUnv.TypeMeta{},
	ObjectMeta: k8sApi.ObjectMeta{Name: "job1"},
	Spec:       k8sBatch.JobSpec{},
	Status: k8sBatch.JobStatus{
		Conditions: []k8sBatch.JobCondition{
			k8sBatch.JobCondition{
				Type:   k8sBatch.JobFailed,
				Status: k8sApi.ConditionFalse,
			},
		},
	},
}

// wf is a Workflow for testing purposes
var wf = &api.Workflow{
	TypeMeta:   k8sApiUnv.TypeMeta{},
	ObjectMeta: k8sApi.ObjectMeta{Name: "testworkflow"},
	Spec:       api.WorkflowSpec{},
	Status:     api.WorkflowStatus{},
}

// TestGetJobsAnnotationSet tests the GetJobsAnnotationSet function
func TestGetJobsAnnotationSet(t *testing.T) {
	expected := k8sLabels.Set{
		"kubernetes.io/created-by": `{"kind":"SerializedReference","apiVersion":"v1","reference":{"kind":"Workflow","name":"testworkflow","apiVersion":"nerdalize.com/v1alpha1"}}`,
	}
	annotations, err := getJobsAnnotationSet(spec, wf)

	expectedField := strings.TrimSpace(expected["kubernetes.io/created-by"])
	gotField := strings.TrimSpace(annotations["kubernetes.io/created-by"])
	if gotField != expectedField {
		t.Errorf("Error in test getJobsAnnotationSet: got '%v', expected '%v'", gotField, gotField)
	}
	if err != nil {
		t.Errorf("Error in test getJobsAnnotationSet: %v", err)
	}
}

// TestGetWorkflowJobLabelSet tests the GetWorkflowJobLabelSet function
func TestGetWorkflowJobLabelSet(t *testing.T) {
	annotations := getWorkflowJobLabelSet(wf, spec, "foo")

	expectedField := "foo"
	gotField := strings.TrimSpace(annotations[WorkflowStepLabelKey])
	if gotField != expectedField {
		t.Errorf("Error in test getWorkflowJobLabel: got '%v', expected '%v'", gotField, gotField)
	}
}

// TestCreateWorkflowJobLabelSelector tests the CreateWorkflowJobLabelSelector function
func TestCreateWorkflowJobLabelSelector(t *testing.T) {
	// This function is tested by kubernetes, see github.com/kubernetes/kubernetes/pkg/labels/selector_test.go
}

// TestCreateJob tests the CreateJob function
func TestCreateJob(t *testing.T) {
	eventBroadcaster := k8sRec.NewBroadcaster()
	cfg, err := clientset.NewForConfig(&restclient.Config{})
	if err != nil {
		t.Errorf("Could not create config: %v", err)
	}
	control := Control{
		KubeClient: cfg,
		Recorder:   eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: "rec"}),
	}

	err = control.CreateJob("default", spec, wf, "step1")
	if err == nil {
		t.Errorf("%v", err)
	}
}

// TestDeleteJob tests the DeleteJob function
func TestDeleteJob(t *testing.T) {
	eventBroadcaster := k8sRec.NewBroadcaster()
	cfg, err := clientset.NewForConfig(&restclient.Config{})
	if err != nil {
		t.Errorf("Could not create config: %v", err)
	}
	control := Control{
		KubeClient: cfg,
		Recorder:   eventBroadcaster.NewRecorder(k8sApi.EventSource{Component: "rec"}),
	}

	control.DeleteJob("default", "job1", wf) // Not yet implemented
}

// TestIsJobFinished tests the IsJobFinished function
func TestIsJobFinished(t *testing.T) {
	res := IsJobFinished(jobObj)
	if res {
		t.Errorf("Error in TestIsJobFinished: got %v, expected %v", res, true)
	}

	jobObj.Status.Conditions = []k8sBatch.JobCondition{
		k8sBatch.JobCondition{
			Type:   k8sBatch.JobComplete,
			Status: k8sApi.ConditionTrue,
		},
	}
	res = IsJobFinished(jobObj)
	if !res {
		t.Errorf("Error in TestIsJobFinished: got %v, expected %v", res, false)
	}
}

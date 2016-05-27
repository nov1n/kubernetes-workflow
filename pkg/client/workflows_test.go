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

package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kylelemons/godebug/pretty"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiErr "k8s.io/kubernetes/pkg/api/errors"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
	k8sTypes "k8s.io/kubernetes/pkg/types"
)

// getTestWorkflow returns a test workflow.
func getTestWorkflow() (api.Workflow, string) {
	name := "test-workflow"
	workflow := api.Workflow{
		TypeMeta: k8sApiUnv.TypeMeta{
			Kind:       "Workflow",
			APIVersion: "nerdalize.com/v1alpha1",
		},
		ObjectMeta: k8sApi.ObjectMeta{
			Name:      name,
			Namespace: k8sApi.NamespaceDefault,
			UID:       k8sTypes.UID("1uid1cafe"),
		},
		Spec: api.WorkflowSpec{
			ActiveDeadlineSeconds: func(i int64) *int64 { return &i }(3600),
			Steps: map[string]api.WorkflowStep{
				"step-a": api.WorkflowStep{
					JobTemplate: &k8sBatch.JobTemplateSpec{},
				},
				"step-b": api.WorkflowStep{
					Dependencies: []string{
						"step-a",
					},
					JobTemplate: &k8sBatch.JobTemplateSpec{},
				},
			},
			JobsSelector: &k8sApiUnv.LabelSelector{
				MatchLabels: map[string]string{"a": "b"},
			},
		},
	}
	return workflow, name
}

// getTestWorkflowList returns a list containing one test workflow.
func getTestWorkflowList() api.WorkflowList {
	wf, _ := getTestWorkflow()
	return api.WorkflowList{
		TypeMeta: k8sApiUnv.TypeMeta{
			Kind: "WorkflowList",
		},
		Items: []api.Workflow{wf},
	}
}

// getClient returns a ThirdPartyClient that always returns the given output string as a response
// to a request
func getClient(output string) (tpc *ThirdPartyClient, err error) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, output)
	}))
	tpc, err = NewThirdParty(k8sApiUnv.GroupVersion{
		Group:   "nerdalize.com",
		Version: "v1alpha1",
	}, k8sRestCl.Config{
		Host: ts.URL,
	})
	return
}

// getWatchClient returns a ThirdPartyClient that always writes whatever it
// receives on ch as a response to a request.
func getWatchClient(ch chan string) (tpc *ThirdPartyClient, err error) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := <-ch
		fmt.Fprintln(w, output)

	}))
	tpc, err = NewThirdParty(k8sApiUnv.GroupVersion{
		Group:   "nerdalize.com",
		Version: "v1alpha1",
	}, k8sRestCl.Config{
		Host: ts.URL,
	})
	return
}

// TestList tests whether the json corresponds to the struct structure defined in api/types.go
func TestList(t *testing.T) {
	expected := getTestWorkflowList()
	json, err := json.Marshal(expected)
	if err != nil {
		t.Errorf("Error when trying to parse workflow list (%#v): %v", expected, err)
	}
	tpc, err := getClient(string(json))
	if err != nil {
		t.Error("Error while creating client")
	}
	opts := k8sApi.ListOptions{}
	list, err := tpc.Workflows(k8sApi.NamespaceAll).List(opts)
	if err != nil {
		t.Errorf("Error while listing workflows: %v", err)
	}
	if !reflect.DeepEqual(list, &expected) {
		t.Errorf("Returned list doesn't match expected list. Diff: \n%v\n", pretty.Compare(list, &expected))
	}
}

// TestGet tests whether the json corresponds to the struct structure defined in api/types.go
func TestGet(t *testing.T) {
	expected, workflowName := getTestWorkflow()
	json, err := json.Marshal(expected)
	if err != nil {
		t.Errorf("Error when trying to parse workflow (%#v): %v", expected, err)
	}
	tpc, err := getClient(string(json))
	if err != nil {
		t.Error("Error while creating client")
	}

	workflow, err := tpc.Workflows(k8sApi.NamespaceDefault).Get(workflowName)
	if err != nil {
		t.Errorf("Error while listing workflows: %v", err)
	}
	if !reflect.DeepEqual(workflow, &expected) {
		t.Errorf("Returned workflow doesn't match expected workflow. Diff: \n%v\n", pretty.Compare(workflow, &expected))
	}
}

//TODO: This test has unexpected flakes. It needs reviewing
// TestWatch tests the client's watch function.
// func TestWatch(t *testing.T) {
// 	initialList := getTestWorkflowList()
// 	patches := map[k8sWatch.EventType]func(api.WorkflowList) api.WorkflowList{
// 		k8sWatch.Added: func(w api.WorkflowList) api.WorkflowList {
// 			return w
// 		},
// 		k8sWatch.Modified: func(w api.WorkflowList) api.WorkflowList {
// 			var deadline int64 = 42
// 			w.Items[0].Spec.ActiveDeadlineSeconds = &deadline
// 			return w
// 		},
// 		k8sWatch.Deleted: func(w api.WorkflowList) api.WorkflowList {
// 			w.Items = []api.Workflow{}
// 			return w
// 		},
// 	}
//
// 	c := make(chan string, len(patches))
// 	tpc, err := getWatchClient(c)
//
// 	if err != nil {
// 		t.Error("Error while creating client")
// 	}
//
// 	opts := k8sApi.ListOptions{}
// 	ticker := make(chan time.Time)
// 	watchClient := newFakeWorkflows(tpc, k8sApi.NamespaceDefault, ticker)
// 	watchInterface, err := watchClient.Watch(opts)
// 	if err != nil {
// 		t.Error("Error watching")
// 	}
// 	for event, patch := range patches {
// 		newList := patch(initialList)
// 		newListJSON, err := json.Marshal(newList)
// 		if err != nil {
// 			t.Errorf("Error when trying to parse patched workflow list (%#v): %v", newList, err)
// 		}
// 		c <- string(newListJSON)
// 		ticker <- time.Now()
// 		received := <-watchInterface.ResultChan()
// 		if event != received.Type {
// 			t.Errorf("Watching expected event %v but got %v", event, received.Type)
// 		}
// 	}
// }

// TestUpdateSuccess tests for a succesfull client update.
func TestUpdateSuccess(t *testing.T) {
	expected, _ := getTestWorkflow()
	json, err := json.Marshal(expected)
	if err != nil {
		t.Errorf("Error when trying to parse workflow (%#v): %v", expected, err)
	}
	tpc, err := getClient(string(json))
	if err != nil {
		t.Error("Error while creating client")
	}

	workflow, err := tpc.Workflows(k8sApi.NamespaceDefault).Update(&expected)
	if err != nil {
		t.Errorf("Error while updating workflows: %v", err)
	}
	if !reflect.DeepEqual(workflow, &expected) {
		t.Errorf("Returned workflow doesn't match expected workflow. Diff: \n%v\n", pretty.Compare(workflow, &expected))
	}
}

// TestUpdateStatusFailure tests the case when the server returns a status
// error when a workflow is updated.
func TestUpdateStatusFailure(t *testing.T) {
	expected := k8sApiErr.NewConflict(k8sApiUnv.GroupResource{}, "test-error", fmt.Errorf("Test error"))
	json, err := json.Marshal(expected.ErrStatus)
	if err != nil {
		t.Errorf("Error when trying to parse workflow (%#v): %v", expected, err)
	}
	tpc, err := getClient(string(json))
	if err != nil {
		t.Error("Error while creating client")
	}

	workflow, _ := getTestWorkflow()
	_, err = tpc.Workflows(k8sApi.NamespaceDefault).Update(&workflow)
	if err != nil {
		if serr, ok := err.(*k8sApiErr.StatusError); ok {
			if !reflect.DeepEqual(serr, expected) {
				t.Errorf("Returned status error doesn't match expected status error. Diff: \n%v\n", pretty.Compare(serr, expected))
			}
			return
		}
	}
	t.Errorf("Expected status error but got: %v", err)
}

// TestUpdateNoName tests the case when Update should return an error because
// the given workflow has no name.
func TestUpdateNoName(t *testing.T) {
	expectedError := "no name found in workflow"
	tpc, err := getClient("")
	if err != nil {
		t.Error("Error while creating client")
	}

	workflow, _ := getTestWorkflow()
	workflow.Name = ""
	_, err = tpc.Workflows(k8sApi.NamespaceDefault).Update(&workflow)
	if err == nil || err.Error() != expectedError {
		t.Errorf("Expected error message %v, but got %v", expectedError, err)
	}
}

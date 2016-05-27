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

package api

import (
	"path"
	"reflect"
	"testing"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
)

var expected = &k8sApiUnv.TypeMeta{
	Kind:       Kind,
	APIVersion: path.Join(Group, Version),
}

func TestWorkflowObjectKind(t *testing.T) {
	wf := Workflow{
		TypeMeta:   k8sApiUnv.TypeMeta{},
		ObjectMeta: k8sApi.ObjectMeta{},
		Spec:       WorkflowSpec{},
		Status:     WorkflowStatus{},
	}

	objKind := wf.GetObjectKind()
	equal := reflect.DeepEqual(objKind, expected)
	if !equal {
		t.Errorf("Error getting Workflow objectkind, got %v, expected %v", objKind, expected)
	}
}

func TestWorkflowListObjectKind(t *testing.T) {
	wfList := WorkflowList{
		TypeMeta: k8sApiUnv.TypeMeta{},
		ListMeta: k8sApiUnv.ListMeta{},
		Items: []Workflow{
			Workflow{
				TypeMeta:   k8sApiUnv.TypeMeta{},
				ObjectMeta: k8sApi.ObjectMeta{},
				Spec:       WorkflowSpec{},
				Status:     WorkflowStatus{},
			},
		},
	}

	objKind := wfList.GetObjectKind()
	equal := reflect.DeepEqual(objKind, expected)
	if !equal {
		t.Errorf("Error getting WorkflowList objectkind, got %v, expected %v", objKind, expected)
	}
}

func TestSetLabels(t *testing.T) {
	wf := Workflow{
		TypeMeta:   k8sApiUnv.TypeMeta{},
		ObjectMeta: k8sApi.ObjectMeta{},
		Spec:       WorkflowSpec{},
		Status:     WorkflowStatus{},
	}

	labels := map[string]string{"foo": "bar"}

	wf.SetLabels(labels)

	if val := wf.Labels["foo"]; val != "bar" {
		t.Errorf("Error setting labels, expected %v, got %v", labels["foo"], val)
	}
}

func TestSetUID(t *testing.T) {
	wf := Workflow{
		TypeMeta:   k8sApiUnv.TypeMeta{},
		ObjectMeta: k8sApi.ObjectMeta{},
		Spec:       WorkflowSpec{},
		Status:     WorkflowStatus{},
	}

	wf.SetUID()

	if val := wf.Labels[WorkflowUIDLabel]; val != string(wf.UID) {
		t.Errorf("Error setting labels, expected %v, got %v", string(wf.UID), val)
	}
}

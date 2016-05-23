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
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kylelemons/godebug/pretty"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
	k8sTypes "k8s.io/kubernetes/pkg/types"
)

const jsonList = `{"kind": "WorkflowList","items": [
{
  "apiVersion": "nerdalize.com/v1alpha1",
  "kind": "Workflow",
  "metadata": {
    "name": "test-workflow",
	"namespace": "default",
	"uid": "1uid1cafe"
  },
  "spec": {
    "activeDeadlineSeconds": 3600,
    "steps": {
      "step-a": {
        "jobTemplate": {
          "metadata": {
            "name": "job1"
          },
          "spec": {
            "parallelism": 1,
            "template": {
              "metadata": {
                "name": "pod1"
              },
              "spec": {
                "restartPolicy": "OnFailure",
                "containers": [
                  {
                    "image": "ubuntu",
                    "name": "ubuntu1",
                    "command": [
                      "/bin/sleep", "30"
                    ]
                  }
                ]
              }
            }
          }
        }
      },
      "step-b": {
        "dependencies": [
          "step-a"
        ],
        "jobTemplate": {
          "metadata": {
            "name": "job2"
          },
          "spec": {
            "parallelism": 1,
            "template": {
              "metadata": {
                "name": "pod2"
              },
              "spec": {
                "restartPolicy": "OnFailure",
                "containers": [
                  {
                    "image": "ubuntu",
                    "name": "ubuntu2",
                    "command": [
                      "/bin/sleep", "30"
                    ]
                  }
                ]
              }
            }
          }
        }
      }
    },
	"jobsSelector": {
		"matchLabels": {
			"a": "b"
		}
	}
  }
}
]}`

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

func TestList(t *testing.T) {
	expected := api.WorkflowList{
		TypeMeta: k8sApiUnv.TypeMeta{
			Kind: "WorkflowList",
		},
		Items: []api.Workflow{api.Workflow{
			TypeMeta: k8sApiUnv.TypeMeta{
				Kind:       "Workflow",
				APIVersion: "nerdalize.com/v1alpha1",
			},
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "test-workflow",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				ActiveDeadlineSeconds: func(i int64) *int64 { return &i }(3600),
				Steps: map[string]api.WorkflowStep{
					"step-a": api.WorkflowStep{
						JobTemplate: &k8sBatch.JobTemplateSpec{
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
						},
					},
					"step-b": api.WorkflowStep{
						Dependencies: []string{
							"step-a",
						},
						JobTemplate: &k8sBatch.JobTemplateSpec{
							ObjectMeta: k8sApi.ObjectMeta{
								Name: "job2",
							},
							Spec: k8sBatch.JobSpec{
								Parallelism: func(i int32) *int32 { return &i }(1),
								Template: k8sApi.PodTemplateSpec{
									ObjectMeta: k8sApi.ObjectMeta{
										Name: "pod2",
									},
									Spec: k8sApi.PodSpec{
										RestartPolicy: "OnFailure",
										Containers: []k8sApi.Container{
											k8sApi.Container{
												Name:  "ubuntu2",
												Image: "ubuntu",
												Command: []string{
													"/bin/sleep", "30",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				JobsSelector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		},
	}
	tpc, err := getClient(jsonList)
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

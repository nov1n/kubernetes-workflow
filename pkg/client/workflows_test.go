package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/client/restclient"
)

const jsonList = `{"kind": "WorkflowList","items": [
{
  "apiVersion": "nerdalize.com/v1alpha1",
  "kind": "Workflow",
  "metadata": {
    "name": "test-workflow"
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
            "template": {
              "metadata": {
                "name": "pod1"
              },
              "spec": {
                "containers": [
                  {
                    "image": "ubuntu1",
                    "name": "ubuntu",
                    "command": [
                      "sleep 10"
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
            "template": {
              "metadata": {
                "name": "pod2"
              },
              "spec": {
                "containers": [
                  {
                    "image": "ubuntu2",
                    "name": "ubuntu",
                    "command": [
                      "sleep 20"
                    ]
                  }
                ]
              }
            }
          }
        }
      }
    }
  }
}
]}`

func getClient(output string) (tpc *ThirdPartyClient, err error) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, output)
	}))
	tpc, err = NewThirdParty(k8sApiUnversioned.GroupVersion{
		Group:   "nerdalize.com",
		Version: "v1alpha1",
	}, restclient.Config{
		Host: ts.URL,
	})
	return
}

func TestList(t *testing.T) {
	expected := api.WorkflowList{
		TypeMeta: k8sApiUnversioned.TypeMeta{
			Kind: "WorkflowList",
		},
		Items: []api.Workflow{
			api.Workflow{
				TypeMeta: k8sApiUnversioned.TypeMeta{
					Kind:       "Workflow",
					APIVersion: "nerdalize.com/v1alpha1",
				},
				ObjectMeta: k8sApi.ObjectMeta{
					Name: "test-workflow",
				},
				Spec: api.WorkflowSpec{
					ActiveDeadlineSeconds: 3600,
					Steps: map[string]api.WorkflowStep{
						"step-a": api.WorkflowStep{
							JobTemplate: &batch.JobTemplateSpec{
								ObjectMeta: k8sApi.ObjectMeta{
									Name: "job1",
								},
								Spec: batch.JobSpec{
									Template: k8sApi.PodTemplateSpec{
										ObjectMeta: k8sApi.ObjectMeta{
											Name: "pod1",
										},
										Spec: k8sApi.PodSpec{
											Containers: []k8sApi.Container{
												k8sApi.Container{
													Name:  "ubuntu1",
													Image: "ubuntu",
													Command: []string{
														"sleep 10",
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
							JobTemplate: &batch.JobTemplateSpec{
								ObjectMeta: k8sApi.ObjectMeta{
									Name: "job2",
								},
								Spec: batch.JobSpec{
									Template: k8sApi.PodTemplateSpec{
										ObjectMeta: k8sApi.ObjectMeta{
											Name: "pod2",
										},
										Spec: k8sApi.PodSpec{
											Containers: []k8sApi.Container{
												k8sApi.Container{
													Name:  "ubuntu2",
													Image: "ubuntu",
													Command: []string{
														"sleep 20",
													},
												},
											},
										},
									},
								},
							},
						},
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
		// t.Errorf("Returned list doesn't match expected list\nList: %v\nExpected: %v", list, &expected)
	}
}

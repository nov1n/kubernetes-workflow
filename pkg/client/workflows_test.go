package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
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
   "steps": [
     {
       "name": "step-a",
       "jobTemplate": {
         "containers": [
           {
             "name": "pi",
             "image": "perl",
             "command": [
               "perl",
               "print bpi(2000)"
             ],
             "restartPolicy": "Never"
           },
           {
             "name": "nginx",
             "image": "nginx"
           }
         ]
       },
       "dependencies": [
         {
           "stepName": "step-b",
           "status": "success"
         }
       ]
     },
     {
       "name": "step-b",
       "jobTemplate": {
         "containers": [
           {
             "name": "git-monitor",
             "image": "kubernetes/git-monitor"
           },
           {
             "name": "logger",
             "image": "google/glog"
           }
         ]
       }
     }
   ]
 }
}
]}`

func getClient(output string) *ThirdPartyClient {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, output)
	}))
	tpc, err := client.NewThirdParty(k8sApiUnversioned.GroupVersion{
		Group:   "nerdalize.com",
		Version: "v1alpha1",
	}, restclient.Config{
		Host: ts.URL,
	})
	return tpc
}

func TestList(t *testing.T) {
	expected := api.WorkflowList{
		TypeMeta: k8sApiUnversioned.TypeMeta{
			Kind: "WorkflowList",
		},
		Items: []api.Workflow{
			TypeMeta: k8sApiUnversioned.TypeMeta{
				Kind:       "Workflow",
				APIVersion: "nerdalize.com/v1alpha1",
			},
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "test-workflow1",
				Namespace: "default",
			},
		},
	}
	tpc := getClient(jsonList)
	opts := &k8sApi.ListOptions{}
	list, err := tpc.Workflows("default").List(opts)

}

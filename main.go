package main

import (
	"fmt"

	"net"

	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/workflow"

	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
	k8sController "k8s.io/kubernetes/pkg/controller"
)

// host and port for kubie api
const (
	HOST = "localhost"
	PORT = "8080"
)

func main() {
	clientConfig := restclient.Config{
		Host: "http://" + net.JoinHostPort(HOST, PORT),
	}
	thirdPartyClient, err := client.NewThirdParty(k8sApiUnversioned.GroupVersion{
		Group:   "nerdalize.com",
		Version: "v1alpha1",
	}, clientConfig)
	if err != nil {
		fmt.Println("Couldn't create 3rd-party client: ", err)
		return
	}

	client, err := clientset.NewForConfig(&clientConfig)
	if err != nil {
		fmt.Println("Couldn't create set client: ", err)
		return
	}
	oldClient, err := k8sClient.New(&clientConfig)
	if err != nil {
		fmt.Println("Couldn't create batch client: ", err)
		return
	}

	manager := workflow.NewWorkflowManager(oldClient, client, thirdPartyClient, k8sController.NoResyncPeriodFunc)
	stopChan := make(chan struct{})
	manager.Run(5, stopChan)
	<-stopChan
	fmt.Println("end")
}

package main

import (
	"flag"
	"fmt"

	"net"

	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/workflow"

	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/dynamic"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
	k8sController "k8s.io/kubernetes/pkg/controller"
)

func main() {
	// Parse cmdline flags
	host := flag.String("host", "127.0.0.1", "IP address of kubernetes API server")
	port := flag.String("port", "8080", "Port of the kubernetes API server")

	flag.Parse()

	// Create client
	clientConfig := restclient.Config{
		Host: "http://" + net.JoinHostPort(*host, *port),
	}

	groupVersion := k8sApiUnversioned.GroupVersion{
		Group:   "nerdalize.com",
		Version: "v1alpha1",
	}

	pool := dynamic.NewClientPool(clientConfig, dynamic.LegacyAPIPathResolverFunc)
	pool.ClientForGroupVersion(groupVersion)

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

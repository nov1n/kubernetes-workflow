package main

import (
	"fmt"

	"net"

	"github.com/nov1n/kubernetes-workflow/client"
	"github.com/nov1n/kubernetes-workflow/job"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
)

// host and port for kubie api
const (
	HOST = "localhost"
	PORT = "8080"
)

func main() {
	cl, err := client.NewRESTClient("127.0.0.1", "8080")

	if err != nil {
		panic(err)
	}

	jobManager := job.Manager{
		Client:    cl,
		Namespace: "my-workflows",
	}

	fmt.Print("Created namespace", jobManager.Namespace)

	thirdPartyClient, err := client.NewThirdparty(&k8sApiUnversioned.GroupVersion{
		Group:   "nerdalize.com",
		Version: "v1alpha1",
	}, &restclient.Config{
		Host: "http://" + net.JoinHostPort(HOST, PORT),
	})
	if err != nil {
		fmt.Println("Couldn't create 3rd-party client: ", err)
		return
	}
	opts := k8sApi.ListOptions{}
	w, err := thirdPartyClient.Workflows("default").Watch(opts)
	if err != nil {
		fmt.Println("Couldn't watch workflows: ", err)
		return
	}
	fmt.Println("Watching..")
	for res := range w.ResultChan() {
		fmt.Println(res.Object)
	}
}

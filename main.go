package main

import (
	"flag"
	"fmt"

	"net"

	"github.com/nov1n/kubernetes-workflow/pkg/client"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
)

func main() {
	// Parse cmdline flags
	host := flag.String("host", "127.0.0.1", "IP address of kubernetes API server")
	port := flag.String("port", "8080", "Port of the kubernetes API server")

	flag.Parse()

	// Create client
	thirdPartyClient, err := client.NewThirdParty(k8sApiUnversioned.GroupVersion{
		Group:   "nerdalize.com",
		Version: "v1alpha1",
	}, restclient.Config{
		Host: "http://" + net.JoinHostPort(*host, *port),
	})
	if err != nil {
		fmt.Println("Couldn't create 3rd-party client: ", err)
		return
	}

	// Watch workflows
	opts := k8sApi.ListOptions{}
	w, err := thirdPartyClient.Workflows(k8sApi.NamespaceDefault).Watch(opts)
	if err != nil {
		fmt.Println("Couldn't watch workflows: ", err)
		return
	}
	fmt.Println("Watching..")
	for res := range w.ResultChan() {
		fmt.Println(res.Object)
	}
}

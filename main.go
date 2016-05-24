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

package main

import (
	"flag"

	"net"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/workflow"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
	k8sApiExtensions "k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	WorkflowKind        = "Workflow"
	WorkflowResource    = "workflow"
	WorkflowAPIGroup    = "nerdalize.com"
	WorkflowAPIVersion  = "v1alpha1"
	WorkflowAPIVersions = []k8sApiExtensions.APIVersion{
		"v1alpha1",
	}
	WorkflowDescription = "An API endpoint for workflows"
)

func main() {
	// Flush any buffered logs on exit
	defer glog.Flush()

	// Parse cmdline flags
	host := flag.String("host", "127.0.0.1", "IP address of kubernetes API server")
	port := flag.String("port", "8080", "Port of the kubernetes API server")
	flag.Parse()

	// Configure host using the cmdline flags
	clientConfig := k8sRestCl.Config{
		Host: "http://" + net.JoinHostPort(*host, *port),
	}

	// Create thirdparty client to manage third party resources
	thirdPartyClient, err := client.NewThirdParty(k8sApiUnversioned.GroupVersion{
		Group:   api.Group,
		Version: api.Version,
	}, clientConfig)
	if err != nil {
		glog.Fatalf("Could not create 3rd party client: %v", err)
	}

	// Create clientset holding multiple different clients
	client, err := clientset.NewForConfig(&clientConfig)
	if err != nil {
		glog.Fatalf("Could not create set client: %v", err)
	}

	// Create old client to manage batch resources (e.g. jobs)
	oldClient, err := k8sClient.New(&clientConfig)
	if err != nil {
		glog.Fatalf("Couldn not create batch client: %v", err)
	}

	glog.V(3).Infof("Clients initialized")

	manager := workflow.NewManager(oldClient, client, thirdPartyClient)
	stopChan := make(chan struct{})
	manager.Run(5, stopChan)
	<-stopChan
}

func registerThirdPartyResource(client k8sClient.Interface) error {
	opts := k8sApi.ListOptions{
		TypeMeta: k8sApiUnversioned.TypeMeta{
			Kind:       WorkflowKind,
			APIVersion: WorkflowAPIGroup + "/" + WorkflowAPIVersion,
		},
	}
	list, err := client.Extensions().ThirdPartyResources().List(opts)
	if err != nil {
		return fmt.Errorf("couldn't do initial list of third party resources: %v", err)
	}

	switch len(list.Items) {
	case 1:
		return nil
	case 0:
		config := &k8sApiExtensions.ThirdPartyResource{
			ObjectMeta: k8sApi.ObjectMeta{
				Name: WorkflowResource + "." + WorkflowAPIGroup,
			},
			Description: WorkflowDescription,
			Versions:    WorkflowAPIVersions,
		}

		_, err := client.Extensions().ThirdPartyResources().Create(config)
		if err != nil {
			return fmt.Errorf("couldn't create third party resource: %v", err)
		}
	default:
		return fmt.Errorf("found too many items when listing third party resources: %d items found", len(list.Items))
	}

	return nil
}

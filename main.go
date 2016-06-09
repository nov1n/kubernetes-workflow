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
	"fmt"
	"time"

	"net"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/client"
	"github.com/nov1n/kubernetes-workflow/pkg/workflow"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiErr "k8s.io/kubernetes/pkg/api/errors"
	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
	k8sApiExtensions "k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	thirdPartyResourceRetryOnFail = 5
	thirdPartyResourceRetryPeriod = 5 * time.Second
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

	glog.V(3).Infof("Clients initialized.")

	err = registerThirdPartyResource(client)
	if err != nil {
		glog.Errorf("Couldn't initialize tpr, shutting down.")
		return
	}

	glog.V(3).Infof("ThirdPartyResource registered.")

	manager := workflow.NewManager(oldClient, client, thirdPartyClient)
	stopChan := make(chan struct{})
	manager.Run(5, stopChan)
	<-stopChan
}

func registerThirdPartyResource(client clientset.Interface) error {
	name := api.Resource + "." + api.Group
	var err error

	// Try to get the TPR from the server. In case the server is not up yet,
	// when the proxy is not up yet, or when some other unexpected error
	// occurs, a GET is tried again after thirdPartyResourceRetryPeriod.
	for {
		_, err = client.Extensions().ThirdPartyResources().Get(name)
		// No error means API server is accessible and workflow tpr is already registered.
		if err == nil {
			glog.V(3).Infof("No errors when getting tpr.")
			return nil
		}
		// Status error StatusReasonNotFound means the API server is accessible,
		// but workflow tpr is not yet registered.
		serr, ok := err.(*k8sApiErr.StatusError)
		if ok && serr.Status().Reason == k8sApiUnversioned.StatusReasonNotFound {
			break
		}
		// Probably the API server is not accessible (yet).
		glog.Errorf("Received unknown error when trying to acces API server: %v. Retrying..", err)
		time.Sleep(thirdPartyResourceRetryPeriod)
	}

	// if we got a status error indicating the resource was not found
	// a.k.a. the tpr was not registered yet.
	if serr, ok := err.(*k8sApiErr.StatusError); ok && serr.Status().Reason == k8sApiUnversioned.StatusReasonNotFound {
		config := &k8sApiExtensions.ThirdPartyResource{
			ObjectMeta: k8sApi.ObjectMeta{
				Name: name,
			},
			Description: api.Description,
			Versions:    api.Versions,
		}

		var createErr error
		for i := 0; i < thirdPartyResourceRetryOnFail; i++ {
			glog.V(3).Infof("Creating third party resource.")
			_, createErr = client.Extensions().ThirdPartyResources().Create(config)
			if createErr == nil {
				return nil
			}
			glog.Errorf("Error while trying to create third party resource on try %v/%v: %v", i, thirdPartyResourceRetryOnFail, createErr)
			time.Sleep(thirdPartyResourceRetryPeriod)
		}
		return fmt.Errorf("couldn't create third party resource: %v", createErr)

	}
	return fmt.Errorf("couldn't do initial list of third party resources: %v", err)
}

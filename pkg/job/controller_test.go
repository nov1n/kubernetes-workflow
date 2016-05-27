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

package job

//
// import (
// 	"fmt"
// 	"net"
// 	"testing"
//
// 	k8sApi "k8s.io/kubernetes/pkg/api"
// 	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
// 	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
// 	k8sClUnv "k8s.io/kubernetes/pkg/client/unversioned"
// )
//
// const (
// 	NAMESPACE = "my-workflows"
// )
//
// func TestJobCreation(t *testing.T) {
// 	// Create rest client
// 	config := &k8sRestCl.Config{
// 		Host: "http://" + net.JoinHostPort("127.0.0.1", "8080"),
// 	}
//
// 	restClient, err := k8sClUnv.New(config)
// 	if err != nil {
// 		t.Error("Could not create REST client.")
// 	}
//
// 	// Create namespace
// 	namespace := &k8sApi.Namespace{
// 		ObjectMeta: k8sApi.ObjectMeta{
// 			Name: NAMESPACE,
// 		},
// 	}
// 	_, err = restClient.Namespaces().Create(namespace)
// 	if err != nil {
// 		t.Error(fmt.Sprintf("Could not create namespace '%s': %s", NAMESPACE, err))
// 	}
//
// 	// Add job
// 	jobManager := &Manager{
// 		Client:    restClient,
// 		Namespace: NAMESPACE,
// 	}
//
// 	jobConfig := &k8sBatch.Job{
// 		ObjectMeta: k8sApi.ObjectMeta{
// 			Name: "my-job",
// 		},
// 		Spec: k8sBatch.JobSpec{
// 			Template: k8sApi.PodTemplateSpec{
// 				ObjectMeta: k8sApi.ObjectMeta{
// 					Name: "my-job",
// 				},
// 				Spec: k8sApi.PodSpec{
// 					Containers: []k8sApi.Container{
// 						k8sApi.Container{
// 							Name:  "nginx",
// 							Image: "nginx",
// 							Ports: []k8sApi.ContainerPort{
// 								k8sApi.ContainerPort{
// 									ContainerPort: 80,
// 								},
// 							},
// 						},
// 					},
// 					RestartPolicy: "Never",
// 				},
// 			},
// 		},
// 	}
//
// 	_, err = jobManager.AddJob(jobConfig)
//
// 	if err != nil {
// 		t.Error(fmt.Sprintf("Could not add job: %v", err))
// 	}
//
// 	// Remove job
// 	err = jobManager.RemoveJob(jobConfig)
// 	if err != nil {
// 		t.Error(fmt.Sprintf("Could not delete job: %v", err))
// 	}
//
// 	// Cleanup
// 	err = restClient.Namespaces().Delete(NAMESPACE)
// 	if err != nil {
// 		t.Error(fmt.Sprintf("Could not delete namespace '%s': %s", NAMESPACE, err))
// 	}
// }

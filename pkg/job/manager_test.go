package job

import (
	"fmt"
	"net"
	"testing"

	k8sApi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	k8sRestClient "k8s.io/kubernetes/pkg/client/restclient"
	k8sClientUnversioned "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	NAMESPACE = "my-workflows"
)

func TestJobCreation(t *testing.T) {
	// Create rest client
	config := &k8sRestClient.Config{
		Host: "http://" + net.JoinHostPort("127.0.0.1", "8080"),
	}

	restClient, err := k8sClientUnversioned.New(config)
	if err != nil {
		t.Error("Could not create REST client.")
	}

	// Create namespace
	namespace := &k8sApi.Namespace{
		ObjectMeta: k8sApi.ObjectMeta{
			Name: NAMESPACE,
		},
	}
	_, err = restClient.Namespaces().Create(namespace)
	if err != nil {
		t.Error(fmt.Sprintf("Could not create namespace '%s': %s", NAMESPACE, err))
	}

	// Add job
	jobManager := &Manager{
		Client:    restClient,
		Namespace: NAMESPACE,
	}

	jobConfig := &batch.Job{
		ObjectMeta: k8sApi.ObjectMeta{
			Name: "my-job",
		},
		Spec: batch.JobSpec{
			Template: k8sApi.PodTemplateSpec{
				ObjectMeta: k8sApi.ObjectMeta{
					Name: "my-job",
				},
				Spec: k8sApi.PodSpec{
					Containers: []k8sApi.Container{
						k8sApi.Container{
							Name:  "nginx",
							Image: "nginx",
							Ports: []k8sApi.ContainerPort{
								k8sApi.ContainerPort{
									ContainerPort: 80,
								},
							},
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	_, err = jobManager.AddJob(jobConfig)

	if err != nil {
		t.Error(fmt.Sprintf("Could not add job: %v", err))
	}

	// Remove job
	err = jobManager.RemoveJob(jobConfig)
	if err != nil {
		t.Error(fmt.Sprintf("Could not delete job: %v", err))
	}

	// Cleanup
	err = restClient.Namespaces().Delete(NAMESPACE)
	if err != nil {
		t.Error(fmt.Sprintf("Could not delete namespace '%s': %s", NAMESPACE, err))
	}
}

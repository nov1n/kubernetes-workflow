/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"reflect"
	"time"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/watch"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiErr "k8s.io/kubernetes/pkg/api/errors"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sWatch "k8s.io/kubernetes/pkg/watch"
)

const (
	// TODO: Use these constants instead of strings

	// Api path for namespaces
	namespacesPathString = "namespaces"
	// Api path for workflows
	workflowsPathString = "workflows"
)

// WorkflowsNamespacer has methods to work with Workflow resources in a namespace.
type WorkflowsNamespacer interface {
	Workflows(namespace string) WorkflowInterface
}

// WorkflowInterface has methods to work with Workflow resources.
type WorkflowInterface interface {
	List(opts k8sApi.ListOptions) (*api.WorkflowList, error)
	Get(name string) (*api.Workflow, error)
	// Delete(name string, options *api.DeleteOptions) error
	// Create(pod *api.Pod) (*api.Pod, error)
	Update(workflow *api.Workflow) (*api.Workflow, error)
	Watch(opts k8sApi.ListOptions) (k8sWatch.Interface, error)
	// Bind(binding *api.Binding) error
	UpdateStatus(workflow *api.Workflow) (*api.Workflow, error)
	// GetLogs(name string, opts *api.PodLogOptions) *restclient.Request
}

// workflows implements WorkflowsNamespacer interface
type workflows struct {
	client  *ThirdPartyClient
	ns      string
	nameMap map[string]api.Workflow
}

// newWorkflows returns a new workflows object given a client and a namespace
func newWorkflows(c *ThirdPartyClient, namespace string) *workflows {
	return &workflows{
		client:  c,
		ns:      namespace,
		nameMap: make(map[string]api.Workflow),
	}
}

// Update updates the given workflow in the cluster
func (w *workflows) Update(workflow *api.Workflow) (result *api.Workflow, err error) {
	return w.UpdateWithSubresource(workflow, "")
}

// UpdateStatus updates the status field for a given workflow TODO: Is this correct?
func (w *workflows) UpdateStatus(workflow *api.Workflow) (result *api.Workflow, err error) {
	return w.UpdateWithSubresource(workflow, "")
}

// UpdateWithSubresource updates a given workflows subresource by communicating with the kubernetes
// API server through the client
func (w *workflows) UpdateWithSubresource(workflow *api.Workflow, subresource string) (result *api.Workflow, err error) {
	nsPath := ""
	if w.ns != "" {
		nsPath = path.Join("namespaces", w.ns)
	}
	if workflow.Name == "" {
		return nil, fmt.Errorf("no name found in workflow")
	}
	url := createURL(w.client.baseURL, nsPath, "workflows", workflow.Name, subresource)
	b, err := json.Marshal(workflow)
	if err != nil {
		return nil, fmt.Errorf("couldn't encode workflow: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("could not reach create request: %v", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %v", url, err)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read from response body: %v", err)
	}
	result = &api.Workflow{}
	err = json.Unmarshal(data, result)
	if err != nil {
		status := &k8sApiUnv.Status{}
		err = json.Unmarshal(data, status)
		if err == nil {
			return nil, &k8sApiErr.StatusError{ErrStatus: *status}
		}
		return nil, fmt.Errorf("could not decode into api.Workflow or k8sApiUnv.Status: %v", err)
	}
	return
}

// List returns a WorkflowList containing all workflows in the workflows namespace
func (w *workflows) List(opts k8sApi.ListOptions) (result *api.WorkflowList, err error) {
	nsPath := ""
	if w.ns != "" {
		nsPath = path.Join("namespaces", w.ns)
	}
	url := createURL(w.client.baseURL, nsPath, "workflows")
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %v", url, err)
	}
	dec := json.NewDecoder(resp.Body)
	result = &api.WorkflowList{}
	err = dec.Decode(&result)
	return
}

func (w *workflows) Get(name string) (result *api.Workflow, err error) {
	nsPath := ""
	if w.ns != "" {
		nsPath = path.Join("namespaces", w.ns)
	}
	url := createURL(w.client.baseURL, nsPath, "workflows", name)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %v", url, err)
	}
	dec := json.NewDecoder(resp.Body)
	result = &api.Workflow{}
	err = dec.Decode(&result)
	return
}

// Watch returns a watch.Interface that watches the requested workflows.
func (w *workflows) Watch(opts k8sApi.ListOptions) (k8sWatch.Interface, error) {
	watcher := watch.NewThirdPartyWatcher()
	ticker := time.NewTicker(time.Millisecond * 1000)
	go func() {
		for range ticker.C {
			list, err := w.List(opts)
			if err != nil {
				// TODO: Do logging
				return
			}
			listMap := make(map[string]api.Workflow)
			for _, wf := range list.Items {
				listMap[wf.Name] = wf
				if _, ok := w.nameMap[wf.Name]; ok == false {
					watcher.Result <- k8sWatch.Event{
						Type:   k8sWatch.Added,
						Object: &wf,
					}
				} else {
					if reflect.DeepEqual(w.nameMap[wf.Name], wf) == false {
						watcher.Result <- k8sWatch.Event{
							Type:   k8sWatch.Modified,
							Object: &wf,
						}
					}
				}
				w.nameMap[wf.Name] = wf
			}
			for k, wf := range w.nameMap {
				if _, ok := listMap[k]; ok == false {
					watcher.Result <- k8sWatch.Event{
						Type:   k8sWatch.Deleted,
						Object: &wf,
					}
					delete(w.nameMap, k)
				}
			}
		}
	}()
	return watcher, nil
}

func createURL(base string, endpoint ...string) string {
	return base + "/" + path.Join(endpoint...)
}

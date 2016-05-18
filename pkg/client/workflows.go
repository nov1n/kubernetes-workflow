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

package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	"github.com/nov1n/kubernetes-workflow/pkg/watch"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sWatch "k8s.io/kubernetes/pkg/watch"
)

// WorkflowsNamespacer has methods to work with Workflow resources in a namespace.
type WorkflowsNamespacer interface {
	Workflows(namespace string) WorkflowInterface
}

// WorkflowInterface has methods to work with Workflow resources.
type WorkflowInterface interface {
	List(opts k8sApi.ListOptions) (*api.WorkflowList, error)
	// Get(name string) (*api.Pod, error)
	// Delete(name string, options *api.DeleteOptions) error
	// Create(pod *api.Pod) (*api.Pod, error)
	// Update(pod *api.Pod) (*api.Pod, error)
	Watch(opts k8sApi.ListOptions) (k8sWatch.Interface, error)
	// Bind(binding *api.Binding) error
	// UpdateStatus(pod *api.Pod) (*api.Pod, error)
	// GetLogs(name string, opts *api.PodLogOptions) *restclient.Request
}

// workflows implements WorkflowsNamespacer interface
type workflows struct {
	client  *ThirdPartyClient
	ns      string
	nameMap map[string]api.Workflow
}

// newPods returns a pods
func newWorkflows(c *ThirdPartyClient, namespace string) *workflows {
	return &workflows{
		client:  c,
		ns:      namespace,
		nameMap: make(map[string]api.Workflow),
	}
}

func (w *workflows) List(opts k8sApi.ListOptions) (result *api.WorkflowList, err error) {
	nsPath := ""
	if w.ns != "" {
		nsPath = "/namespaces/" + w.ns
	}
	url := w.client.baseURL + nsPath + "/workflows"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %v", url, err)
	}
	dec := json.NewDecoder(resp.Body)
	result = &api.WorkflowList{}
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
					// TODO: Add changed event
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

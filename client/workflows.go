package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nov1n/kubernetes-workflow/api"
	"github.com/nov1n/kubernetes-workflow/watch"

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
	r     *ThirdPartyClient
	ns    string
	wfMap map[string]api.Workflow
}

// newPods returns a pods
func newWorkflows(c *ThirdPartyClient, namespace string) *workflows {
	return &workflows{
		r:     c,
		ns:    namespace,
		wfMap: make(map[string]api.Workflow),
	}
}

func (c *workflows) List(opts k8sApi.ListOptions) (result *api.WorkflowList, err error) {
	url := c.r.baseURL + "/namespaces/" + c.ns + "/workflows"
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
func (c *workflows) Watch(opts k8sApi.ListOptions) (k8sWatch.Interface, error) {
	watcher := watch.NewThirdPartyWatcher()
	ticker := time.NewTicker(time.Millisecond * 1000)
	go func() {
		for range ticker.C {
			list, err := c.List(opts)
			if err != nil {
				// TODO: Do logging
				return
			}
			listMap := make(map[string]api.Workflow)
			for _, wf := range list.Items {
				listMap[wf.Name] = wf
				if _, ok := c.wfMap[wf.Name]; ok == false {
					watcher.Result <- k8sWatch.Event{
						Type:   k8sWatch.Added,
						Object: &wf,
					}
				} else {
					// TODO: Add changed event
				}
				c.wfMap[wf.Name] = wf
			}
			for k, wf := range c.wfMap {
				if _, ok := listMap[k]; ok == false {
					watcher.Result <- k8sWatch.Event{
						Type:   k8sWatch.Deleted,
						Object: &wf,
					}
					delete(c.wfMap, k)
				}
			}
		}
	}()
	return watcher, nil
}

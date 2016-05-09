package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nov1n/kubernetes-workflow/api"

	k8sApi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/thirdpartyresourcedata"
	"k8s.io/kubernetes/pkg/watch"
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
	Watch(opts k8sApi.ListOptions) (watch.Interface, error)
	// Bind(binding *api.Binding) error
	// UpdateStatus(pod *api.Pod) (*api.Pod, error)
	// GetLogs(name string, opts *api.PodLogOptions) *restclient.Request
}

// workflows implements WorkflowsNamespacer interface
type workflows struct {
	r  *ThirdPartyClient
	ns string
}

// newPods returns a pods
func newWorkflows(c *ThirdPartyClient, namespace string) *workflows {
	return &workflows{
		r:  c,
		ns: namespace,
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

// Watch returns a watch.Interface that watches the requested pods.
func (c *workflows) Watch(opts k8sApi.ListOptions) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Namespace("default").
		Resource("workflows").
		VersionedParams(&opts, thirdpartyresourcedata.NewThirdPartyParameterCodec(k8sApi.ParameterCodec)).
		Watch()
}

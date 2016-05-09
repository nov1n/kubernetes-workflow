package client

import (
	"k8s.io/kubernetes/pkg/api"
	api_unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
)

// ThirdPartyClient can be used to access third party resources
type ThirdPartyClient struct {
	*restclient.RESTClient
}

func (c *ThirdPartyClient) Workflows(namespace string) WorkflowInterface {
	return newWorkflows(c, namespace)
}

// NewThirdparty creates a new ThirdPartyClient
func NewThirdparty(gv *api_unversioned.GroupVersion, c *restclient.Config) (*ThirdPartyClient, error) {
	config := *c
	groupVersion := *gv
	if err := setThirdPartyDefaults(&groupVersion, &config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ThirdPartyClient{client}, nil
}

// Configuration for RESTClient
func setThirdPartyDefaults(groupVersion *api_unversioned.GroupVersion, config *restclient.Config) error {
	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = restclient.DefaultKubernetesUserAgent()
	}

	config.GroupVersion = groupVersion

	config.Codec = api.Codecs.LegacyCodec(*config.GroupVersion)
	config.NegotiatedSerializer = api.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}
	return nil
}

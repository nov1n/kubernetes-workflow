package client

import (
	"k8s.io/kubernetes/pkg/api"
	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
)

// ThirdPartyClient can be used to access third party resources
type ThirdPartyClient struct {
	*restclient.RESTClient
	baseURL string
}

func (c *ThirdPartyClient) Workflows(namespace string) WorkflowInterface {
	return newWorkflows(c, namespace)
}

// NewThirdparty creates a new ThirdPartyClient
func NewThirdparty(gv *k8sApiUnversioned.GroupVersion, c *restclient.Config) (*ThirdPartyClient, error) {
	config := *c
	groupVersion := *gv
	if err := setThirdPartyDefaults(&groupVersion, &config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	baseURL := c.Host + config.APIPath + "/" + config.GroupVersion.Group + "/" + config.GroupVersion.Version
	return &ThirdPartyClient{client, baseURL}, nil
}

// Configuration for RESTClient
func setThirdPartyDefaults(groupVersion *k8sApiUnversioned.GroupVersion, config *restclient.Config) error {
	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = restclient.DefaultKubernetesUserAgent()
	}

	config.GroupVersion = groupVersion

	//config.Codec = thirdpartyresourcedata.NewCodec(client.NewExtensions(config).RESTClient.Codec(), gvk.Kind)
	config.Codec = api.Codecs.LegacyCodec(*config.GroupVersion)
	// config.NegotiatedSerializer = api.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}
	return nil
}

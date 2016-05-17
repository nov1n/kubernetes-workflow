package client

import (
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
)

// ThirdPartyClient can be used to access third party resources
type ThirdPartyClient struct {
	*k8sRestCl.RESTClient
	baseURL string
}

func (c *ThirdPartyClient) Workflows(namespace string) WorkflowInterface {
	return newWorkflows(c, namespace)
}

// NewThirdParty creates a new ThirdPartyClient
func NewThirdParty(gv k8sApiUnv.GroupVersion, c k8sRestCl.Config) (*ThirdPartyClient, error) {
	if err := setThirdPartyDefaults(&gv, &c); err != nil {
		return nil, err
	}
	client, err := k8sRestCl.RESTClientFor(&c)
	if err != nil {
		return nil, err
	}
	baseURL := c.Host + c.APIPath + "/" + c.GroupVersion.Group + "/" + c.GroupVersion.Version
	return &ThirdPartyClient{client, baseURL}, nil
}

// Configuration for RESTClient
func setThirdPartyDefaults(groupVersion *k8sApiUnv.GroupVersion, config *k8sRestCl.Config) error {
	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = k8sRestCl.DefaultKubernetesUserAgent()
	}

	config.GroupVersion = groupVersion

	//config.Codec = thirdpartyresourcedata.NewCodec(client.NewExtensions(config).RESTClient.Codec(), gvk.Kind)
	config.Codec = k8sApi.Codecs.LegacyCodec(*config.GroupVersion)
	config.NegotiatedSerializer = k8sApi.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}
	return nil
}

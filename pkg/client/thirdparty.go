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
	"path"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sRestCl "k8s.io/kubernetes/pkg/client/restclient"
)

const (
	defaultAPIPath = "/apis"
	defaultQPS     = 5
	defaultBurst   = 10
)

// ThirdPartyClient can be used to access third party resources
type ThirdPartyClient struct {
	*k8sRestCl.RESTClient
	baseURL string
}

// Workflows returns a new workflows object given a namespace
func (c *ThirdPartyClient) Workflows(namespace string) WorkflowInterface {
	return newWorkflows(c, namespace)
}

// NewThirdParty returns a new ThirdPartyClient
func NewThirdParty(gv k8sApiUnv.GroupVersion, c k8sRestCl.Config) (*ThirdPartyClient, error) {
	if err := setThirdPartyDefaults(&gv, &c); err != nil {
		return nil, err
	}
	client, err := k8sRestCl.RESTClientFor(&c)
	if err != nil {
		return nil, err
	}
	baseURL := c.Host + path.Join(c.APIPath, c.GroupVersion.Group, c.GroupVersion.Version)
	return &ThirdPartyClient{client, baseURL}, nil
}

// NewThirdPartyOrDie returns a new ThirdPartyClient or panics when it encounters an error.
func NewThirdPartyOrDie(gv k8sApiUnv.GroupVersion, c k8sRestCl.Config) *ThirdPartyClient {
	if err := setThirdPartyDefaults(&gv, &c); err != nil {
		panic(err)
	}
	client, err := k8sRestCl.RESTClientFor(&c)
	if err != nil {
		panic(err)
	}
	baseURL := c.Host + path.Join(c.APIPath, c.GroupVersion.Group, c.GroupVersion.Version)
	return &ThirdPartyClient{client, baseURL}
}

// Configuration for RESTClient
func setThirdPartyDefaults(groupVersion *k8sApiUnv.GroupVersion, config *k8sRestCl.Config) error {
	config.APIPath = defaultAPIPath
	if config.UserAgent == "" {
		config.UserAgent = k8sRestCl.DefaultKubernetesUserAgent()
	}

	config.GroupVersion = groupVersion

	config.Codec = k8sApi.Codecs.LegacyCodec(*config.GroupVersion)
	config.NegotiatedSerializer = k8sApi.Codecs

	if config.QPS == 0 {
		config.QPS = defaultQPS
	}
	if config.Burst == 0 {
		config.Burst = defaultBurst
	}
	return nil
}

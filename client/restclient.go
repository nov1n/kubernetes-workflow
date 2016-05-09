package workflow

import (
	"net"

	k8sRestClient "k8s.io/kubernetes/pkg/client/restclient"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
)

func NewRESTClient(host, port string) (client *k8sClient.Client, err error) {
	config := &k8sRestClient.Config{
		Host: "http://" + net.JoinHostPort(host, port),
	}

	return k8sClient.New(config)
}

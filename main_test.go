package main

import (
	"testing"

	"github.com/nov1n/kubernetes-workflow/pkg/api"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiExt "k8s.io/kubernetes/pkg/apis/extensions"
	k8sFake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
)

func newThirdPartyResourceList(num int) []k8sApiExt.ThirdPartyResource {
	tpr := k8sApiExt.ThirdPartyResource{
		ObjectMeta: k8sApi.ObjectMeta{
			Name: api.Resource + "." + api.Group,
		},
		Description: api.Description,
		Versions:    api.Versions,
	}

	var list []k8sApiExt.ThirdPartyResource
	for i := 0; i < num; i++ {
		list = append(list, tpr)
	}
	return list
}

func TestRegisterThirdPartyResource(t *testing.T) {
	tprs := newThirdPartyResourceList(1)
	clientset := k8sFake.NewSimpleClientset(tprs...)

	err := registerThirdPartyResource(clientset)
	if err != nil {
		t.Errorf("Didn't expect an error but encountered %v", err)
	}
}

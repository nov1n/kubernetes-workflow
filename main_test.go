package main

import (
	"testing"

	"github.com/nov1n/kubernetes-workflow/pkg/api"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiExt "k8s.io/kubernetes/pkg/apis/extensions"
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

// TODO: Implement this when kube fake client supports tpr.
func TestRegisterThirdPartyResource(t *testing.T) {
	// tprs := newThirdPartyResourceList(1)
	// 	tpr := k8sApiExt.ThirdPartyResource{
	// 		ObjectMeta: k8sApi.ObjectMeta{
	// 			Name: api.Resource + "." + api.Group,
	// 		},
	// 		Description: api.Description,
	// 		Versions:    api.Versions,
	// 	}
	// 	clientset := fake.NewSimpleClientset(&tpr)
	//
	// 	err := registerThirdPartyResource(clientset)
	// 	if err != nil {
	// 		t.Errorf("Didn't expect an error but encountered: %v", err)
	// 	}
	// }

	// func TestRegisterThirdPartyResourceTwoExistingSucces(t *testing.T) {
	// 	tprs := newThirdPartyResourceList(2)
	// 	tprs[1].Name = "OtherKind"
	// 	clientset := fake.NewSimpleClientset(&tprs[0], &tprs[1])
	//
	// 	err := registerThirdPartyResource(clientset)
	// 	if err != nil {
	// 		t.Errorf("Didn't expect an error but encountered: %v", err)
	// 	}
	// }
	//
	// func TestRegisterThirdPartyResourceTwoExistingFail(t *testing.T) {
	// 	tprs := newThirdPartyResourceList(2)
	// 	clientset := fake.NewSimpleClientset(&tprs[0], &tprs[1])
	//
	// 	err := registerThirdPartyResource(clientset)
	// 	if err == nil {
	// 		t.Errorf("Expected error because two instances of the same third party resource already exist.")
	// 	}
}

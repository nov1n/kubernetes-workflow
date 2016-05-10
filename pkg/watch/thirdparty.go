package watch

import (
	k8sWatch "k8s.io/kubernetes/pkg/watch"
)

type ThirdPartyWatcher struct {
	Result  chan k8sWatch.Event
	Stopped bool
}

func NewThirdPartyWatcher() *ThirdPartyWatcher {
	return &ThirdPartyWatcher{
		Result: make(chan k8sWatch.Event),
	}
}

func (tpw *ThirdPartyWatcher) ResultChan() <-chan k8sWatch.Event {
	return tpw.Result
}

func (tpw *ThirdPartyWatcher) Stop() {
	tpw.Stopped = true
}

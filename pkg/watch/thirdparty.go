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

package watch

import (
	k8sWatch "k8s.io/kubernetes/pkg/watch"
)

// ThirdPartyWatcher can watch thirdparty resources for changes and
// generate events on the Result channel
type ThirdPartyWatcher struct {
	Result  chan k8sWatch.Event
	Stopped bool
}

// NewThirdPartyWatcher creates a ThirdPartyWatcher
func NewThirdPartyWatcher() *ThirdPartyWatcher {
	return &ThirdPartyWatcher{
		Result: make(chan k8sWatch.Event),
	}
}

// ResultChan returns the channel on which events are sent
func (tpw *ThirdPartyWatcher) ResultChan() <-chan k8sWatch.Event {
	return tpw.Result
}

// Stop stops the ThirdPartyWatcher from watching and generating events
func (tpw *ThirdPartyWatcher) Stop() {
	tpw.Stopped = true
}

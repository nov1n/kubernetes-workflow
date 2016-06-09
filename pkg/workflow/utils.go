package workflow

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/util"
)

const (
	// ExpectationsTimeout value was copied from "k8s.io/kubernetes/controller/controller_utils.go".
	ExpectationsTimeout = 5 * time.Minute
)

// StepsToTimestamp is a set of steps holding a timestamp.
// The timestamp is the time when the expectation for the step was created.
// This timestamp is used to keep track of a possible expectation timeout.
type StepsToTimestamp map[string]time.Time

// WorkflowStepExpectations is a way to keep track of expectations for steps of
// workflows. See "k8s.io/kubernetes/controller/controller_utils.go" for more
// information on expectations.
type WorkflowStepExpectations struct {
	// storeLock is used to make use of safe concurrent calls.
	storeLock sync.Mutex
	// store maps a workflow key to a set of steps.
	store map[string]StepsToTimestamp
}

// ExpectCreation creates a creation expectation for a given workflow and step.
func (w *WorkflowStepExpectations) ExpectCreation(wfKey, step string) {
	w.storeLock.Lock()
	defer w.storeLock.Unlock()

	// Create step set if it doesn't exist yet.
	if _, exists := w.store[wfKey]; !exists {
		w.store[wfKey] = StepsToTimestamp{}
	}
	w.store[wfKey][step] = time.Now()
}

// isExpired returns true if a step expectation has expired.
func isExpired(timestamp time.Time) bool {
	return util.RealClock{}.Since(timestamp) > ExpectationsTimeout
}

// ExpectationsForStepSatisfied returns whether creation expectations for a
// step for a workflow are satisfied.
func (w *WorkflowStepExpectations) ExpectationsForStepSatisfied(wfKey, step string) bool {
	w.storeLock.Lock()
	defer w.storeLock.Unlock()

	// When no step set exists for the given workflow at all, expectations are satisfied.
	if _, exists := w.store[wfKey]; !exists {
		return true
	}

	if stepTimestamp, exists := w.store[wfKey][step]; exists {
		// When an expectation for the given step exists, but it's expired, the
		// expectations are satisfied.
		if isExpired(stepTimestamp) {
			return true
		}
		// Creation expectation for given step exists.
		return false
	}
	// Creation expectation for given step doesn't exist.
	return true
}

// CreationObserved removes the expectation for the given step.
// CreationObserved should be called when a creation was observed by the informer.
func (w *WorkflowStepExpectations) CreationObserved(wfKey, step string) error {
	w.storeLock.Lock()
	defer w.storeLock.Unlock()

	if _, exists := w.store[wfKey]; !exists {
		return fmt.Errorf("No deletion expectations found for %v", wfKey)
	}

	delete(w.store[wfKey], step)
	return nil
}

// DeleteExpectations deletes all expectations for a given workflow.
func (w *WorkflowStepExpectations) DeleteExpectations(wfKey string) {
	w.storeLock.Lock()
	defer w.storeLock.Unlock()
	delete(w.store, wfKey)
}

// NewWorkflowStepExpectations creates a new WorkflowStepExpectations
func NewWorkflowStepExpectations() *WorkflowStepExpectations {
	return &WorkflowStepExpectations{store: make(map[string]StepsToTimestamp)}
}

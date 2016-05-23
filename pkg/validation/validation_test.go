package validation

import (
	"strings"
	"testing"
	"time"

	"github.com/nov1n/kubernetes-workflow/pkg/api"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sTypes "k8s.io/kubernetes/pkg/types"
)

func TestValidateWorkflowSpec(t *testing.T) {
	successCases := map[string]api.Workflow{
		"one step, no dependencies": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
				}, Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"two steps, no dependencies": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
					"two": {},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"two steps, one with dependencies": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name: "mydag", Namespace: k8sApi.NamespaceDefault,
				UID: k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
					"two": {
						Dependencies: []string{"one"},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"two steps, two with dependencies": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
					"two": {
						Dependencies: []string{"one"},
					},
					"three": {},
					"four": {
						Dependencies: []string{"three"},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"three steps, two with dependencies": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
					"two": {
						Dependencies: []string{"one"},
					},
					"three": {
						Dependencies: []string{"one", "two"},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"realistic with two steps, one with dependencies": {
			TypeMeta: k8sApiUnv.TypeMeta{
				Kind:       "Workflow",
				APIVersion: "nerdalize.com/v1alpha1",
			},
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "test-workflow",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				ActiveDeadlineSeconds: func(i int64) *int64 { return &i }(3600),
				Steps: map[string]api.WorkflowStep{
					"step-a": api.WorkflowStep{
						JobTemplate: &k8sBatch.JobTemplateSpec{
							ObjectMeta: k8sApi.ObjectMeta{
								Name: "job1",
							},
							Spec: k8sBatch.JobSpec{
								Parallelism: func(i int32) *int32 { return &i }(1),
								Template: k8sApi.PodTemplateSpec{
									ObjectMeta: k8sApi.ObjectMeta{
										Name: "pod1",
									},
									Spec: k8sApi.PodSpec{
										RestartPolicy: "OnFailure",
										Containers: []k8sApi.Container{
											k8sApi.Container{
												Name:  "ubuntu1",
												Image: "ubuntu",
												Command: []string{
													"/bin/sleep", "30",
												},
											},
										},
									},
								},
							},
						},
					},
					"step-b": api.WorkflowStep{
						Dependencies: []string{
							"step-a",
						},
						JobTemplate: &k8sBatch.JobTemplateSpec{
							ObjectMeta: k8sApi.ObjectMeta{
								Name: "job2",
							},
							Spec: k8sBatch.JobSpec{
								Parallelism: func(i int32) *int32 { return &i }(1),
								Template: k8sApi.PodTemplateSpec{
									ObjectMeta: k8sApi.ObjectMeta{
										Name: "pod2",
									},
									Spec: k8sApi.PodSpec{
										RestartPolicy: "OnFailure",
										Containers: []k8sApi.Container{
											k8sApi.Container{
												Name:  "ubuntu2",
												Image: "ubuntu",
												Command: []string{
													"/bin/sleep", "30",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
	}
	for k, v := range successCases {
		errs := ValidateWorkflow(&v)
		if len(errs) != 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}
	negative64 := int64(-42)
	errorCases := map[string]api.Workflow{
		"spec.steps: Forbidden: detected cycle [one]": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {
						Dependencies: []string{"one"},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"spec.steps: Forbidden: detected cycle [two one]": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {
						Dependencies: []string{"two"},
					},
					"two": {
						Dependencies: []string{"one"},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"spec.steps: Forbidden: detected cycle [three four five]": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
					"two": {
						Dependencies: []string{"one"},
					},
					"three": {
						Dependencies: []string{"two", "five"},
					},
					"four": {
						Dependencies: []string{"three"},
					},
					"five": {
						Dependencies: []string{"four"},
					},
					"six": {
						Dependencies: []string{"five"},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"spec.steps: Not found: \"three\"": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
					"two": {
						Dependencies: []string{"three"},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"spec.activeDeadlineSeconds: Invalid value: -42: must be greater than or equal to 0": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
				},
				ActiveDeadlineSeconds: &negative64,
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
		"spec.selector: Required value": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {},
				},
			},
		},
		"spec.steps: Invalid value: \"one\": jobTemplate and externalRef are mutually exclusive": {
			ObjectMeta: k8sApi.ObjectMeta{
				Name:      "mydag",
				Namespace: k8sApi.NamespaceDefault,
				UID:       k8sTypes.UID("1uid1cafe"),
			},
			Spec: api.WorkflowSpec{
				Steps: map[string]api.WorkflowStep{
					"one": {
						JobTemplate: &k8sBatch.JobTemplateSpec{},
						ExternalRef: &k8sApi.ObjectReference{},
					},
				},
				Selector: &k8sApiUnv.LabelSelector{
					MatchLabels: map[string]string{"a": "b"},
				},
			},
		},
	}

	for k, v := range errorCases {
		errs := ValidateWorkflow(&v)
		if len(errs) == 0 {
			t.Errorf("expected failure for %s", k)
		} else {
			s := strings.SplitN(k, ":", 2)
			err := errs[0]
			if err.Field != s[0] || !strings.Contains(err.Error(), s[1]) {
				t.Errorf("%v, %v", err.Field, s[0])
				t.Errorf("unexpected error: %v, expected: %s", err.Error(), s[1])
				//t.Errorf("unexpected error: %v, expected: %s", err, k)
			}
		}
	}
}

func NewWorkflow() api.Workflow {
	return api.Workflow{
		ObjectMeta: k8sApi.ObjectMeta{
			Name:            "mydag",
			Namespace:       k8sApi.NamespaceDefault,
			UID:             k8sTypes.UID("1uid1cafe"),
			ResourceVersion: "42",
		},
		Spec: api.WorkflowSpec{
			Steps: map[string]api.WorkflowStep{
				"one": {
					JobTemplate: &k8sBatch.JobTemplateSpec{},
				},
				"two": {
					JobTemplate: &k8sBatch.JobTemplateSpec{},
				},
			},
			Selector: &k8sApiUnv.LabelSelector{
				MatchLabels: map[string]string{"a": "b"},
			},
		},
		Status: api.WorkflowStatus{
			StartTime: &k8sApiUnv.Time{Time: time.Date(2009, time.January, 1, 27, 6, 25, 0, time.UTC)},
			Statuses: map[string]api.WorkflowStepStatus{
				"one": {
					Complete: false,
				},
				"two": {
					Complete: false,
				},
			},
		},
	}
}

func TestValidateWorkflowUpdate(t *testing.T) {

	type WorkflowPair struct {
		current      api.Workflow
		patchCurrent func(*api.Workflow)
		update       api.Workflow
		patchUpdate  func(*api.Workflow)
	}
	errorCases := map[string]WorkflowPair{
		"status.statuses: Invalid value: null: statuses map may not be nil": {
			current: NewWorkflow(),
			update:  NewWorkflow(),
			patchUpdate: func(w *api.Workflow) {
				w.Status.Statuses = nil
			},
		},
		"metadata.resourceVersion: Invalid value: \"\": must be specified for an update": {
			current: NewWorkflow(),
			update:  NewWorkflow(),
			patchUpdate: func(w *api.Workflow) {
				w.ObjectMeta.ResourceVersion = ""
			},
		},
		"workflow: Forbidden: cannot update completed workflow": {
			current: NewWorkflow(),
			patchCurrent: func(w *api.Workflow) {
				s1 := w.Status.Statuses["one"]
				s1.Complete = true
				w.Status.Statuses["one"] = s1
				s2 := w.Status.Statuses["two"]
				s2.Complete = true
				w.Status.Statuses["two"] = s2
			},
			update: NewWorkflow(),
		},
		"spec.steps: Forbidden: cannot delete running step \"one\"": {
			current: NewWorkflow(),
			patchCurrent: func(w *api.Workflow) {
			},
			update: NewWorkflow(),
			patchUpdate: func(w *api.Workflow) {
				delete(w.Spec.Steps, "one") // trying to remove a running step
			},
		},
		"spec.steps: Forbidden: cannot modify running step \"one\"": {
			current: NewWorkflow(),
			patchCurrent: func(w *api.Workflow) {
			},
			update: NewWorkflow(),
			patchUpdate: func(w *api.Workflow) {
				// modify "one"
				s := w.Spec.Steps["one"]
				s.JobTemplate = nil
				s.ExternalRef = &k8sApi.ObjectReference{}
				w.Spec.Steps["one"] = s
			},
		},
		"spec.steps: Forbidden: cannot delete completed step \"one\"": {
			current: NewWorkflow(),
			patchCurrent: func(w *api.Workflow) {
				s := w.Status.Statuses["one"]
				s.Complete = true
				w.Status.Statuses["one"] = s
			},
			update: NewWorkflow(),
			patchUpdate: func(w *api.Workflow) {
				delete(w.Spec.Steps, "one") // removing a complete step
			},
		},
		"spec.steps: Forbidden: cannot modify completed step \"one\"": {
			current: NewWorkflow(),
			patchCurrent: func(w *api.Workflow) {
				s := w.Status.Statuses["one"]
				s.Complete = true
				w.Status.Statuses["one"] = s
			},
			update: NewWorkflow(),
			patchUpdate: func(w *api.Workflow) {
				// modify "one"
				s := w.Spec.Steps["one"]
				s.JobTemplate = nil
				s.ExternalRef = &k8sApi.ObjectReference{}
				w.Spec.Steps["one"] = s
			},
		},
	}
	for k, v := range errorCases {
		if v.patchUpdate != nil {
			v.patchUpdate(&v.update)
		}
		if v.patchCurrent != nil {
			v.patchCurrent(&v.current)
		}
		errs := ValidateWorkflowUpdate(&v.update, &v.current)
		if len(errs) == 0 {
			t.Errorf("expected failure for %s", k)
			continue
		}
		if errs.ToAggregate().Error() != k {
			t.Errorf("unexpected error: %v, expected: %s", errs, k)
		}
	}
}

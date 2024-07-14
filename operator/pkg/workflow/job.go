/*
Copyright 2023 The Karmada Authors.

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

package workflow

import (
	"k8s.io/klog/v2"
)

// RunData is an interface represents all of runDatas abstract object.
type RunData = interface{}

// Job represents an executable workflow, it has list of tasks.
// these tasks must be execution order. if one of these tasks throws
// error, the entire job will fail. During the workflow,if there are
// some artifacts, we can store it to runData.
type Job struct {
	Tasks []Task

	runData RunData

	runDataInitializer func() (RunData, error)
}

// NewJob return a Job with task array.
func NewJob() *Job {
	return &Job{
		Tasks: []Task{},
	}
}

// AppendTask append a task to job, a job has a list of task.
func (j *Job) AppendTask(t Task) {
	j.Tasks = append(j.Tasks, t)
}

func (j *Job) initData() (RunData, error) {
	if j.runData == nil && j.runDataInitializer != nil {
		var err error
		if j.runData, err = j.runDataInitializer(); err != nil {
			klog.ErrorS(err, "failed to initialize running data")
			return nil, err
		}
	}

	return j.runData, nil
}

// SetDataInitializer set a initialize runData function to job.
func (j *Job) SetDataInitializer(build func() (RunData, error)) {
	j.runDataInitializer = build
}

// Run start execute job workflow. if the task has sub-task, it will
// recursive call the sub-tasks util all task be completed or error be thrown.
func (j *Job) Run() error {
	runData := j.runData
	if runData == nil {
		if _, err := j.initData(); err != nil {
			return err
		}
	}

	for _, t := range j.Tasks {
		if err := run(t, j.runData); err != nil {
			return err
		}
	}

	return nil
}

func run(t Task, data RunData) error {
	if t.Skip != nil {
		skip, err := t.Skip(data)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
	}

	if t.Run != nil {
		if err := t.Run(data); err != nil {
			return err
		}
		if t.RunSubTasks {
			for _, p := range t.Tasks {
				if err := run(p, data); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

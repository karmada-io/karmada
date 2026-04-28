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

// Task is minimum unit workflow. It is sample tree structure.
// we can set a list of sub-tasks, they will all be executed if
// RunSubTasks is true.
type Task struct {
	Name        string
	Run         func(RunData) error
	Skip        func(RunData) (bool, error)
	Tasks       []Task
	RunSubTasks bool
}

// AppendTask append a sub task.
func (t *Task) AppendTask(task Task) {
	t.Tasks = append(t.Tasks, task)
}

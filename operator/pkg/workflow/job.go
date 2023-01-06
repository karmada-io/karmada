package workflow

import (
	"k8s.io/klog/v2"
)

// RunData is a interface represents all of runDatas abstract object.
type RunData = interface{}

// Job represents a executable workflow, it has list of tasks.
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

// Run start execte job workflow. if the task has sub task, it will
// recursive call the sub tasks util all of task be completed or error be thrown.
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

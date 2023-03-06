package workflow

// Task is minimum unit workflow. It is sample tree structrue.
// we can set a list of sub tasks, they will all be executed if
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

package auth

import "errors"

var (
	ErrAccountInUse      = errors.New("account is used by running task, stop the task first")
	ErrRunningTasksExist = errors.New("running tasks exist, stop all tasks first")
)

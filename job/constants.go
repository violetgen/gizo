package job

import (
	"errors"
	"time"
)

var (
	ErrExecNotFound           = errors.New("Exec Not Found")
	ErrInvalidPriority        = errors.New("Invalid priority number")
	ErrRetriesOutsideLimit    = errors.New("Retries outside limit")
	ErrRetryDelayOutsideLimit = errors.New("Retry Delay outside limit")
	ErrExecutionTimeBehind    = errors.New("Execution time is past")
)

const (
	MaxRetries      = 5
	MaxRetryBackoff   = 120 //! 2 minutes
	DefaultMaxTTL   = time.Minute * 10
	DefaultRetries  = 0
	DefaultPriority = NORMAL
)

//! priorities
const (
	HIGH   = 3
	MEDIUM = 2
	LOW    = 1
	NORMAL = 0 //! default
)

//TODO: add errors
//! statuses
const (
	QUEUED      = "QUEUED"     // job added to job queue
	TIMEOUT     = "TIMEOUT"    // job timed out
	RUNNING     = "RUNNING"    //job executed
	FINISHED    = "FINISHED"   //job done
	RETRYING    = "RETRYING"   //job retrying
	DISPATHCHED = "DISPATCHED" //job dispatched to worker
	STARTED     = "STARTED"    //job received by dispatcher (prior to dispatch)
)

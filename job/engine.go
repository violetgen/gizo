package job

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"time"

	"github.com/gizo-network/gizo/helpers"

	"github.com/google/uuid"
	"github.com/kpango/glg"
	"github.com/mattn/anko/vm"
)

type Job struct {
	ID        string    `json:"id"`
	Hash      []byte    `json:"hash"`
	Execs     []JobExec `json:"execs"`
	Name      string    `json:"name"`
	Task      string    `json:"task"`
	Signature []byte    `json:"signature"` // signature of deployer
}

func (j Job) IsEmpty() bool {
	return j.GetID() == "" && reflect.ValueOf(j.GetHash()).IsNil() && reflect.ValueOf(j.GetExecs()).IsNil() && j.GetTask() == "" && reflect.ValueOf(j.GetSignature()).IsNil() && j.GetName() == ""
}

func NewJob(task string, name string) *Job {
	j := &Job{
		ID:    uuid.New().String(),
		Execs: []JobExec{},
		Name:  name,
		Task:  helpers.Encode64([]byte(task)),
	}
	j.setHash()
	return j
}

func (j Job) GetName() string {
	return j.Name
}

func (j *Job) setName(n string) {
	j.Name = n
}

func (j Job) GetID() string {
	return j.ID
}

func (j Job) GetHash() []byte {
	return j.Hash
}

func (j *Job) setHash() {
	headers := bytes.Join(
		[][]byte{
			[]byte(j.GetID()),
			j.serializeExecs(),
			[]byte(j.GetTask()),
		},
		[]byte{},
	)
	hash := sha256.Sum256(headers)
	j.Hash = hash[:]
}

func (j Job) serializeExecs() []byte {
	temp, err := json.Marshal(j.GetExecs())
	if err != nil {
		glg.Error(err)
	}
	return temp
}

func (j Job) GetExec(hash []byte) (*JobExec, error) {
	var check int
	for _, exec := range j.GetExecs() {
		check = bytes.Compare(exec.GetHash(), hash)
		if check == 0 {
			return &exec, nil
		}
	}
	return nil, ErrExecNotFound
}

func (j Job) GetLatestExec() JobExec {
	return j.Execs[len(j.GetExecs())-1]
}

func (j Job) GetExecs() []JobExec {
	return j.Execs
}

func (j Job) GetSignature() []byte {
	return j.Signature
}

func (j *Job) SetSignature(sign []byte) {
	j.Signature = sign
}

func (j *Job) AddExec(je JobExec) {
	j.Execs = append(j.Execs, je)
	j.setHash() //regenerates hash
}

func (j Job) GetTask() string {
	return j.Task
}

func (j *Job) Serialize() []byte {
	temp, err := json.Marshal(*j)
	if err != nil {
		glg.Fatal(err)
	}
	return temp
}

func argsStringified(args []interface{}) string {
	temp := "("
	for i, val := range args {
		if i == len(args)-1 {
			temp += val.(string) + ""
		} else {
			temp += val.(string) + ","
		}
	}
	return temp + ")"
}

//!TODO: handle retry and retrydelay limit
func (j *Job) Execute(exec *JobExec) {
	exec.SetStatus(RUNNING)
	r := exec.GetRetries()
retry:
	env := vm.NewEnv()
	start := time.Now()
	var result interface{}
	var err error
	if len(exec.GetArgs()) == 0 {
		result, err = env.Execute(string(helpers.Decode64(j.GetTask())) + "\n" + j.GetName() + "()")
	} else {
		result, err = env.Execute(string(helpers.Decode64(j.GetTask())) + "\n" + j.GetName() + argsStringified(exec.GetArgs()))
	}

	if r != 0 && err != nil {
		r--
		time.Sleep(exec.GetRetryDelay() * time.Second)
		exec.SetStatus(RETRYING)
		glg.Info("Retrying job - " + j.GetID())
		goto retry
	}
	exec.SetTimestamp(time.Now().Unix())
	exec.SetDuration(time.Duration(time.Now().Sub(start).Nanoseconds()))
	exec.SetErr(err)
	exec.SetResult(result)
	exec.setHash()
	exec.SetStatus(FINISHED)
	j.AddExec(*exec)
	// return &exec
}

package job

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/gizo-network/gizo/helpers"

	"github.com/google/uuid"
	"github.com/kpango/glg"
	"github.com/mattn/anko/vm"
)

var (
	ErrExecNotFound = errors.New("Exec Not Found")
)

type Job struct {
	ID        string    `json:"id"`
	Hash      []byte    `json:"hash"`
	Execs     []JobExec `json:"execs"`
	Source    string    `json:"source"`
	Signature []byte    `json:"signature"` // signature of deployer
}

func (j Job) IsEmpty() bool {
	return j.GetID() == "" && reflect.ValueOf(j.GetHash()).IsNil() && reflect.ValueOf(j.GetExecs()).IsNil() && j.GetSource() == "" && reflect.ValueOf(j.GetSignature()).IsNil()
}

func NewJob(s string) *Job {
	j := &Job{
		ID:     uuid.New().String(),
		Execs:  []JobExec{},
		Source: helpers.Encode64([]byte(s)),
	}
	j.setHash()
	return j
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
			[]byte(j.GetSource()),
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

func (j Job) GetSource() string {
	return j.Source
}

func (j *Job) Serialize() []byte {
	temp, err := json.Marshal(*j)
	if err != nil {
		glg.Fatal(err)
	}
	return temp
}

//FIXME: add fault tolerance and security
func (j *Job) Execute() (interface{}, error) {
	env := vm.NewEnv()
	start := time.Now()
	result, err := env.Execute(string(helpers.Decode64(j.GetSource())))
	exec := JobExec{
		Timestamp: time.Now().Unix(),
		Duration:  time.Now().Sub(start).Nanoseconds(),
		Err:       err,
		Result:    result,
		By:        []byte("0000"), //FIXME: replace with real ID
	}
	exec.setHash()
	j.AddExec(exec)
	return result, err
}

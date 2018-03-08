package job

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"reflect"
	"strconv"
	"time"

	"github.com/satori/go.uuid"

	"github.com/gizo-network/gizo/helpers"

	"github.com/kpango/glg"
	anko_core "github.com/mattn/anko/builtins"
	anko_vm "github.com/mattn/anko/vm"
)

var (
	ErrUnverifiedSignature = errors.New("signature not verified")
)

type Job struct {
	ID             string    `json:"id"`
	Hash           []byte    `json:"hash"`
	Execs          []Exec    `json:"execs"`
	Name           string    `json:"name"`
	Task           string    `json:"task"`
	Signature      [][]byte  `json:"signature"` // signature of owner
	SubmissionTime time.Time `json:"submission_time"`
	Private        bool      `json:"private"` //private job flag (default to false - public)
}

func (j *Job) Sign(priv []byte) {
	hash := sha256.Sum256([]byte(j.GetTask()))
	privateKey, _ := x509.ParseECPrivateKey(priv)
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		glg.Fatal("Job: unable to sign job")
	}
	var temp [][]byte
	temp = append(temp, r.Bytes(), s.Bytes())
	j.setSignature(temp)
}

func (j Job) VerifySignature(pub string) bool {
	pubBytes, err := hex.DecodeString(pub)
	if err != nil {
		glg.Fatal(err)
	}

	var r big.Int
	var s big.Int
	r.SetBytes(j.GetSignature()[0])
	s.SetBytes(j.GetSignature()[1])

	publicKey, _ := x509.ParsePKIXPublicKey(pubBytes)
	hash := sha256.Sum256([]byte(j.GetTask()))
	switch pubConv := publicKey.(type) {
	case *ecdsa.PublicKey:
		return ecdsa.Verify(pubConv, hash[:], &r, &s)
	default:
		return false
	}
}

func (j Job) GetSubmissionTime() time.Time {
	return j.SubmissionTime
}

func (j *Job) setSubmissionTime(t time.Time) {
	j.SubmissionTime = t
}

func (j Job) IsEmpty() bool {
	return j.GetID() == "" && reflect.ValueOf(j.GetHash()).IsNil() && reflect.ValueOf(j.GetExecs()).IsNil() && j.GetTask() == "" && reflect.ValueOf(j.GetSignature()).IsNil() && j.GetName() == ""
}

func NewJob(task string, name string, priv bool, privKey string) *Job {
	j := &Job{
		SubmissionTime: time.Now(),
		ID:             uuid.NewV4().String(),
		Execs:          []Exec{},
		Name:           name,
		Task:           helpers.Encode64([]byte(task)),
		Private:        priv,
	}
	privBytes, err := hex.DecodeString(privKey)
	if err != nil {
		log.Fatal(err)
	}
	j.Sign(privBytes)
	j.setHash()
	return j
}

func (j Job) GetPrivate() bool {
	return j.Private
}

func (j *Job) setPrivate(p bool) {
	j.Private = p
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
			[]byte(j.GetTask()),
			[]byte(j.GetName()),
			j.GetSignature()[0],
			j.GetSignature()[1],
			[]byte(string(j.GetSubmissionTime().Unix())),
			[]byte(strconv.FormatBool(j.GetPrivate())),
			// j.GetOwner(),
		},
		[]byte{},
	)
	hash := sha256.Sum256(headers)
	j.Hash = hash[:]
}

func (j Job) Verify() bool {
	headers := bytes.Join(
		[][]byte{
			[]byte(j.GetID()),
			[]byte(j.GetTask()),
			[]byte(j.GetName()),
			j.GetSignature()[0],
			j.GetSignature()[1],
			[]byte(string(j.GetSubmissionTime().Unix())),
			[]byte(strconv.FormatBool(j.GetPrivate())),
			// j.GetOwner(),
		},
		[]byte{},
	)
	hash := sha256.Sum256(headers)
	return bytes.Compare(j.GetHash(), hash[:]) == 0
}

func (j Job) serializeExecs() []byte {
	temp, err := json.Marshal(j.GetExecs())
	if err != nil {
		glg.Error(err)
	}
	return temp
}

func (j Job) GetExec(hash []byte) (*Exec, error) {
	glg.Info("Job: Getting exec - " + hex.EncodeToString(hash))
	var check int
	for _, exec := range j.GetExecs() {
		check = bytes.Compare(exec.GetHash(), hash)
		if check == 0 {
			return &exec, nil
		}
	}
	return nil, ErrExecNotFound
}

func (j Job) GetLatestExec() Exec {
	return j.Execs[len(j.GetExecs())-1]
}

func (j Job) GetExecs() []Exec {
	return j.Execs
}

func (j Job) GetSignature() [][]byte {
	return j.Signature
}

func (j *Job) setSignature(sign [][]byte) {
	j.Signature = sign
}

func (j *Job) AddExec(je Exec) {
	glg.Info("Job: Adding exec - " + hex.EncodeToString(je.GetHash()) + " to job - " + j.GetID())
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

func DeserializeJob(b []byte) (*Job, error) {
	var temp Job
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return nil, err
	}
	return &temp, nil
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

//! run in goroutine
func (j *Job) Execute(exec *Exec) *Exec {
	if j.GetPrivate() == true {
		if j.VerifySignature(exec.getPub()) == false {
			exec.SetErr(ErrUnverifiedSignature)
			return exec
		}
	}
	glg.Info("Job: Executing job - " + j.GetID())
	start := time.Now()
	done := make(chan struct{})
	exec.SetStatus(RUNNING)
	exec.SetTimestamp(time.Now().Unix())
	go func() {
		var ttl time.Duration
		if exec.GetTTL() != 0 {
			ttl = exec.GetTTL()
		} else {
			ttl = DefaultMaxTTL
		}
		select {
		case <-time.NewTimer(ttl).C:
			exec.SetStatus(TIMEOUT)
			glg.Warn("Job: Job timeout - " + j.GetID())
			done <- struct{}{}
		}
	}()
	go func() {
		r := exec.GetRetries()
	retry:
		env := anko_vm.NewEnv()
		anko_core.LoadAllBuiltins(env) //!FIXME: limiit packages that are loaded in
		//?FIXME: check if mattn replies so as to make a map (env) to store all environment variables like process.env in node.js
		for _, val := range exec.GetEnvs() {
			env.Define(val.GetKey(), val.GetValue())
		}
		var result interface{}
		var err error
		if len(exec.GetArgs()) == 0 {
			result, err = env.Execute(string(helpers.Decode64(j.GetTask())) + "\n" + j.GetName() + "()")
		} else {
			result, err = env.Execute(string(helpers.Decode64(j.GetTask())) + "\n" + j.GetName() + argsStringified(exec.GetArgs()))
		}

		if r != 0 && err != nil {
			r--
			time.Sleep(exec.GetBackoff())
			exec.SetStatus(RETRYING)
			exec.IncrRetriesCount()
			glg.Warn("Job: Retrying job - " + j.GetID())
			goto retry
		}
		exec.SetDuration(time.Duration(time.Now().Sub(start).Nanoseconds()))
		exec.SetErr(err)
		exec.SetResult(result)
		exec.setHash()
		exec.SetStatus(FINISHED)
		done <- struct{}{}
	}()
	select {
	case <-done:
		return exec
	}
}

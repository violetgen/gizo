package job

import (
	"encoding/json"

	"github.com/kpango/glg"
)

type JobRequestMultiple struct {
	ID    string
	Execs []*Exec
}

func NewJobRequestMultiple(id string, exec ...*Exec) *JobRequestMultiple {
	return &JobRequestMultiple{
		ID:    id,
		Execs: exec,
	}
}

func (jr *JobRequestMultiple) AppendExec(exec *Exec) {
	jr.Execs = append(jr.Execs, exec)
}

func (jr *JobRequestMultiple) SetID(id string) {
	jr.ID = id
}

func (jr JobRequestMultiple) GetID() string {
	return jr.ID
}

func (jr JobRequestMultiple) GetExec() []*Exec {
	return jr.Execs
}

func DeserializeJRM(b []byte) JobRequestMultiple {
	var temp JobRequestMultiple
	err := json.Unmarshal(b, &temp)
	if err != nil {
		glg.Fatal(err)
	}
	return temp
}

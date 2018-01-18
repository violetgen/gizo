package core

import (
	"encoding/json"

	"github.com/kpango/glg"
)

type BlockInfo struct {
	Header    BlockHeader `json:"header"`
	Height    uint64      `json:"height"`
	TotalJobs uint        `json:"total_jobs"`
	FileName  string      `json:"file_name"`
	FileSize  int64       `json:"file_size"`
}

func (bi *BlockInfo) SetHeader(bh BlockHeader) {
	bi.Header = bh
}

func (bi BlockInfo) GetHeader() BlockHeader {
	return bi.Header
}

func (bi *BlockInfo) SetHeight(h uint64) {
	bi.Height = h
}

func (bi BlockInfo) GetHeight() uint64 {
	return bi.Height
}

func (bi *BlockInfo) SetTotalJobs(t uint) {
	bi.TotalJobs = t
}

func (bi BlockInfo) GetTotalJobs() uint {
	return bi.TotalJobs
}

func (bi *BlockInfo) SetFileName(n string) {
	bi.FileName = n
}

func (bi BlockInfo) GetFileName() string {
	return bi.FileName
}

func (bi *BlockInfo) SetFileSize(s int64) {
	bi.FileSize = s
}

func (bi BlockInfo) GetFileSize() int64 {
	return bi.FileSize
}

func (b *BlockInfo) Serialize() []byte {
	temp, err := json.Marshal(*b)
	if err != nil {
		glg.Fatal(err)
	}
	return temp
}

func (b BlockInfo) GetBlock() *Block {
	var temp Block
	temp.Import(b.Header.GetHash())
	return &temp
}

func DeserializeBlockInfo(bi []byte) *BlockInfo {
	var temp BlockInfo
	err := json.Unmarshal(bi, &temp)
	if err != nil {
		glg.Fatal(err)
	}
	return &temp
}

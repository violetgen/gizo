package core

import (
	"bytes"
	"crypto/sha256"
	"math"
	"math/big"
	"strconv"

	"github.com/gizo-network/gizo/core/merkletree"
	"github.com/kpango/glg"
)

var maxNonce = math.MaxInt64

type POW struct {
	difficulty uint8
	block      *Block
	target     *big.Int
}

func (p *POW) SetBlock(b *Block) {
	p.block = b
}

func (p POW) GetBlock() *Block {
	return p.block
}

func (p *POW) SetTarget(t *big.Int) {
	p.target = t
}

func (p POW) GetTarget() *big.Int {
	return p.target
}

func (p POW) GetDifficulty() uint8 {
	return p.difficulty
}

func (p *POW) SetDifficulty(d uint8) {
	p.difficulty = d
}

func (p POW) prepareData(nonce int) []byte {
	tree := merkletree.MerkleTree{Root: p.GetBlock().GetHeader().GetMerkleRoot(), LeafNodes: p.GetBlock().GetJobs()}
	mBytes, err := tree.Serialize()
	if err != nil {
		glg.Fatal(err)
	}
	data := bytes.Join(
		[][]byte{
			p.block.GetHeader().GetPrevBlockHash(),
			[]byte(strconv.FormatInt(p.GetBlock().GetHeader().GetTimestamp(), 10)),
			mBytes,
			[]byte(strconv.FormatInt(int64(nonce), 10)),
			[]byte(strconv.FormatInt(int64(p.GetBlock().GetHeight()), 10)),
			[]byte(strconv.FormatInt(int64(p.GetDifficulty()), 10)),
		},
		[]byte{},
	)
	return data
}

func (p *POW) Run() {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0
	for nonce < maxNonce {
		hash = sha256.Sum256(p.prepareData(nonce))
		hashInt.SetBytes(hash[:])
		if hashInt.Cmp(p.GetTarget()) == -1 {
			break
		} else {
			nonce++
		}
	}
	p.GetBlock().Header.SetHash(hash[:])
	p.GetBlock().Header.SetNonce(uint64(nonce))
}

func NewPOW(b *Block) *POW {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-b.GetHeader().GetDifficulty().Int64()))

	pow := &POW{
		target: target,
		block:  b,
	}
	return pow
}

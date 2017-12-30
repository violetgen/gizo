package main

import (
	"fmt"

	"github.com/gizo-network/gizo/core"
	"github.com/gizo-network/gizo/helpers"
)

func main() {
	block := core.NewBlock([]byte("jobs example"), []byte("genesis block"), []byte("merkle root"))
	block.SetHash()
	fmt.Println(helpers.MarshalBlock(*block))
	fmt.Println(block.VerifyBlock())
	err := block.SetHash()
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(helpers.MarshalBlock(*block))
}

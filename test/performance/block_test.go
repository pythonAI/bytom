package performance

import (
	"os"
	"testing"

	dbm "github.com/tendermint/tmlibs/db"

	"github.com/bytom/config"
	"github.com/bytom/test"
)

// Benchamark function ValidateBlock()
func BenchmarkValidateBlock(b *testing.B) {
	testDB := dbm.NewDB("testdb", "leveldb", "temp")
	defer os.RemoveAll("temp")

	chain, err := test.MockChain(testDB)
	if err != nil {
		b.Fatal(err)
	}

	genesisBlock := config.GenerateGenesisBlock()
	preBlock, _ := chain.GetBlockByHash(&genesisBlock.PreviousBlockHash)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.ValidateBlock(genesisBlock, preBlock)
	}
}

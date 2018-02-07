package integration

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	dbm "github.com/tendermint/tmlibs/db"

	"github.com/bytom/blockchain/account"
	"github.com/bytom/blockchain/asset"
	"github.com/bytom/blockchain/pseudohsm"
	"github.com/bytom/blockchain/txbuilder"
	"github.com/bytom/blockchain/txdb"
	"github.com/bytom/blockchain/wallet"
	cfg "github.com/bytom/config"
	"github.com/bytom/consensus"
	"github.com/bytom/crypto/ed25519/chainkd"
	"github.com/bytom/protocol"
	"github.com/bytom/protocol/bc"
	"github.com/bytom/protocol/bc/legacy"
	"github.com/bytom/protocol/vm/vmutil"
)

func TestWalletUpdate(t *testing.T) {
	dirPath, err := ioutil.TempDir(".", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dirPath)

	testDB := dbm.NewDB("testdb", "leveldb", "temp")
	defer os.RemoveAll("temp")

	store := txdb.NewStore(testDB)
	txPool := protocol.NewTxPool()

	chain, err := protocol.NewChain(bc.Hash{}, store, txPool)
	if err != nil {
		t.Fatal(err)
	}

	genesisBlock := cfg.GenerateGenesisBlock()
	chain.SaveBlock(genesisBlock)
	chain.ConnectBlock(genesisBlock)

	accountManager := account.NewManager(testDB, chain)
	hsm, err := pseudohsm.New(dirPath)
	if err != nil {
		t.Fatal(err)
	}

	xpub1, err := hsm.XCreate("alias", "password")
	if err != nil {
		t.Fatal(err)
	}

	testAccount, err := accountManager.Create(nil, []chainkd.XPub{xpub1.XPub}, 1, "test", nil)
	if err != nil {
		t.Fatal(err)
	}

	controlProg, err := accountManager.CreateAddress(nil, testAccount.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	controlProg.KeyIndex = 1

	utxo := mockUTXO(controlProg)
	_, txData, err := mockTxData(utxo, testAccount)
	if err != nil {
		t.Fatal(err)
	}

	tx := legacy.NewTx(*txData)

	block := mockBlock(t, tx, legacy.MapBlock(genesisBlock))

	reg := asset.NewRegistry(testDB, chain)
	w, err := wallet.NewWallet(testDB, accountManager, reg, chain, nil)
	if err != nil {
		t.Fatal(err)
	}

	// TODO: update when AttachBlock is exposed formally.
	err = w.AttachBlock(block)
	if err != nil {
		t.Fatal(err)
	}

	want, err := w.GetTransactionsByTxID(tx.ID.String())
	if len(want) != 1 {
		t.Fatalf("The number of transactions is unexpected %d\n", len(want))
	}

	if tx.ID != want[0].ID {
		t.Errorf("The ID of transactions mismatch.\nwant: %v\n get: %v\n", tx.ID, want[0].ID)
	}

	wants, err := w.GetTransactionsByAccountID(testAccount.ID)
	if len(wants) != 1 {
		t.Fatalf("The number of transactions is unexpected %d\n", len(wants))
	}
	if tx.ID != wants[0].ID {
		t.Errorf("The ID of transactions mismatch.\nwant: %v\n get: %v\n", tx.ID, want[0].ID)
	}
}

func mockBlock(t *testing.T, tx *legacy.Tx, prev *bc.Block) *legacy.Block {
	b := &legacy.Block{
		BlockHeader: legacy.BlockHeader{
			Version:           1,
			Height:            prev.Height + 1,
			PreviousBlockHash: prev.ID,
			Timestamp:         prev.Timestamp + 1,
			TransactionStatus: bc.TransactionStatus{
				Bitmap: []byte{0},
			},
			BlockCommitment: legacy.BlockCommitment{},
			Bits:            2305843009230471167,
		},
		Transactions: []*legacy.Tx{mockCoinbaseTx(624000000000), tx},
	}

	var err error
	bcBlock := legacy.MapBlock(b)
	b.TransactionsMerkleRoot, err = bc.MerkleRoot(bcBlock.Transactions)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func mockCoinbaseTx(amount uint64) *legacy.Tx {
	cp, _ := vmutil.DefaultCoinbaseProgram()
	txData := legacy.TxData{
		Version: 1,
		Inputs: []*legacy.TxInput{
			legacy.NewCoinbaseInput(nil, nil),
		},
		Outputs: []*legacy.TxOutput{
			legacy.NewTxOutput(*consensus.BTMAssetID, amount, cp, nil),
		},
	}
	return legacy.NewTx(txData)
}

func mockUTXO(controlProg *account.CtrlProgram) *account.UTXO {
	utxo := &account.UTXO{}
	utxo.OutputID = bc.Hash{V0: 1}
	utxo.SourceID = bc.Hash{V0: 2}
	utxo.AssetID = *consensus.BTMAssetID
	utxo.Amount = 1000000000
	utxo.SourcePos = 0
	utxo.ControlProgram = controlProg.ControlProgram
	utxo.AccountID = controlProg.AccountID
	utxo.Address = controlProg.Address
	utxo.ControlProgramIndex = controlProg.KeyIndex
	return utxo
}

func mockTxData(utxo *account.UTXO, testAccount *account.Account) (*txbuilder.Template, *legacy.TxData, error) {
	txInput, sigInst, err := account.UtxoToInputs(testAccount.Signer, utxo, nil)
	if err != nil {
		return nil, nil, err
	}

	b := txbuilder.NewBuilder(time.Now())
	b.AddInput(txInput, sigInst)
	out := legacy.NewTxOutput(*consensus.BTMAssetID, 100, utxo.ControlProgram, nil)
	b.AddOutput(out)
	return b.Build()
}

package transactionpool

import (
	"errors"
	"testing"

	"github.com/NebulousLabs/Sia/consensus"
	"github.com/NebulousLabs/Sia/modules"
	"github.com/NebulousLabs/Sia/modules/gateway"
	"github.com/NebulousLabs/Sia/modules/miner"
	"github.com/NebulousLabs/Sia/modules/tester"
	"github.com/NebulousLabs/Sia/modules/wallet"
)

// A tpoolTester contains a consensus tester and a transaction pool, and
// provides a set of helper functions for testing the transaction pool without
// modules that need to use the transaction pool.
//
// updateChan is a channel that will block until the transaction pool posts an
// update. This is useful for synchronizing with updates from the state.
type tpoolTester struct {
	cs     *consensus.State
	tpool  *TransactionPool
	miner  modules.Miner
	wallet modules.Wallet

	updateChan chan struct{}

	t *testing.T
}

// checkEmpty checks that all of the internal objects to the transaction pool
// have been emptied.
func (tpt *tpoolTester) checkEmpty() error {
	id := tpt.tpool.mu.RLock()
	if len(tpt.tpool.transactions) != 0 {
		return errors.New("tpool.transactions isn't empty")
	}
	if len(tpt.tpool.siacoinOutputs) != 0 {
		return errors.New("tpool.siacoinOutputs isn't empty")
	}
	if len(tpt.tpool.fileContracts) != 0 {
		return errors.New("tpool.fileContracts isn't empty")
	}
	if len(tpt.tpool.siafundOutputs) != 0 {
		return errors.New("tpool.siafundOuptuts isn't empty")
	}
	if len(tpt.tpool.referenceFileContracts) != 0 {
		return errors.New("tpool.referenceFileContracts isn't empty")
	}
	if len(tpt.tpool.usedSiacoinOutputs) != 0 {
		return errors.New("tpool.usedSiacoinOutputs wasn't properly emtied out.")
	}
	if len(tpt.tpool.newFileContracts) != 0 {
		return errors.New("tpool.newFileContracts isn't empty")
	}
	if len(tpt.tpool.fileContractTerminations) != 0 {
		return errors.New("tpool.fileContractTerminations isn't empty")
	}
	if len(tpt.tpool.storageProofsByStart) != 0 {
		return errors.New("tpool.storageProofsByStart isn't empty")
	}
	if len(tpt.tpool.storageProofsByExpiration) != 0 {
		return errors.New("tpool.storageProofsByExipration isn't empty")
	}
	if len(tpt.tpool.usedSiafundOutputs) != 0 {
		return errors.New("tpool.usedSiafundOutputs isn't empty")
	}
	tpt.tpool.mu.RUnlock(id)
	return nil
}

// emptyUnlockTransaction creates a transaction with empty UnlockConditions,
// meaning it's trivial to spend the output.
func (tpt *tpoolTester) emptyUnlockTransaction() consensus.Transaction {
	// Send money to an anyone-can-spend address.
	emptyHash := consensus.UnlockConditions{}.UnlockHash()
	txn, err := tpt.wallet.SpendCoins(consensus.NewCurrency64(1), emptyHash)
	if err != nil {
		tpt.t.Fatal(err)
	}
	outputID := txn.SiacoinOutputID(0)

	// Create a transaction spending the coins.
	txn = consensus.Transaction{
		SiacoinInputs: []consensus.SiacoinInput{
			consensus.SiacoinInput{
				ParentID: outputID,
			},
		},
		SiacoinOutputs: []consensus.SiacoinOutput{
			consensus.SiacoinOutput{
				Value:      consensus.NewCurrency64(1),
				UnlockHash: emptyHash,
			},
		},
	}

	return txn
}

// CreatetpoolTester initializes a tpoolTester.
func newTpoolTester(directory string, t *testing.T) (tpt *tpoolTester) {
	// Create the consensus set.
	cs := consensus.CreateGenesisState()

	// Create the gateway.
	gDir := tester.TempDir(directory, modules.GatewayDir)
	g, err := gateway.New(":0", cs, gDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create the transaction pool.
	tp, err := New(cs, g)
	if err != nil {
		t.Fatal(err)
	}

	// Create the wallet.
	wDir := tester.TempDir(directory, modules.WalletDir)
	w, err := wallet.New(cs, tp, wDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create the miner.
	m, err := miner.New(cs, g, tp, w)
	if err != nil {
		t.Fatal(err)
	}

	// Subscribe to the updates of the transaction pool.
	updateChan := make(chan struct{}, 1)
	id := tp.mu.Lock()
	tp.subscribers = append(tp.subscribers, updateChan)
	tp.mu.Unlock(id)

	// Assebmle all of the objects in to a tpoolTester
	tpt = &tpoolTester{
		cs:         cs,
		tpool:      tp,
		miner:      m,
		wallet:     w,
		updateChan: updateChan,
		t:          t,
	}

	// Mine blocks until there is money in the wallet.
	for i := 0; i <= consensus.MaturityDelay; i++ {
		for {
			var found bool
			_, found, err = tpt.miner.FindBlock()
			if err != nil {
				t.Fatal(err)
			}
			if found {
				<-updateChan
				break
			}
		}
	}

	return
}

// TestNewNilInputs tries to trigger a panic with nil inputs.
func TestNewNilInputs(t *testing.T) {
	// Create a consensus set and gateway.
	cs := consensus.CreateGenesisState()
	gDir := tester.TempDir("Transaction Pool - TestNewNilInputs", modules.GatewayDir)
	g, err := gateway.New(":0", cs, gDir)
	if err != nil {
		t.Fatal(err)
	}
	New(nil, nil)
	New(cs, nil)
	New(nil, g)
}
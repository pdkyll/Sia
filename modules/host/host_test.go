package host

import (
	// "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/NebulousLabs/Sia/build"
	"github.com/NebulousLabs/Sia/crypto"
	"github.com/NebulousLabs/Sia/modules"
	"github.com/NebulousLabs/Sia/modules/consensus"
	"github.com/NebulousLabs/Sia/modules/gateway"
	"github.com/NebulousLabs/Sia/modules/miner"
	// "github.com/NebulousLabs/Sia/modules/renter"
	"github.com/NebulousLabs/Sia/modules/transactionpool"
	"github.com/NebulousLabs/Sia/modules/wallet"
	"github.com/NebulousLabs/Sia/types"
)

// A hostTester is the helper object for host testing, including helper modules
// and methods for controlling synchronization.
type hostTester struct {
	cs      modules.ConsensusSet
	gateway modules.Gateway
	miner   modules.TestMiner
	// renter    modules.Renter
	renting   bool
	tpool     modules.TransactionPool
	wallet    modules.Wallet
	walletKey crypto.TwofishKey

	host *Host

	persistDir string
}

/*
// initRenting prepares the host tester for uploads and downloads by announcing
// the host to the network and performing other preparational tasks.
// initRenting takes a while because the renter needs to process the host
// announcement, requiring asynchronous network communication between the
// renter and host.
func (ht *hostTester) initRenting() error {
	if ht.renting {
		return nil
	}

	// Because the renting test takes a long time, it will fail if
	// testing.Short.
	if testing.Short() {
		return errors.New("cannot call initRenting in short tests")
	}

	// Announce the host.
	err := ht.host.Announce()
	if err != nil {
		return err
	}

	// Mine a block to get the announcement into the blockchain.
	_, err = ht.miner.AddBlock()
	if err != nil {
		return err
	}

	// Wait for the renter to see the host announcement.
	for i := 0; i < 50; i++ {
		time.Sleep(time.Millisecond * 100)
		if len(ht.renter.ActiveHosts()) != 0 {
			break
		}
	}
	if len(ht.renter.ActiveHosts()) == 0 {
		return errors.New("could not start renting in the host tester")
	}
	ht.renting = true
	return nil
}
*/

// initWallet creates a wallet key, initializes the host wallet, unlocks it,
// and then stores the key in the host tester.
func (ht *hostTester) initWallet() error {
	// Create the keys for the wallet and unlock it.
	key, err := crypto.GenerateTwofishKey()
	if err != nil {
		return err
	}
	ht.walletKey = key
	_, err = ht.wallet.Encrypt(key)
	if err != nil {
		return err
	}
	err = ht.wallet.Unlock(key)
	if err != nil {
		return err
	}
	return nil
}

// blankHostTester creates a host tester where the modules are created but no
// extra initialization has been done, for example no blocks have been mined
// and the wallet keys have not been created.
func blankHostTester(name string) (*hostTester, error) {
	testdir := build.TempDir(modules.HostDir, name)

	// Create the modules.
	g, err := gateway.New("localhost:0", filepath.Join(testdir, modules.GatewayDir))
	if err != nil {
		return nil, err
	}
	cs, err := consensus.New(g, filepath.Join(testdir, modules.ConsensusDir))
	if err != nil {
		return nil, err
	}
	tp, err := transactionpool.New(cs, g)
	if err != nil {
		return nil, err
	}
	w, err := wallet.New(cs, tp, filepath.Join(testdir, modules.WalletDir))
	if err != nil {
		return nil, err
	}
	m, err := miner.New(cs, tp, w, filepath.Join(testdir, modules.MinerDir))
	if err != nil {
		return nil, err
	}
	h, err := New(cs, tp, w, "localhost:0", filepath.Join(testdir, modules.HostDir))
	if err != nil {
		return nil, err
	}
	/*
		r, err := renter.New(cs, w, tp, filepath.Join(testdir, modules.RenterDir))
		if err != nil {
			return nil, err
		}
	*/

	// Assemble all objects into a hostTester
	ht := &hostTester{
		cs:      cs,
		gateway: g,
		miner:   m,
		// renter:  r,
		tpool:  tp,
		wallet: w,

		host: h,

		persistDir: testdir,
	}

	return ht, nil
}

// newHostTester creates a host tester with an initialized wallet and money in
// that wallet.
func newHostTester(name string) (*hostTester, error) {
	// Create a blank host tester.
	ht, err := blankHostTester(name)
	if err != nil {
		return nil, err
	}

	// Initialize the wallet and mine blocks until the wallet has money.
	err = ht.initWallet()
	if err != nil {
		return nil, err
	}
	for i := types.BlockHeight(0); i <= types.MaturityDelay; i++ {
		_, err = ht.miner.AddBlock()
		if err != nil {
			return nil, err
		}
	}

	// Create two storage folder for the host, one the minimum size and one
	// twice the minimum size.
	storageFolderOne := filepath.Join(ht.persistDir, "hostTesterStorageFolderOne")
	err = os.Mkdir(storageFolderOne, 0700)
	if err != nil {
		return nil, err
	}
	err = ht.host.AddStorageFolder(storageFolderOne, modules.SectorSize*8)
	if err != nil {
		return nil, err
	}
	storageFolderTwo := filepath.Join(ht.persistDir, "hostTesterStorageFolderTwo")
	err = os.Mkdir(storageFolderTwo, 0700)
	if err != nil {
		return nil, err
	}
	err = ht.host.AddStorageFolder(storageFolderTwo, modules.SectorSize*8*2)
	if err != nil {
		return nil, err
	}
	return ht, nil
}

// TestHostInitialization checks that the host intializes to sensisble default
// values.
func TestHostInitialization(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()
	// Create a blank host tester and check that the height is zero.
	bht, err := blankHostTester("TestHostInitialization")
	if err != nil {
		t.Fatal(err)
	}
	if bht.host.blockHeight != 0 {
		t.Error("host initialized to the wrong block height")
	}

	// Initialize the wallet so that a block can be mined, then mine a block
	// and check that it sets the host height to 1.
	err = bht.initWallet()
	if err != nil {
		t.Fatal(err)
	}
	_, err = bht.miner.AddBlock()
	if err != nil {
		t.Fatal(err)
	}
	if bht.host.blockHeight != 1 {
		t.Fatal("block height did not increase correctly after first block mined:", bht.host.blockHeight, 1)
	}
}

// TestNilValues tries initializing the host with nil values.
func TestNilValues(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()
	ht, err := blankHostTester("TestStartupRescan")
	if err != nil {
		t.Fatal(err)
	}

	hostDir := filepath.Join(ht.persistDir, modules.HostDir)
	_, err = New(nil, ht.tpool, ht.wallet, "localhost:0", hostDir)
	if err != errNilCS {
		t.Fatal("could not trigger errNilCS")
	}
	_, err = New(ht.cs, nil, ht.wallet, "localhost:0", hostDir)
	if err != errNilTpool {
		t.Fatal("could not trigger errNilTpool")
	}
	_, err = New(ht.cs, ht.tpool, nil, "localhost:0", hostDir)
	if err != errNilWallet {
		t.Fatal("Could not trigger errNilWallet")
	}
}

// TestSetAndGetInternalSettings checks that the functions for interacting with
// the host's internal settings object are working as expected.
func TestSetAndGetInternalSettings(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ht, err := newHostTester("TestSetAndGetInternalSettings")
	if err != nil {
		t.Fatal(err)
	}

	// Check the default settings get returned at first call.
	settings := ht.host.InternalSettings()
	if settings.AcceptingContracts != false {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MaxDuration != defaultMaxDuration {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MaxDownloadBatchSize != uint64(defaultMaxDownloadBatchSize) {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MaxReviseBatchSize != uint64(defaultMaxReviseBatchSize) {
		t.Error("settings retrieval did not return default value")
	}
	if settings.NetAddress != "" {
		t.Error("settings retrieval did not return default value")
	}
	if settings.WindowSize != defaultWindowSize {
		t.Error("settings retrieval did not return default value")
	}
	if settings.Collateral.Cmp(defaultCollateral) != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.CollateralBudget.Cmp(defaultCollateralBudget) != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MaxCollateralFraction.Cmp(defaultCollateralFraction) != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MaxCollateral.Cmp(defaultMaxCollateral) != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.DownloadLimitGrowth != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.DownloadLimitCap != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.DownloadSpeedLimit != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.UploadLimitGrowth != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.UploadLimitCap != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.UploadSpeedLimit != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MinimumContractPrice.Cmp(defaultContractPrice) != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MinimumDownloadBandwidthPrice.Cmp(defaultDownloadBandwidthPrice) != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MinimumStoragePrice.Cmp(defaultStoragePrice) != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.MinimumUploadBandwidthPrice.Cmp(defaultUploadBandwidthPrice) != 0 {
		t.Error("settings retrieval did not return default value")
	}

	// Check that calling SetInternalSettings with valid settings updates the settings.
	settings.AcceptingContracts = true
	settings.NetAddress = "foo.com:123"
	err = ht.host.SetInternalSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	settings = ht.host.InternalSettings()
	if settings.AcceptingContracts != true {
		t.Fatal("SetInternalSettings failed to update settings")
	}
	if settings.NetAddress != "foo.com:123" {
		t.Fatal("SetInternalSettings failed to update settings")
	}

	// Check that calling SetInternalSettings with invalid settings does not update the settings.
	settings.NetAddress = "invalid"
	err = ht.host.SetInternalSettings(settings)
	if err == nil {
		t.Fatal("expected SetInternalSettings to error with invalid settings")
	}
	settings = ht.host.InternalSettings()
	if settings.NetAddress != "foo.com:123" {
		t.Fatal("SetInternalSettings should not modify the settings if the new settings are invalid")
	}

	// Reload the host and verify that the altered settings persisted.
	err = ht.host.Close()
	if err != nil {
		t.Fatal(err)
	}
	rebootHost, err := New(ht.cs, ht.tpool, ht.wallet, "localhost:0", filepath.Join(ht.persistDir, modules.HostDir))
	if err != nil {
		t.Fatal(err)
	}
	rebootSettings := rebootHost.InternalSettings()
	if rebootSettings.AcceptingContracts != settings.AcceptingContracts {
		t.Error("settings retrieval did not return updated value")
	}
	if rebootSettings.NetAddress != settings.NetAddress {
		t.Error("settings retrieval did not return updated value")
	}
}

/*
// TestSetAndGetSettings checks that the functions for interacting with the
// hosts settings object are working as expected.
func TestSetAndGetSettings(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	ht, err := newHostTester("TestSetAndGetSettings")
	if err != nil {
		t.Fatal(err)
	}

	// Check the default settings get returned at first call.
	settings := ht.host.Settings()
	if settings.MaxDuration != defaultMaxDuration {
		t.Error("settings retrieval did not return default value")
	}
	if settings.WindowSize != defaultWindowSize {
		t.Error("settings retrieval did not return default value")
	}
	if settings.Price.Cmp(defaultPrice) != 0 {
		t.Error("settings retrieval did not return default value")
	}
	if settings.Collateral.Cmp(defaultCollateral) != 0 {
		t.Error("settings retrieval did not return default value")
	}

	// Submit updated settings and check that the changes stuck.
	settings.TotalStorage += 15
	settings.MaxDuration += 16
	settings.WindowSize += 17
	settings.Price = settings.Price.Add(types.NewCurrency64(18))
	settings.Collateral = settings.Collateral.Add(types.NewCurrency64(19))
	err = ht.host.SetSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	newSettings := ht.host.Settings()
	if settings.MaxDuration != newSettings.MaxDuration {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.WindowSize != newSettings.WindowSize {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.Price.Cmp(newSettings.Price) != 0 {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.Collateral.Cmp(newSettings.Collateral) != 0 {
		t.Error("settings retrieval did not return updated value")
	}

	// Reload the host and verify that the altered settings persisted.
	err = ht.host.Close()
	if err != nil {
		t.Fatal(err)
	}
	rebootHost, err := New(ht.cs, ht.tpool, ht.wallet, "localhost:0", filepath.Join(ht.persistDir, modules.HostDir))
	if err != nil {
		t.Fatal(err)
	}
	rebootSettings := rebootHost.Settings()
	if settings.TotalStorage != rebootSettings.TotalStorage {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.MaxDuration != rebootSettings.MaxDuration {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.WindowSize != rebootSettings.WindowSize {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.Price.Cmp(rebootSettings.Price) != 0 {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.Collateral.Cmp(rebootSettings.Collateral) != 0 {
		t.Error("settings retrieval did not return updated value")
	}
}

// TestPersistentSettings checks that settings persist between instances of the
// host.
func TestPersistentSettings(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	ht, err := newHostTester("TestSetPersistentSettings")
	if err != nil {
		t.Fatal(err)
	}

	// Submit updated settings.
	settings := ht.host.Settings()
	settings.TotalStorage += 25
	settings.MaxDuration += 36
	settings.WindowSize += 47
	settings.Price = settings.Price.Add(types.NewCurrency64(38))
	settings.Collateral = settings.Collateral.Add(types.NewCurrency64(99))
	err = ht.host.SetSettings(settings)
	if err != nil {
		t.Fatal(err)
	}

	// Reboot the host and verify that the new settings stuck.
	err = ht.host.Close() // host saves upon closing
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(ht.cs, ht.tpool, ht.wallet, "localhost:0", filepath.Join(ht.persistDir, modules.HostDir))
	if err != nil {
		t.Fatal(err)
	}
	newSettings := h.Settings()
	if settings.TotalStorage != newSettings.TotalStorage {
		t.Error("settings retrieval did not return updated value:", settings.TotalStorage, "vs", newSettings.TotalStorage)
	}
	if settings.MaxDuration != newSettings.MaxDuration {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.WindowSize != newSettings.WindowSize {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.Price.Cmp(newSettings.Price) != 0 {
		t.Error("settings retrieval did not return updated value")
	}
	if settings.Collateral.Cmp(newSettings.Collateral) != 0 {
		t.Error("settings retrieval did not return updated value")
	}
}
*/

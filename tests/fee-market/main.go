package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/Inphi/eip4844-interop/tests/ctrl"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/libp2p/go-libp2p"
	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/protolambda/ztyp/view"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/p2p/encoder"
	beaconchainsync "github.com/prysmaticlabs/prysm/v3/beacon-chain/sync"
	consensustypes "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	ethpbv1 "github.com/prysmaticlabs/prysm/v3/proto/eth/v1"
	ethpbv2 "github.com/prysmaticlabs/prysm/v3/proto/eth/v2"
	ethpb "github.com/prysmaticlabs/prysm/v3/proto/prysm/v1alpha1"
)

func GetBlob() types.Blobs {
	// dummy data for the test
	return shared.EncodeBlobs([]byte("EKANS"))
}

// 1. Uploads *multiple* blobs
// 2. Downloads blobs
// 3. Asserts that downloaded blobs match the upload
// 4. Asserts execution and beacon block attributes
func main() {
	ctrl.InitE2ETest()
	ctrl.WaitForShardingFork()
	ctrl.WaitForEip4844ForkEpoch()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20)
	defer cancel()

	blobsData := make([]types.Blobs, 20)
	for i := range blobsData {
		blobsData[i] = GetBlob()
	}

	// Retrieve the current slot to being our blobs search on the beacon chain
	startSlot := GetHeadSlot(ctx)

	// Send multiple transactions at the same time to induce non-zero excess_blobs
	UploadBlobsAndCheckBlockHeader(ctx, blobsData)

	WaitForNextSlot(ctx)
	WaitForNextSlot(ctx)

	blocks := FindBlocksWithBlobs(ctx, startSlot)

	log.Printf("checking blob from beacon node")
	var downloadedData []byte
	for _, b := range blocks {
		data := DownloadBlobs(ctx, b.Slot, 1, shared.BeaconMultiAddress)
		downloadedData = append(downloadedData, data...)
	}

	flatBlobs := FlattenBlobs(blobsData)

	if !bytes.Equal(flatBlobs, downloadedData) {
		log.Fatalf("mismatch %d %v", len(flatBlobs), len(downloadedData))
	}

	log.Printf("checking blob from beacon node follower")
	time.Sleep(time.Second * 2 * time.Duration(ctrl.Env.BeaconChainConfig.SecondsPerSlot)) // wait a bit for sync

	downloadedData = nil
	for _, b := range blocks {
		data := DownloadBlobs(ctx, b.Slot, 1, shared.BeaconFollowerMultiAddress)
		downloadedData = append(downloadedData, data...)
	}
	if !bytes.Equal(flatBlobs, downloadedData) {
		log.Fatalf("mismatch %d %v", len(flatBlobs), len(downloadedData))
	}
}

func FlattenBlobs(blobsData []types.Blobs) []byte {
	var out []byte
	for _, blobs := range blobsData {
		for _, blob := range blobs {
			rawBlob := make([][]byte, len(blob))
			for i := range blob {
				rawBlob[i] = blob[i][:]
			}
			decoded := shared.DecodeBlob(rawBlob)
			out = append(out, decoded...)
		}
	}
	return out
}

func WaitForNextSlot(ctx context.Context) {
	if err := ctrl.WaitForSlot(ctx, GetHeadSlot(ctx).Add(1)); err != nil {
		log.Fatalf("error waiting for next slot: %v", err)
	}
}

func GetHeadSlot(ctx context.Context) consensustypes.Slot {
	req := &ethpbv1.BlockRequest{BlockId: []byte("head")}
	header, err := ctrl.Env.BeaconChainClient.GetBlockHeader(ctx, req)
	if err != nil {
		log.Fatalf("unable to get beacon chain head: %v", err)
	}
	return header.Data.Header.Message.Slot
}

func UploadBlobsAndCheckBlockHeader(ctx context.Context, blobsData []types.Blobs) {
	chainId := big.NewInt(1)
	signer := types.NewDankSigner(chainId)

	key, err := crypto.HexToECDSA(shared.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	nonce, err := ctrl.Env.EthClient.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	var txs []*types.Transaction
	for i := range blobsData {
		blobs := blobsData[i]
		commitments, versionedHashes, aggregatedProof, err := blobs.ComputeCommitmentsAndAggregatedProof()
		to := common.HexToAddress("ffb38a7a99e3e2335be83fc74b7faa19d5531243")
		txData := types.SignedBlobTx{
			Message: types.BlobTxMessage{
				ChainID:             view.Uint256View(*uint256.NewInt(chainId.Uint64())),
				Nonce:               view.Uint64View(nonce + uint64(i)),
				Gas:                 210000,
				GasFeeCap:           view.Uint256View(*uint256.NewInt(5000000000)),
				GasTipCap:           view.Uint256View(*uint256.NewInt(5000000000)),
				MaxFeePerDataGas:    view.Uint256View(*uint256.NewInt(3000000000)), // needs to be at least the min fee
				Value:               view.Uint256View(*uint256.NewInt(12345678)),
				To:                  types.AddressOptionalSSZ{Address: (*types.AddressSSZ)(&to)},
				BlobVersionedHashes: versionedHashes,
			},
		}

		wrapData := types.BlobTxWrapData{
			BlobKzgs:           commitments,
			Blobs:              blobs,
			KzgAggregatedProof: aggregatedProof,
		}
		tx := types.NewTx(&txData, types.WithTxWrapData(&wrapData))
		tx, err = types.SignTx(tx, signer, key)
		if err != nil {
			log.Fatalf("Error signing tx: %v", err)
		}
		txs = append(txs, tx)
	}

	receipts := make(chan *types.Receipt, len(txs))
	var wg sync.WaitGroup
	wg.Add(len(txs))
	for _, tx := range txs {
		tx := tx
		go func() {
			defer wg.Done()
			err := ctrl.Env.EthClient.SendTransaction(ctx, tx)
			if err != nil {
				log.Fatalf("Error sending tx: %v", err)
			}

			log.Printf("Waiting for transaction (%v) to be included...", tx.Hash())

			receipt, err := shared.WaitForReceipt(ctx, ctrl.Env.EthClient, tx.Hash())
			if err != nil {
				log.Fatalf("Error waiting for transaction receipt %v: %v", tx.Hash(), err)
			}
			receipts <- receipt
		}()
	}
	wg.Wait()
	close(receipts)

	log.Printf("checking mined blocks...")
	blockNumbers := make(map[uint64]bool)
	var blocks []*types.Block
	for receipt := range receipts {
		blocknum := receipt.BlockNumber.Uint64()
		if _, ok := blockNumbers[blocknum]; !ok {
			blockHash := receipt.BlockHash.Hex()
			block, err := ctrl.Env.EthClient.BlockByHash(ctx, common.HexToHash(blockHash))
			if err != nil {
				log.Fatalf("Error getting block: %v", err)
			}
			excessDataGas := block.ExcessDataGas()
			if excessDataGas == nil {
				log.Fatalf("nil excess_blobs in block header. block_hash=%v", blockHash)
			}
			blockNumbers[blocknum] = true
			blocks = append(blocks, block)
		}
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Number().Uint64() < blocks[j].Number().Uint64()
	})

	prevExcessDataGas := new(big.Int)
	parentBlock, err := ctrl.Env.EthClient.BlockByHash(ctx, blocks[0].ParentHash())
	if err != nil {
		log.Fatalf("Error getting block: %v", err)
	}
	if e := parentBlock.ExcessDataGas(); e != nil {
		prevExcessDataGas.Set(e)
	}

	for _, block := range blocks {
		// Assuming each transaction contains a single blob
		expected := misc.CalcExcessDataGas(prevExcessDataGas, len(block.Transactions()))
		if expected.Cmp(block.ExcessDataGas()) != 0 {
			log.Fatalf("unexpected excess_data_gas field in header. expected %v. got %v", expected, block.ExcessDataGas())
		}
		prevExcessDataGas = expected
	}
}

func FindBlocksWithBlobs(ctx context.Context, startSlot consensustypes.Slot) []*ethpbv2.BeaconBlockEip4844 {
	slot := startSlot
	endSlot := GetHeadSlot(ctx)

	var blocks []*ethpbv2.BeaconBlockEip4844
	for {
		if slot == endSlot {
			break
		}

		blockID := fmt.Sprintf("%d", uint64(slot))
		req := &ethpbv2.BlockRequestV2{BlockId: []byte(blockID)}
		block, err := ctrl.Env.BeaconChainClient.GetBlockV2(ctx, req)
		if err != nil {
			log.Fatalf("beaconchainclient.GetBlock: %v", err)
		}
		eip4844, ok := block.Data.Message.(*ethpbv2.SignedBeaconBlockContainer_Eip4844Block)
		if ok {
			if len(eip4844.Eip4844Block.Body.BlobKzgs) != 0 {
				blocks = append(blocks, eip4844.Eip4844Block)
			}
		}

		slot = slot.Add(1)
	}

	if len(blocks) == 0 {
		log.Fatalf("Unable to find beacon block containing blobs")
	}
	return blocks
}

func DownloadBlobs(ctx context.Context, startSlot consensustypes.Slot, count uint64, beaconMA string) []byte {
	// TODO: Use Beacon gRPC to download blobs rather than p2p RPC
	log.Print("downloading blobs...")

	req := &ethpb.BlobsSidecarsByRangeRequest{
		StartSlot: startSlot,
		Count:     count,
	}

	h, err := libp2p.New()
	if err != nil {
		log.Fatalf("failed to create libp2p context: %v", err)
	}
	defer func() {
		_ = h.Close()
	}()

	multiaddr, err := getMultiaddr(ctx, h, beaconMA)
	if err != nil {
		log.Fatalf("getMultiAddr: %v", err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(multiaddr)
	if err != nil {
		log.Fatal(err)
	}

	err = h.Connect(ctx, *addrInfo)
	if err != nil {
		log.Fatalf("libp2p host connect: %v", err)
	}

	// Hack to ensure that we are able to download blob chunks with larger chunk sizes (which is 10 MiB post-bellatrix)
	encoder.MaxChunkSize = 10 << 20
	sidecars, err := sendBlobsSidecarsByRangeRequest(ctx, h, encoder.SszNetworkEncoder{}, addrInfo.ID, req)
	if err != nil {
		log.Fatalf("failed to send blobs p2p request: %v", err)
	}

	anyBlobs := false
	blobsBuffer := new(bytes.Buffer)
	for _, sidecar := range sidecars {
		log.Printf("found sidecar with %d blobs", len(sidecar.Blobs))
		if sidecar.Blobs == nil || len(sidecar.Blobs) == 0 {
			continue
		}
		anyBlobs = true
		for _, blob := range sidecar.Blobs {
			data := shared.DecodeBlob(blob.Blob)
			_, _ = blobsBuffer.Write(data)
		}

		// stop after the first sidecar with blobs:
		break
	}

	if !anyBlobs {
		log.Fatalf("No blobs found in requested slots, sidecar count: %d", len(sidecars))
	}

	return blobsBuffer.Bytes()
}

func getMultiaddr(ctx context.Context, h host.Host, addr string) (ma.Multiaddr, error) {
	multiaddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}
	_, id := peer.SplitAddr(multiaddr)
	if id != "" {
		return multiaddr, nil
	}
	// peer ID wasn't provided, look it up
	id, err = retrievePeerID(ctx, h, addr)
	if err != nil {
		return nil, err
	}
	return ma.NewMultiaddr(fmt.Sprintf("%s/p2p/%s", addr, string(id)))
}

// Helper for retrieving the peer ID from a security error... obviously don't use this in production!
// See https://github.com/libp2p/go-libp2p-noise/blob/v0.3.0/handshake.go#L250
func retrievePeerID(ctx context.Context, h host.Host, addr string) (peer.ID, error) {
	incorrectPeerID := "16Uiu2HAmSifdT5QutTsaET8xqjWAMPp4obrQv7LN79f2RMmBe3nY"
	addrInfo, err := peer.AddrInfoFromString(fmt.Sprintf("%s/p2p/%s", addr, incorrectPeerID))
	if err != nil {
		return "", err
	}
	err = h.Connect(ctx, *addrInfo)
	if err == nil {
		return "", errors.New("unexpected successful connection")
	}
	if strings.Contains(err.Error(), "but remote key matches") {
		split := strings.Split(err.Error(), " ")
		return peer.ID(split[len(split)-1]), nil
	}
	return "", err
}

func sendBlobsSidecarsByRangeRequest(ctx context.Context, h host.Host, encoding encoder.NetworkEncoding, pid peer.ID, req *ethpb.BlobsSidecarsByRangeRequest) ([]*ethpb.BlobsSidecar, error) {
	topic := fmt.Sprintf("%s%s", p2p.RPCBlobsSidecarsByRangeTopicV1, encoding.ProtocolSuffix())

	stream, err := h.NewStream(ctx, pid, protocol.ID(topic))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = stream.Close()
	}()

	if _, err := encoding.EncodeWithMaxLength(stream, req); err != nil {
		_ = stream.Reset()
		return nil, err
	}

	if err := stream.CloseWrite(); err != nil {
		_ = stream.Reset()
		return nil, err
	}

	var blobsSidecars []*ethpb.BlobsSidecar
	for {
		blobs, err := readChunkedBlobsSidecar(stream, encoding)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		blobsSidecars = append(blobsSidecars, blobs)
	}
	return blobsSidecars, nil
}

func readChunkedBlobsSidecar(stream libp2pcore.Stream, encoding encoder.NetworkEncoding) (*ethpb.BlobsSidecar, error) {
	code, errMsg, err := beaconchainsync.ReadStatusCode(stream, encoding)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New(errMsg)
	}
	sidecar := new(ethpb.BlobsSidecar)
	err = encoding.DecodeWithMaxLength(stream, sidecar)
	return sidecar, err
}

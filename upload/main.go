package main

import (
	"context"
	"flag"
	"log"
	"math"
	"math/big"
	"encoding/hex"
	"os"
	"time"

	"github.com/Inphi/eip4844-interop/shared"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/crypto/kzg"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/protolambda/go-kzg/bls"
	gokzg "github.com/protolambda/go-kzg"
	"github.com/holiman/uint256"
	"github.com/protolambda/ztyp/view"
)

func main() {
	prv := "45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
	addr := "http://localhost:8545"

	before := flag.Uint64("before", 0, "Block to wait for before submitting transaction")
	after := flag.Uint64("after", 0, "Block to wait for after submitting transaction")
	flag.Parse()

	file := flag.Arg(0)
	if file == "" {
		log.Fatalf("File parameter missing")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	chainId := big.NewInt(1)
	signer := types.NewDankSigner(chainId)

	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, addr)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	key, err := crypto.HexToECDSA(prv)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	if *before > 0 {
		waitForBlock(ctx, client, *before)
	}

	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	blobs := shared.EncodeBlobs(data)
	var commitments []types.KZGCommitment
	var hashes []common.Hash
	for index, b := range blobs {
		c, ok := b.ComputeCommitment()
		if !ok {
			panic("Could not compute commitment")
		}
		commitments = append(commitments, c)
		versionedHash := c.ComputeVersionedHash()
		hashes = append(hashes, versionedHash)
		text, _ := c.MarshalText()
		log.Printf("Commitment: %s", text)
		createProof(index, b, c, versionedHash)
	}
	to := common.HexToAddress("ffb38a7a99e3e2335be83fc74b7faa19d5531243")
	txData := types.SignedBlobTx{
		Message: types.BlobTxMessage{
			ChainID:             view.Uint256View(*uint256.NewInt(chainId.Uint64())),
			Nonce:               view.Uint64View(nonce),
			Gas:                 210000,
			GasFeeCap:           view.Uint256View(*uint256.NewInt(5000000000)),
			GasTipCap:           view.Uint256View(*uint256.NewInt(5000000000)),
			Value:               view.Uint256View(*uint256.NewInt(12345678)),
			To:                  types.AddressOptionalSSZ{Address: (*types.AddressSSZ)(&to)},
			BlobVersionedHashes: hashes,
		},
	}

	wrapData := types.BlobTxWrapData{
		BlobKzgs: commitments,
		Blobs:    blobs,
	}
	tx := types.NewTx(&txData, types.WithTxWrapData(&wrapData))
	tx, err = types.SignTx(tx, signer, key)
	if err != nil {
		log.Fatalf("Error signing tx: %v", err)
	}

	err = client.SendTransaction(ctx, tx)
	if err != nil {
		log.Fatalf("Error sending tx: %v", err)
	}

	log.Printf("Transaction submitted. hash=%v", tx.Hash())

	if *after > 0 {
		waitForBlock(ctx, client, *after)
	}
}

func waitForBlock(ctx context.Context, client *ethclient.Client, block uint64) {
	for {
		bn, err := client.BlockNumber(ctx)
		if err != nil {
			log.Fatalf("Error requesting block number: %v", err)
		}
		if bn >= block {
			return
		}
		log.Printf("Waiting for block %d, current %d", block, bn)
		time.Sleep(1 * time.Second)
	}
}

func createProof(index int, blob types.Blob, commitment types.KZGCommitment, versionedHash common.Hash) {
	evalPoly, err := blob.Parse()
	if err != nil {
			log.Fatalf("Error parsing blob field elements: %v", err)
	}

	fs := gokzg.NewFFTSettings(uint8(math.Log2(params.FieldElementsPerBlob)))
	polynomial, err := fs.FFT(evalPoly, true)
	if err != nil {
			log.Fatalf("Error reverse evaluating poly: %v", err)
	}

	x := uint64(0x4)
	proof := computeProof(polynomial, x, kzg.KzgSetupG1)

	// Get actual evaluation at x
	var xFr bls.Fr
	bls.AsFr(&xFr, x)
	var y bls.Fr
	bls.EvalPolyAt(&y, polynomial, &xFr)

	// Verify kzg proof
	commitmentPoint, _ := commitment.Point()
	if kzg.VerifyKzgProof(commitmentPoint, &xFr, &y, proof) != true {
			panic("failed proof verification")
	}

	var commitmentBytes types.KZGCommitment
	copy(commitmentBytes[:], bls.ToCompressedG1(commitmentPoint))

	proofBytes := bls.ToCompressedG1(proof)
	xBytes := bls.FrTo32(&xFr)
	yBytes := bls.FrTo32(&y)

	calldata := append(versionedHash[:], xBytes[:]...)
	calldata = append(calldata, yBytes[:]...)
	calldata = append(calldata, commitmentBytes[:]...)
	calldata = append(calldata, proofBytes...)

	log.Printf(
			"Blob %d Version Hash: %s, Evaluation Point: %s Expected Ouput: %s, Commitment: %s, Proof: %s",
			index,
			hex.EncodeToString(versionedHash.Bytes()),
			hex.EncodeToString(xBytes[:]),
			hex.EncodeToString(yBytes[:]),
			hex.EncodeToString(commitmentBytes[:]),
			hex.EncodeToString(proofBytes),
	)

	log.Printf(
			"Blob %d Point Evaluation Input: %s",
			index,
			"0x"+hex.EncodeToString(calldata),
	)

	precompile := vm.PrecompiledContractsDanksharding[common.BytesToAddress([]byte{0x14})]
	if _, err := precompile.Run(calldata); err != nil {
			log.Fatalf("expected point verification to succeed: %s", err.Error())
	}
}

func computeProof(poly []bls.Fr, x uint64, crsG1 []bls.G1Point) *bls.G1Point {
	// divisor = [-x, 1]
	divisor := [2]bls.Fr{}
	var tmp bls.Fr
	bls.AsFr(&tmp, x)
	bls.SubModFr(&divisor[0], &bls.ZERO, &tmp)
	bls.CopyFr(&divisor[1], &bls.ONE)
	//for i := 0; i < 2; i++ {
	//	fmt.Printf("div poly %d: %s\n", i, FrStr(&divisor[i]))
	//}
	// quot = poly / divisor
	quotientPolynomial := polyLongDiv(poly, divisor[:])
	//for i := 0; i < len(quotientPolynomial); i++ {
	//	fmt.Printf("quot poly %d: %s\n", i, FrStr(&quotientPolynomial[i]))
	//}

	// evaluate quotient poly at shared secret, in G1
	return bls.LinCombG1(crsG1[:len(quotientPolynomial)], quotientPolynomial)
}

func polyLongDiv(dividend []bls.Fr, divisor []bls.Fr) []bls.Fr {
	a := make([]bls.Fr, len(dividend))
	for i := 0; i < len(a); i++ {
		bls.CopyFr(&a[i], &dividend[i])
	}
	aPos := len(a) - 1
	bPos := len(divisor) - 1
	diff := aPos - bPos
	out := make([]bls.Fr, diff+1)
	for diff >= 0 {
		quot := &out[diff]
		polyFactorDiv(quot, &a[aPos], &divisor[bPos])
		var tmp, tmp2 bls.Fr
		for i := bPos; i >= 0; i-- {
			// In steps: a[diff + i] -= b[i] * quot
			// tmp =  b[i] * quot
			bls.MulModFr(&tmp, quot, &divisor[i])
			// tmp2 = a[diff + i] - tmp
			bls.SubModFr(&tmp2, &a[diff+i], &tmp)
			// a[diff + i] = tmp2
			bls.CopyFr(&a[diff+i], &tmp2)
		}
		aPos -= 1
		diff -= 1
	}
	return out
}

// Helper: invert the divisor, then multiply
func polyFactorDiv(dst *bls.Fr, a *bls.Fr, b *bls.Fr) {
	// TODO: use divmod instead.
	var tmp bls.Fr
	bls.InvModFr(&tmp, b)
	bls.MulModFr(dst, &tmp, a)
}

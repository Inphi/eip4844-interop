package main

import (
	"context"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/Inphi/eip4844-interop/tests/util"
	consensustypes "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
)

func main() {
	ctx := context.Background()
	blobSlot := consensustypes.Slot(32)
	followerMultiaddr := ""
	blobs := shared.EncodeBlobs([]byte("EKANS"))
	downloadedData := util.DownloadBlobs(ctx, blobSlot, 1, followerMultiaddr)
	downloadedBlobs := shared.EncodeBlobs(downloadedData)
	util.AssertBlobsEquals(blobs, downloadedBlobs)

	// beaconClient, err := ctrl.GetBeaconNodeClient(ctx)
	// if err != nil {
	// 	log.Fatalf("unable to get beacon client: %v", err)
	// }

	// headSlot := util.GetHeadSlot(ctx, beaconClient)
	// fmt.Println(headSlot)
}

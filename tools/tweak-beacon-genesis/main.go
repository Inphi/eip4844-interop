package main

import (
	"flag"
	"io/fs"
	"io/ioutil"
	"log"

	ethpb "github.com/prysmaticlabs/prysm/proto/prysm/v1alpha1"
	bytesutil2 "github.com/wealdtech/go-bytesutil"
)

var (
	file          = flag.String("file", "", "genesis.ssz file")
	eth1BlockHash = flag.String("eth1-blockhash", "", "eth1 block hash override")
)

func main() {
	flag.Parse()

	buf, err := ioutil.ReadFile(*file)
	if err != nil {
		log.Fatalf("unable to read genesis file: %v", err)
	}

	genesisState := new(ethpb.BeaconState)
	err = genesisState.UnmarshalSSZ(buf)
	if err != nil {
		log.Fatalf("Could not decode the genesis file: %v", err)
	}
	genesisState.Eth1Data.BlockHash, err = bytesutil2.FromHexString(*eth1BlockHash)
	if err != nil {
		log.Fatalf("invalid hex")
	}

	b, err := genesisState.MarshalSSZ()
	if err != nil {
		log.Fatalf("Could not marshal genesis: %v", err)
	}
	err = ioutil.WriteFile(*file, b, fs.ModePerm)
	if err != nil {
		log.Fatalf("Could not write genesis file: %v", err)
	}
}

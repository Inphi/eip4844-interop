package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/params"
)

func GetBaseDir() string {
	path := os.Getenv("TEST_INTEROP_BASEDIR")
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			log.Printf("error geting interop basedir (%v). defaulting to /", err)
		}
	}
	return path
}

func GethChainConfigFilepath() string {
	return fmt.Sprintf("%s/shared/generated-genesis.json", GetBaseDir())
}

func BeaconChainConfigFilepath() string {
	return fmt.Sprintf("%s/shared/chain-config.yml", GetBaseDir())
}

func UpdateChainConfig(config *params.ChainConfig) error {
	file, err := json.MarshalIndent(config, "", " ")
	if err != nil {
		return err
	}
	path := GethChainConfigFilepath()
	return ioutil.WriteFile(path, file, 0644)
}

func GetBeaconMultiAddress() (string, error) {
	return getMultiaddress("http://" + BeaconAPI)
}

func GetBeaconFollowerMultiAddress() (string, error) {
	return getMultiaddress("http://" + BeaconFollowerAPI)
}

func getMultiaddress(beaconAPI string) (string, error) {
	url := fmt.Sprintf("%s/eth/v1/node/identity", beaconAPI)
	r, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	type Data struct {
		Data struct {
			P2PAddresses []string `json:"p2p_addresses"`
		} `json:"data"`
	}
	var data Data
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return "", err
	}
	if len(data.Data.P2PAddresses) == 0 {
		return "", errors.New("no multiaddresses found")
	}

	// There might be multiple addresses and we need the one with 127.0.0.1 to work
	for _, addr := range data.Data.P2PAddresses {
		if strings.Contains(addr, "127.0.0.1") {
			return addr, nil
		}
	}

	return data.Data.P2PAddresses[0], nil
}

var (
	GethRPC           = "http://localhost:8545"
	PrivateKey        = "45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
	BeaconAPI         = "localhost:3500"
	BeaconFollowerAPI = "localhost:3501"
	ValidatorAPI      = "http://localhost:7500"
)

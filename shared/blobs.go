package shared

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func EncodeBlobs(data []byte) types.Blobs {
	blobs := []types.Blob{{}}
	blobIndex := 0
	fieldIndex := -1
	for i := 0; i < len(data); i += 31 {
		fieldIndex++
		if fieldIndex == params.FieldElementsPerBlob {
			blobs = append(blobs, types.Blob{})
			blobIndex++
			fieldIndex = 0
		}
		max := i + 31
		if max > len(data) {
			max = len(data)
		}
		copy(blobs[blobIndex][fieldIndex][:], data[i:max])
	}
	return blobs
}

func DecodeBlobs(blobs [][]byte) []byte {
	var data []byte
	for _, b := range blobs {
		// ignore the last byte in every 32-byte blob (see encoding in EncodeBlobs)
		data = append(data, b[0:31]...)
	}
	return TrimArray(data)
}

func DecodeBlob(blob []byte) []byte {
	var data []byte
	for i, b := range blob {
		// ignore the last byte in every 32-byte block (see encoding in EncodeBlobs)
		if (i+1)%32 == 0 {
			continue
		}
		data = append(data, b)
	}
	return TrimArray(data)
}

func TrimArray(data []byte) []byte {
	// XXX: the following removes trailing 0s, which could be unexpected for certain blobs
	i := len(data) - 1
	for ; i >= 0; i-- {
		if data[i] != 0x00 {
			break
		}
	}
	data = data[:i+1]
	return data
}

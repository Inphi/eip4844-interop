const ethers = require("ethers")
const axios = require('axios')

const input = process.argv[2]
const expected_kzgs = process.argv[3]
const provider = new ethers.providers.JsonRpcProvider("http://localhost:8545")

const BYTES_PER_FIELD_ELEMENT = 32
const FIELD_ELEMENTS_PER_BLOB = 4096
const USEFUL_BYTES_PER_BLOB = 32 * FIELD_ELEMENTS_PER_BLOB
const MAX_BLOBS_PER_TX = 2
const MAX_USEFUL_BYTES_PER_TX = (USEFUL_BYTES_PER_BLOB * MAX_BLOBS_PER_TX) - 1
const BLOB_SIZE = BYTES_PER_FIELD_ELEMENT * FIELD_ELEMENTS_PER_BLOB

function get_padded(data, blobs_len) {
    let pdata = Buffer.alloc(blobs_len * USEFUL_BYTES_PER_BLOB)
    const datalen = Buffer.byteLength(data)
    pdata.fill(data, 0, datalen)
    // TODO: if data already fits in a pad, then ka-boom
    pdata[datalen] = 0x80
    return pdata
}

function get_blob(data) {
    let blob = Buffer.alloc(BLOB_SIZE, 'binary')
    for (let i = 0; i < FIELD_ELEMENTS_PER_BLOB; i++) {
        let chunk = Buffer.alloc(32, 'binary')
        chunk.fill(data.subarray(i*31, (i+1)*31), 0, 31)
        blob.fill(chunk, i*32, (i+1)*32)
    }

    return blob
}

// ref: https://github.com/asn-d6/blobbers/blob/packing_benchmarks/src/packer_naive.rs
function get_blobs(data) {
    data = Buffer.from(data, 'binary')
    const len = Buffer.byteLength(data)
    if (len === 0) {
        throw Error("invalid blob data")
    }
    if (len > MAX_USEFUL_BYTES_PER_TX) {
        throw Error("blob data is too large")
    }

    const blobs_len = Math.ceil(len / USEFUL_BYTES_PER_BLOB)

    const pdata = get_padded(data, blobs_len)

    let blobs = []
    for (let i = 0; i < blobs_len; i++) {
        let chunk = pdata.subarray(i*USEFUL_BYTES_PER_BLOB, (i+1)*USEFUL_BYTES_PER_BLOB)
        let blob = get_blob(chunk)
        blobs.push(blob)
    }

    return blobs
}

function sleep(ms) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

async function estimateGas(tx) {
    const req = {
        "id": "1",
        "jsonrpc": "2.0",
        "method": "eth_estimateGas",
        "params": [tx]
    }
    const res = await axios.post("http://localhost:8545", req)
    return res.data.result
}

async function run(data, expected_kzgs) {
    while (true) {
        const num = await provider.getBlockNumber()
        if (num >= 9) {
            break
        }
        console.log(`waiting for eip4844 proc.... bn=${num}`)
        await sleep(1000)
    }
    let blobs = get_blobs(data)
    console.log("number of blobs is " + blobs.length)
    const blobshex = blobs.map(x => `0x${x.toString("hex")}`)

    const account = ethers.Wallet.createRandom()
    const txData = {
        "from": "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b",
        "to": account.address,
        "data": "0x",
        "chainId": "0x1",
        "blobs": blobshex,
    }
    txData["gas"] = await estimateGas(txData)

    const req = {
        "id": "1",
        "jsonrpc": "2.0",
        "method": "eth_sendTransaction",
        "params": [txData]
    }
    console.log(`sending to ${account.address}`)
    const res = await axios.post("http://localhost:8545", req)
    console.log(res.data)
    if (res.data.error) {
        return false
    }

    if (expected_kzgs === undefined) {
        return true
    }

    let blob_kzg = null
    try {
        let start = (await axios.get("http://localhost:3500/eth/v1/beacon/headers")).data.data[0].header.message.slot - 1
        for (let i = 0; i < 5; i++) {
            const res = (await axios.get(`http://localhost:3500/eth/v2/beacon/blocks/${start + i}`)).data.data.message.body.blob_kzgs
            if (res.length > 0) {
                blob_kzg = res[0]
            }
            while (true) {
                const current = (await axios.get("http://localhost:3500/eth/v1/beacon/headers")).data.data[0].header.message.slot - 1
                if (current > start + i) {
                    break
                }
                console.log(`waiting for tx to be included in block.... bn=${current}`)
                await sleep(1000)
            }
        }
    } catch(error) {
        console.log(`Error retrieving blocks from ${error.config.url}: ${error.response.data}`)
        return false
    }

    if (blob_kzg !== expected_kzgs) {
        console.log(`Unexpected KZG value: expected ${expected_kzgs}, got ${blob_kzg}`)
        return false
    } else {
        console.log(`Found expected KZG value: ${blob_kzg}`)
    }

    return true
}

(async () => { process.exit((await run(input, expected_kzgs)) ? 0 : 1) })()

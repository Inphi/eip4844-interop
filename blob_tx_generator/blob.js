const ethers = require("ethers")
const axios = require('axios')

const input = process.argv[2]
const privateKey = "0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
const provider = new ethers.providers.JsonRpcProvider("http://localhost:8545")
const wallet = new ethers.Wallet(privateKey, provider)

const BYTES_PER_FIELD_ELEMENT = 32
const FIELD_ELEMENTS_PER_BLOB = 1016
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
    if (len == 0) {
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

async function run(data) {
    while (true) {
        const num = await provider.getBlockNumber()
        if (num >= 6) {
            break
        }
        console.log(`waiting for eip4844 proc.... bn=${num}`)
        await sleep(1000)
    }
    let blobs = get_blobs(data)
    console.log("number of blobs is " + blobs.length)
    const bb = blobs.toString('binary')
    const blobshex = blobs.map((x) => { x.toString('hex') })

    const account = ethers.Wallet.createRandom()
    const txData = {
        "from": "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b",
        "to": account.address,
        "data": "0x",
        "chainId": "0x1",
        "blobs": blobshex,
    }
    const gas = await estimateGas(txData)
    txData["gas"] = gas

    const req = {
        "id": "1",
        "jsonrpc": "2.0",
        "method": "eth_sendTransaction",
        "params": [txData]
    }
    console.log(`sending to ${account.address}`)
    const res = await axios.post("http://localhost:8545", req)
    console.log(res.data)
}

(async () => { run(input) })()

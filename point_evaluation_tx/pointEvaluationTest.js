const { solidity } = require('ethereum-waffle')
const chai = require("chai");
const ethers = require("ethers");

const { abi, evm } = require('./compile');


const GAS_CONFIG = {gasPrice: 100, gasLimit: 1000000};
const POINT_EVALUATION_INPUT = "0x0129f3efd12f6b12645260a7324926389d0d0420adfe97418432ef8c87b0dd9442000000000000000000000000000000000000000000000000000000000000002feb813e53e58d635820600b96afec7b7fde1e427110da98bec5aa1ff39c4150921203c4718ab2c572ca3018b1a3b3efda6c5226e9459d78920d971ad82c571d13503dc5b3276ebe7b5e32ac4f72a0e885525186c0ea46b6f6f2e8446ba56e2d19c4e4ce9bd731455a9985441bca379f11689ce57722dcddf0924c6979e44696"
const EXECUTION_NODE_RPC = "http://localhost:8545";
const PRIVATE_KEY = "0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8";


async function main() {
    chai.use(solidity);
    let provider = await new ethers.providers.JsonRpcProvider(EXECUTION_NODE_RPC);
    let signer = new ethers.Wallet(PRIVATE_KEY, provider);

    const PointEvaluationTest= await new ethers.ContractFactory(abi, evm.bytecode.object, signer);
    console.log("running script")
    
    let tx = await PointEvaluationTest.deploy("0x", GAS_CONFIG)
    await chai.expect(tx.deployed()).to.be.reverted;
    console.log("Point evaluation test failure")
    
    tx = await PointEvaluationTest.deploy(POINT_EVALUATION_INPUT, GAS_CONFIG)
    await chai.expect(tx.deployed()).to.not.be.reverted;
    console.log("Point evaluation success test pass")
    
  }

main().catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
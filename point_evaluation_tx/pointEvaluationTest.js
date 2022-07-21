const { solidity } = require('ethereum-waffle')
const chai = require("chai");
const ethers = require("ethers");

const { abi, evm } = require('./compile');


const GAS_CONFIG = {gasPrice: 100000, gasLimit: 1000000};
const EXECUTION_NODE_RPC = "http://localhost:8545";
const PRIVATE_KEY = "0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8";


async function main(pointEvaluationInput) {
    chai.use(solidity);
    let provider = await new ethers.providers.JsonRpcProvider(EXECUTION_NODE_RPC);
    let signer = new ethers.Wallet(PRIVATE_KEY, provider);

    const PointEvaluationTest= await new ethers.ContractFactory(abi, evm.bytecode.object, signer);
    console.log("Calling Point Evaluation Precompile")

    tx = await PointEvaluationTest.deploy(pointEvaluationInput, GAS_CONFIG)
    await chai.expect(tx.deployed()).to.not.be.reverted;
    
    console.log("Point Evaluation Test Passed")

  }

main(process.argv[2]).catch((error) => {
    console.error(error);
    process.exitCode = 1;
  }); 
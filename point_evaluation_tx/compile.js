const path = require('path');
const fs = require('fs');
const solc = require('solc');
 
const source = fs.readFileSync("./PointEvaluationTest.sol", 'utf8');

const input = {
    language: 'Solidity',
    sources: {
        'PointEvaluationTest.sol': {
            content: source,
        },
    },
    settings: {
        outputSelection: {
            '*': {
                '*': ['*'],
            },
        },
    },
};

module.exports = JSON.parse(solc.compile(JSON.stringify(input))).contracts[
    'PointEvaluationTest.sol'
].PointEvaluationTest;

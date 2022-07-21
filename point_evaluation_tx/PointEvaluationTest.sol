//SPDX-License-Identifier: Unlicense
pragma solidity ^0.8.0;

contract PointEvaluationTest {
    constructor(bytes memory input) {
        assembly {
            if iszero(staticcall(gas(), 0x14, mload(input), 0xc0, 0, 0)) {
                revert(0,0)
            }
        }
    }
}
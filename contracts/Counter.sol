// SPDX-License-Identifier: MIT
pragma solidity >= 0.6.0 <0.9.0;

import "Bank.sol";

address constant BANK_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000000064;
IBankModule constant BANK_CONTRACT = IBankModule(BANK_PRECOMPILE_ADDRESS);

contract Counter {
    uint256 counter = 0;

    event Increased(address who, uint256 oldValue, uint256 newValue);
    event Decreased(address who, uint256 oldValue, uint256 newValue);

    function add() public {
        emit Increased(msg.sender, counter, counter+1);
        counter++;
    }
    
    function addValue(uint256 value) public {
        emit Increased(msg.sender, counter, counter+value);
        counter = counter + value;
    }
    
    function subtract() public {
        emit Decreased(msg.sender, counter, counter-1);
        counter--;
    }
    
    function subtractValue(uint256 value) public {
        emit Decreased(msg.sender, counter, counter-value);
        counter = counter - value;
    }
    
    function getCounter() public view returns (uint256) {
        return counter;
    }

    function getBalance(address denom, address who) public view returns (uint256) {
        return BANK_CONTRACT.balanceOf(denom, who);
    }
    
    event Created(address maker);

    constructor() {
        emit Created(msg.sender);
    }
}

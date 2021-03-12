package sol

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhichSolc(t *testing.T) {
	assert := assert.New(t)
	path, err := WhichSolc()
	assert.NoError(err)
	if !assert.NotEmpty(path) {
		t.FailNow()
	}
}

func TestCompile(t *testing.T) {
	assert := assert.New(t)
	prepare(`// SPDX-License-Identifier: MIT
		pragma solidity >=0.6.0 <0.9.0;

contract Greeter  {
    string greeting;
    address public owner;

    function kill() public { if (msg.sender == owner) selfdestruct(payable(msg.sender)); }

    constructor(string memory _greeting) {
    	owner = msg.sender;
        greeting = _greeting;
    }

    function greet() public view returns (string memory) {
        return greeting;
    }
}`)
	defer cleanup()
	solcPath, err := WhichSolc()
	orPanic(err)

	c, err := NewSolCompiler(solcPath)
	orPanic(err)

	contracts, err := c.Compile("", "test.sol", 0)
	if !assert.NoError(err) {
		return
	}

	if !assert.Contains(contracts, "Greeter") {
		return
	}
	assert.NotEmpty(contracts["Greeter"].CompilerVersion)
	assert.NotEmpty(contracts["Greeter"].ABI)
	assert.NotEmpty(contracts["Greeter"].Bin)
	assert.Equal(contracts["Greeter"].Name, "Greeter")
	assert.Equal(contracts["Greeter"].SourcePath, "test.sol")

	contracts, err = c.CompileWithCoverage("", "test.sol")
	if !assert.NoError(err) {
		return
	}

	if !assert.Contains(contracts, "Greeter") {
		return
	}
	assert.NotEmpty(contracts["Greeter"].CompilerVersion)
	assert.NotEmpty(contracts["Greeter"].ABI)
	assert.NotEmpty(contracts["Greeter"].Bin)
	assert.Equal(contracts["Greeter"].Name, "Greeter")
	assert.Equal(contracts["Greeter"].SourcePath, "test.sol")
}

func cleanup() {
	os.Remove("test.sol")
}

func prepare(sol string) {
	err := ioutil.WriteFile("test.sol", []byte(sol), 0644)
	orPanic(err)
}

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

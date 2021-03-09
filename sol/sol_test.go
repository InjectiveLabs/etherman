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
	prepare(`pragma solidity >=0.6.0 <0.9.0;

contract Mortal {
    /* Define variable owner of the type address */
    address owner;

    /* This constructor is executed at initialization and sets the owner of the contract */
    constructor() { owner = msg.sender; }

    /* Function to recover the funds on the contract */
    function kill() public { if (msg.sender == owner) selfdestruct(payable(msg.sender)); }
}

contract Greeter is Mortal {
    /* Define variable greeting of the type string */
    string greeting;

    /* This runs when the contract is executed */
    constructor(string memory _greeting) {
        greeting = _greeting;
    }

    /* Main function */
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

	if !assert.Contains(contracts, "Mortal") {
		return
	}
	assert.NotEmpty(contracts["Mortal"].CompilerVersion)
	assert.NotEmpty(contracts["Mortal"].ABI)
	assert.NotEmpty(contracts["Mortal"].Bin)
	assert.Equal(contracts["Mortal"].Name, "Mortal")
	assert.Equal(contracts["Mortal"].SourcePath, "test.sol")

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

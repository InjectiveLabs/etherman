## evm-deploy-contract

A testing tool for contract deployment. Was used to debug Injective's EVM sidechain. Supports both Homestead and EIP155 modes of transaction signing.

### Usage

Install `solc` first, the executable will be located automatically.

```
$ evm-deploy-contract -h

Usage: evm-deploy-contract [OPTIONS]

Deploys arbitrary contract on an arbitrary EVM. Requires solc 0.6.x

Options:
      --solc-path   Set path solc executable. Found using 'which' otherwise (env $SOLC_PATH)
  -N, --name        Specify contract name to use. (env $SOL_CONTRACT_NAME) (default "Counter")
  -S, --source      Set path for .sol source file of the contract. (env $SOL_SOURCE_FILE) (default "contracts/Counter.sol")
  -E, --endpoint    Specify HTTP URI for EVM JSON-RPC endpoint (env $EVM_RPC_HTTP) (default "http://localhost:8545")
  -P, --privkey     Provide hex-encoded private key for tx signing. (env $TX_FROM_PRIVKEY)
      --signer      Override the default signer with other supported: homestead, eip155 (env $TX_SIGNER) (default "eip155")
  -G, --gas-price   Override estimated gas price with this option. (env $TX_GAS_PRICE) (default 50)
  -L, --gas-limit   Set the maximum gas for tx. (env $TX_GAS_LIMIT) (default 5000000)
```

### Example

```
$ evm-deploy-contract --signer homestead -E http://localhost:1317 -P 0x59F455CBF7B02A2C1F6B55B4D8D8FEF21BCD530457A9570999FB1C12C82F5201 -G 0
```

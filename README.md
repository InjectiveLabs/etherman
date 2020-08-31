## evm-deploy-contract

A testing tool for contract deployment. Was used to debug Injective's EVM sidechain. Supports both Homestead and EIP155 modes of transaction signing.

### Usage

Install `solc` first, the executable will be located automatically.

```
$ evm-deploy-contract --help

Usage: evm-deploy-contract [OPTIONS] COMMAND [arg...]

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
      --cache-dir   Set cache dir for build artifacts. (env $BUILD_CACHE_DIR) (default "build/")
      --no-cache    Disables build cache completely. (env $BUILD_DISABLE_CACHE)

Commands:
  deploy            Deploys given contract on the EVM chain. Caches build artefacts.
  tx                Creates a transaction for particular contract method. Uses build cache.

Run 'evm-deploy-contract COMMAND --help' for more information on a command.
```

### Deploying

```
$ evm-deploy-contract deploy --help

Usage: evm-deploy-contract deploy [--bytecode | --await] [ARGS...]

Deploys given contract on the EVM chain. Caches build artefacts.

Arguments:
  ARGS             Contract constructor's arguments. Will be ABI-encoded.

Options:
      --bytecode   Produce hex-encoded contract bytecode only. Do not interact with RPC.
      --await      Await transaction confirmation from the RPC. (default true)
```

**Example**

```
$ evm-deploy-contract --signer homestead -E http://localhost:1317 -P 59F455CBF7B02A2C1F6B55B4D8D8FEF21BCD530457A9570999FB1C12C82F5201 -G 0 deploy
```

```
$ evm-deploy-contract --source contracts/Counter.sol deploy --bytecode
```

### Method transact

```
$ evm-deploy-contract tx --help

Usage: evm-deploy-contract tx [--await] ADDRESS METHOD [ARGS...]

Creates a transaction for particular contract method. Uses build cache.

Arguments:
  ADDRESS       Contract address to interact with.
  METHOD        Contract method to transact.
  ARGS          Method transaction arguments. Will be ABI-encoded.

Options:
      --await   Await transaction confirmation from the RPC. (default true)
```

**Example**

```
$ evm-deploy-contract -E http://localhost:1317 -P 1F2FAB11FA77AE1110D9E9AF59191C656B8BA1093F1480F99486F635E38597CC -G 0 \
    tx 0x33832d3A5e359A0689088c832755461dDaD5d41B add
```

```
$ evm-deploy-contract -E http://localhost:1317 -P 1F2FAB11FA77AE1110D9E9AF59191C656B8BA1093F1480F99486F635E38597CC -G 0 \
    tx --await=false 0x33832d3A5e359A0689088c832755461dDaD5d41B addValue 10
```

### Read logs

```
$evm-deploy-contract logs --help

Usage: evm-deploy-contract logs ADDRESS TX_HASH EVENT_NAME

Loads logs of a particular event from contract.

Arguments:
  ADDRESS      Contract address to interact with.
  TX_HASH      Transaction hash to find receipt.
  EVENT_NAME   Contract event to find in the logs.
```

**Example**

```
evm-deploy-contract -E http://localhost:1317 logs 0x33832d3A5e359A0689088c832755461dDaD5d41B 0x8d2a06a2811cc4be16536c54e693ef1c268f8d04956fa0899e18372f6201fbe9 Increment
```

### License

[MIT](/LICENSE)

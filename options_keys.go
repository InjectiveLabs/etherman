package main

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/usbwallet"
	"github.com/ethereum/go-ethereum/common"
	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	cli "github.com/jawher/mow.cli"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/InjectiveLabs/evm-deploy-contract/keystore"
)

var (
	keystoreDir = app.String(cli.StringOpt{
		Name:   "keystore-dir",
		Desc:   "Specify Ethereum keystore dir (Geth or Clef) prefix.",
		EnvVar: "DEPLOYER_KEYSTORE_DIR",
	})

	from = app.String(cli.StringOpt{
		Name:   "F from",
		Desc:   "Specify the from address. If specified, must exist in keystore, ledger or match the privkey.",
		EnvVar: "DEPLOYER_FROM",
	})

	fromPassphrase = app.String(cli.StringOpt{
		Name:   "from-passphrase",
		Desc:   "Passphrase to unlock the private key from armor, if empty then stdin is used.",
		EnvVar: "DEPLOYER_FROM_PASSPHRASE",
	})

	fromPrivKey = app.String(cli.StringOpt{
		Name:   "P from-pk",
		Desc:   "Provide a raw Ethereum private key of the validator in hex.",
		EnvVar: "DEPLOYER_FROM_PK",
	})

	useLedger = app.Bool(cli.BoolOpt{
		Name:   "ledger",
		Desc:   "Use the Ethereum app on hardware ledger to sign transactions.",
		EnvVar: "DEPLOYER_USE_LEDGER",
		Value:  false,
	})
)

var emptyEthAddress = ethcmn.Address{}

func initEthereumAccountsManager(
	chainID uint64,
	keystoreDir *string,
	from *string,
	fromPassphrase *string,
	fromPrivKey *string,
	useLedger *bool,
) (
	fromAddress ethcmn.Address,
	signerFn bind.SignerFn,
	err error,
) {
	switch {
	case *useLedger:
		if from == nil {
			err := errors.New("cannot use Ledger without from address specified")
			return emptyEthAddress, nil, err
		}

		fromAddress = ethcmn.HexToAddress(*from)
		if fromAddress == (ethcmn.Address{}) {
			err = errors.Wrap(err, "failed to parse Ethereum from address")
			return emptyEthAddress, nil, err
		}

		ledgerBackend, err := usbwallet.NewLedgerHub()
		if err != nil {
			err = errors.Wrap(err, "failed to connect with Ethereum app on Ledger device")
			return emptyEthAddress, nil, err
		}

		signerFn = func(from common.Address, tx *ethtypes.Transaction) (*ethtypes.Transaction, error) {
			acc := accounts.Account{
				Address: from,
			}

			wallets := ledgerBackend.Wallets()
			for _, w := range wallets {
				if err := w.Open(""); err != nil {
					err = errors.Wrap(err, "failed to connect to wallet on Ledger device")
					return nil, err
				}

				if !w.Contains(acc) {
					if err := w.Close(); err != nil {
						err = errors.Wrap(err, "failed to disconnect the wallet on Ledger device")
						return nil, err
					}

					continue
				}

				tx, err = w.SignTx(acc, tx, new(big.Int).SetUint64(chainID))
				_ = w.Close()
				return tx, err
			}

			return nil, errors.Errorf("account %s not found on Ledger", from.String())
		}

		return fromAddress, signerFn, nil

	case len(*fromPrivKey) > 0:
		pkHex := strings.TrimPrefix(*fromPrivKey, "0x")
		ethPk, err := crypto.HexToECDSA(pkHex)
		if err != nil {
			err = errors.Wrap(err, "failed to hex-decode Ethereum ECDSA Private Key")
			return emptyEthAddress, nil, err
		}

		ethAddressFromPk := ethcrypto.PubkeyToAddress(ethPk.PublicKey)

		if len(*from) > 0 {
			addr := ethcmn.HexToAddress(*from)
			if addr == (ethcmn.Address{}) {
				err = errors.Wrap(err, "failed to parse Ethereum from address")
				return emptyEthAddress, nil, err
			} else if addr != ethAddressFromPk {
				err = errors.Wrap(err, "Ethereum from address does not match address from ECDSA Private Key")
				return emptyEthAddress, nil, err
			}
		}

		txOpts, err := bind.NewKeyedTransactorWithChainID(ethPk, new(big.Int).SetUint64(chainID))
		if err != nil {
			err = errors.New("failed to init NewKeyedTransactorWithChainID")
			return emptyEthAddress, nil, err
		}

		return txOpts.From, txOpts.Signer, nil

	case len(*keystoreDir) > 0:
		if from == nil {
			err := errors.New("cannot use Ethereum keystore without from address specified")
			return emptyEthAddress, nil, err
		}

		fromAddress = ethcmn.HexToAddress(*from)
		if fromAddress == (ethcmn.Address{}) {
			err = errors.Wrap(err, "failed to parse Ethereum from address")
			return emptyEthAddress, nil, err
		}

		if info, err := os.Stat(*keystoreDir); err != nil || !info.IsDir() {
			err = errors.New("failed to locate keystore dir")
			return emptyEthAddress, nil, err
		}

		ks, err := keystore.New(*keystoreDir)
		if err != nil {
			err = errors.Wrap(err, "failed to load keystore")
			return emptyEthAddress, nil, err
		}

		var pass string
		if len(*fromPassphrase) > 0 {
			pass = *fromPassphrase
		} else {
			pass, err = ethPassFromStdin()
			if err != nil {
				return emptyEthAddress, nil, err
			}
		}

		signerFn, err := ks.SignerFn(chainID, fromAddress, pass)
		if err != nil {
			err = errors.Wrapf(err, "failed to load key for %s", fromAddress)
			return emptyEthAddress, nil, err
		}

		return fromAddress, signerFn, nil

	default:
		err := errors.New("insufficient ethereum key details provided")
		return emptyEthAddress, nil, err
	}
}

func ethPassFromStdin() (string, error) {
	fmt.Print("Passphrase for Ethereum account: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		err := errors.Wrap(err, "failed to read password from stdin")
		return "", err
	}

	password := string(bytePassword)
	return strings.TrimSpace(password), nil
}

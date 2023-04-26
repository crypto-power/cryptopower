package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"hash"
	"math"
	"math/big"

	// sha3 "crypto/sha3"
	"fmt"

	// "path/filepath"

	"log"
	// "log"

	// "code.cryptopower.dev/group/cryptopower/libwallet/instantswap"

	// "github.com/decred/slog"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
	// hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
)

type wallet struct {
	*ethclient.Client
}

type account struct {
	privateKey      *ecdsa.PrivateKey
	privateKeyBytes []byte
	publicKey       *ecdsa.PublicKey
	address         string
	hash            hash.Hash
	hex             string
}

func createWallet() (*account, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	privateKeyBytes := crypto.FromECDSA(privateKey)
	// fmt.Println(hexutil.Encode(privateKeyBytes)[2:]) // 0xfad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		err := fmt.Errorf("error casting public key to ECDSA")
		return nil, err
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	// fmt.Println(hexutil.Encode(publicKeyBytes)[4:]) // 0x049a7df67f79246283fdc93af76d4f8cdd62c4886e8cd870944e817dd0b97934fdd7719d0810951e03418205868a5c1b40b192451367f28e0088dd75e15de40c05

	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	// fmt.Println(address) // 0x96216849c49358B10257cb55b28eA603c874b05E

	hash := sha3.NewLegacyKeccak256()
	hash.Write(publicKeyBytes[1:])
	// fmt.Println(hexutil.Encode(hash.Sum(nil)[12:]))

	acc := &account{
		privateKey:      privateKey,
		privateKeyBytes: privateKeyBytes,
		publicKey:       publicKeyECDSA,
		address:         address,
		hash:            hash,
		hex:             hexutil.Encode(hash.Sum(nil)[12:]),
	}

	return acc, nil

	// return hexutil.Encode(hash.Sum(nil)[12:]), nil
}

func (w *wallet) AccountBalance(walletHex string) (*big.Float, error) {
	account := common.HexToAddress(walletHex) // 0xC2A956Bc4C5447a30b900dD37Cce955F95895a73
	balance, err := w.Client.BalanceAt(context.Background(), account, nil)
	if err != nil {
		log.Fatal(err)

		return nil, err
	}

	fbalance := new(big.Float)
	fbalance.SetString(balance.String())
	ethValue := new(big.Float).Quo(fbalance, big.NewFloat(math.Pow10(18)))
	return ethValue, nil
}

// Get wallet address
func (w *wallet) GetWalletAddress(walletHex string) common.Address {
	return common.HexToAddress("0xe41d2489571d322189246dafa5ebde1f4699f498")
}

// get best block
func (w *wallet) GetBestBlockHeight() (uint64, error) {
	header, err := w.Client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	return header.Number.Uint64(), nil
}

// transfer ETH between accounts
func (w *wallet) TransferETH(fromHex string, toHex string, amount *big.Int) (string, error) {
	privateKey, err := crypto.HexToECDSA(fromHex)
	if err != nil {
		log.Fatal(err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		err := fmt.Errorf("error casting public key to ECDSA")
		return "", err
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := w.Client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return "", err
	}

	value := amount
	gasLimit := uint64(21000) // in units
	gasPrice, err := w.Client.SuggestGasPrice(context.Background())
	if err != nil {
		return "", err
	}

	toAddress := common.HexToAddress(toHex)
	var data []byte
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)

	chainID, err := w.Client.NetworkID(context.Background())
	if err != nil {
		return "", err
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return "", err
	}

	err = w.Client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", err
	}

	return signedTx.Hash().Hex(), nil
}

// PrivateKeyFromString converts a hex-encoded private key string to a *ecdsa.PrivateKey.
func PrivateKeyFromString(keyStr string) (*ecdsa.PrivateKey, error) {
	// Convert the hex string to bytes
	keyBytes, err := hex.DecodeString(keyStr[2:])
	if err != nil {
		return nil, err
	}

	// Parse the key bytes as a DER-encoded ECDSA private key
	keyDER, _ := pem.Decode(keyBytes)
	if keyDER == nil {
		return nil, err
	}

	// Parse the DER-encoded key as a *ecdsa.PrivateKey
	key, err := x509.ParseECPrivateKey(keyDER.Bytes)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// needs some upstream updates to get working
// func createHDWallet() {
// 	mnemonic := "tag volcano eight thank tide danger coast health above argue embrace heavy"
// 	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	path := hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/0")
// 	account, err := wallet.Derive(path, false)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	fmt.Println(account.Address.Hex()) // 0xC49926C4124cEe1cbA0Ea94Ea31a6c12318df947

// 	path = hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/1")
// 	account, err = wallet.Derive(path, false)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	fmt.Println(account.Address.Hex()) // 0x8230645aC28A4EdD1b0B53E7Cd8019744E9dD559
// }

func main() {

	// connect to the eth client, in this case we are using the local node
	client, err := ethclient.Dial("http://127.0.0.1:8545")
	if err != nil {
		fmt.Println("err: ", err)
	}
	fmt.Println("client: ", client)

	w := &wallet{client}

	// CREATE WALLET
	// wallet, err := createWallet()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("wallet: ", wallet)

	// print each field in a single line print statement
	// fmt.Printf("privateKey: %v \nprivateKeyBytes: %v \npublicKey: %v \naddress: %v \nhash: %v \nhex: %v \n", wallet.privateKey, wallet.privateKeyBytes, wallet.publicKey, wallet.address, wallet.hash, wallet.hex)

	balance, err := w.AccountBalance("0xdf5534e4567532089e2529095b4f554fd3bbba00")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("balance: ", balance)

	// get wallet address
	walletAddress := w.GetWalletAddress("0xdf5534e4567532089e2529095b4f554fd3bbba00")
	fmt.Println("walletAddress: ", walletAddress)

	// get best block
	bestBlock, err := w.GetBestBlockHeight()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("bestBlock: ", bestBlock)

	// from account hex is gotten from my local node
	amount := big.NewInt(1000000000000000000) // in wei (1 eth)

	signedTxHex, err := w.TransferETH("50050d5ca2af4f29ea8a83f517f24e89c0a28b4ca077882df0e64bf3211f576f", "0xdf5534e4567532089e2529095b4f554fd3bbba00", amount)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("signedTxHex: ", signedTxHex)

	// get balance after transfer
	balance, err = w.AccountBalance("0xdf5534e4567532089e2529095b4f554fd3bbba00")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("new balance after transfer: ", balance)
}

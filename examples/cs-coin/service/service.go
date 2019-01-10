package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	sbacapi "chainspace.io/prototype/sbac/api"
	"golang.org/x/crypto/ed25519"
)

const (
	ContractID     = "cs-coin"
	labelFmtString = "cs-coin:wallet:%v"
)

var (
	ErrInsufficientFunds      = errors.New("unsufficient funds in the wallet")
	ErrInvalidSignature       = errors.New("invalid signature")
	ErrInvalidSignatureFormat = errors.New("invalid signature format")
	ErrInvalidPubKeyFormat    = errors.New("invalid public key format")
)

type Service struct {
}

// Wallet the format of the wallet object stored in chainspace
type Wallet struct {
	Address string  `json:"address"`
	Balance float64 `json:"balance"`
	PubKey  string  `json:"pubKey"`
}

func New() *Service {
	return &Service{}
}

// Init create the seed object required to initialize the contract for a new wallet
func (s *Service) Init(addr string) string {
	return fmt.Sprintf(labelFmtString, addr)
}

// CreateWallet Create a new Wallet from an address
func (s *Service) CreateWallet(address, pubkey, initobjID string, mappings map[string]string,
) (*sbacapi.Transaction, error) {
	wallet := Wallet{
		Address: address,
		PubKey:  pubkey,
		Balance: 0,
	}

	walletBytes, err := json.Marshal(wallet)
	if err != nil {
		return nil, err
	}

	return &sbacapi.Transaction{
		Mappings: mappings,
		Traces: []sbacapi.Trace{
			{
				ContractID:            "cs-coin",
				Procedure:             "create-wallet",
				InputObjectVersionIDs: []string{initobjID},
				OutputObjects: []sbacapi.OutputObject{
					{
						Object: string(walletBytes),
						Labels: []string{fmt.Sprintf(labelFmtString, address)},
					},
				},
				Parameters: []string{address},
			},
		},
	}, nil
}

// AddFunds add new funds to a given wallet
func (s *Service) AddFunds(
	wallet Wallet, amount float64, sig string, mappings map[string]string, walletID string) (*sbacapi.Transaction, error) {
	// sig = base64 encoded
	sigbytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return nil, ErrInvalidSignatureFormat
	}

	pubkeybytes, err := base64.StdEncoding.DecodeString(wallet.PubKey)
	if err != nil {
		return nil, ErrInvalidPubKeyFormat
	}

	// check signature
	data := wallet.Address + strconv.FormatFloat(amount, 'f', 6, 64)
	if !ed25519.Verify(ed25519.PublicKey(pubkeybytes), []byte(data), sigbytes) {
		return nil, ErrInvalidSignature
	}

	newWallet := wallet
	newWallet.Balance += amount

	// marshal everything again
	newWalletBytes, err := json.Marshal(newWallet)
	if err != nil {
		return nil, err
	}

	return &sbacapi.Transaction{
		Mappings: mappings,
		Traces: []sbacapi.Trace{
			{
				ContractID:            ContractID,
				Procedure:             "add-funds",
				InputObjectVersionIDs: []string{walletID},
				OutputObjects: []sbacapi.OutputObject{
					{
						Object: string(newWalletBytes),
						Labels: []string{
							fmt.Sprintf(labelFmtString, wallet.Address)},
					},
				},
				Parameters: []string{strconv.FormatFloat(amount, 'f', 6, 64), sig},
			},
		},
	}, nil

}

//TransferFunds transfer money from a wallet to another
func (s *Service) TransferFunds(
	fromWallet, toWallet Wallet, amount float64, sig string, mappings map[string]string, fromWalletID, toWalletID string,
) (*sbacapi.Transaction, error) {
	// sig = base64 encoded
	sigbytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return nil, ErrInvalidSignatureFormat
	}

	pubkeybytes, err := base64.StdEncoding.DecodeString(fromWallet.PubKey)
	if err != nil {
		return nil, ErrInvalidPubKeyFormat
	}

	// check signature
	data := fromWallet.Address + toWallet.Address + strconv.FormatFloat(amount, 'f', 6, 64)
	if !ed25519.Verify(ed25519.PublicKey(pubkeybytes), []byte(data), sigbytes) {
		return nil, ErrInvalidSignature
	}

	// ensure first wallet have enough founds
	if fromWallet.Balance-amount < 0 {
		return nil, ErrInsufficientFunds
	}

	// update wallets
	newFromWallet := fromWallet
	newToWallet := toWallet
	newFromWallet.Balance -= amount
	newToWallet.Balance += amount

	// marshal everything again
	newFromWalletBytes, err := json.Marshal(newFromWallet)
	if err != nil {
		return nil, err
	}
	newToWalletBytes, err := json.Marshal(newToWallet)
	if err != nil {
		return nil, err
	}

	return &sbacapi.Transaction{
		Mappings: mappings,
		Traces: []sbacapi.Trace{
			{
				ContractID:            ContractID,
				Procedure:             "transfer-funds",
				InputObjectVersionIDs: []string{fromWalletID, toWalletID},
				OutputObjects: []sbacapi.OutputObject{
					{
						Object: string(newFromWalletBytes),
						Labels: []string{
							fmt.Sprintf(labelFmtString, fromWallet.Address)},
					},
					{
						Object: string(newToWalletBytes),
						Labels: []string{
							fmt.Sprintf(labelFmtString, toWallet.Address)},
					},
				},
				Parameters: []string{strconv.FormatFloat(amount, 'f', 6, 64), sig},
			},
		},
	}, nil
}

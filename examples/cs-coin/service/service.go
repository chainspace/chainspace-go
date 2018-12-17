package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

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
)

type Service struct {
}

// Wallet the format of the wallet object stored in chainspace
type Wallet struct {
	Address string `json:"address"`
	Balance uint   `json:"balance"`
	PubKey  string `json:"pubKey"`
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
	wallet Wallet, amount uint, sig string, mappings map[string]string, walletID string) (*sbacapi.Transaction, error) {
	// sig = base64 encoded
	sigbytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return nil, ErrInvalidSignatureFormat
	}

	// check signature
	data := wallet.Address + fmt.Sprintf("%v", amount)
	if !ed25519.Verify(ed25519.PublicKey(wallet.PubKey), []byte(data), sigbytes) {
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
				InputObjectVersionIDs: []string{walletID, walletID},
				OutputObjects: []sbacapi.OutputObject{
					{
						Object: string(newWalletBytes),
						Labels: []string{
							fmt.Sprintf(labelFmtString, wallet.Address)},
					},
				},
				Parameters: []string{fmt.Sprintf("%v", amount), sig},
			},
		},
	}, nil

}

//TransferFunds transfer money from a wallet to another
func (s *Service) TransferFunds(
	fromWallet, toWallet Wallet, amount uint, sig string, mappings map[string]string, fromWalletID, toWalletID string,
) (*sbacapi.Transaction, error) {
	// sig = base64 encoded
	sigbytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return nil, ErrInvalidSignatureFormat
	}

	// check signature
	data := fromWallet.Address + toWallet.Address + fmt.Sprintf("%v", amount)
	if !ed25519.Verify(ed25519.PublicKey(fromWallet.PubKey), []byte(data), sigbytes) {
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
				Parameters: []string{fmt.Sprintf("%v", amount), sig},
			},
		},
	}, nil
}

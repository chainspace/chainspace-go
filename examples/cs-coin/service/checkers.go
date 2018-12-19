package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"golang.org/x/crypto/ed25519"
)

// OutputObject ...
type OutputObject struct {
	Labels []string `json:"labels"`
	Object string   `json:"object"`
}

// CheckerTrace
type CheckerTrace struct {
	Dependencies    []CheckerTrace `json:"dependencies"`
	Inputs          []string       `json:"inputs"`
	Outputs         []OutputObject `json:"outputs"`
	Parameters      []string       `json:"parameters"`
	ReferenceInputs []string       `json:"referenceInputs"`
	Returns         []string       `json:"returns"`
}

func (s *Service) CreateWalletChecker(tr *CheckerTrace) bool {
	if len(tr.Inputs) != len(tr.Outputs) {
		return false
	}

	if len(tr.Inputs) != 1 {
		return false
	}

	if len(tr.Parameters) != 1 {
		return false
	}

	return true
}

func (s *Service) AddFundsChecker(tr *CheckerTrace) bool {
	if len(tr.Inputs) != len(tr.Outputs) {
		return false
	}

	if len(tr.Inputs) != 1 {
		return false
	}

	// expect 2 params, 0 = amount 1 = signature
	if len(tr.Parameters) != 2 {
		return false
	}

	// load old wallet
	oldWalletStr := tr.Inputs[0]
	oldWallet := Wallet{}
	err := json.Unmarshal([]byte(oldWalletStr), &oldWallet)
	if err != nil {
		return false
	}

	// load new wallet
	newWallet := Wallet{}
	err = json.Unmarshal([]byte(tr.Outputs[0].Object), &newWallet)
	if err != nil {
		return false
	}

	// load the amount of coin to add to the wallet
	amount, err := strconv.ParseUint(tr.Parameters[0], 10, 32)
	if err != nil {
		return false
	}

	// now ensure that oldWallet.Balance+amount = newWallet.Balance
	if oldWallet.Balance+uint(amount) != newWallet.Balance {
		return false
	}

	// finally check signature

	sigbytes, err := base64.StdEncoding.DecodeString(tr.Parameters[1])
	if err != nil {
		return false
	}

	// check signature
	data := oldWallet.Address + fmt.Sprintf("%v", amount)
	if !ed25519.Verify(ed25519.PublicKey(oldWallet.PubKey), []byte(data), sigbytes) {
		return false
	}

	return true
}

func (s *Service) TransferFundsChecker(tr *CheckerTrace) bool {
	if len(tr.Inputs) != len(tr.Outputs) {
		return false
	}

	if len(tr.Inputs) != 2 {
		return false
	}

	// expect 2 params, 0 = amount 1 = signature
	if len(tr.Parameters) != 2 {
		return false
	}

	// load from wallet
	fromWalletStr := tr.Inputs[0]
	fromWallet := Wallet{}
	err := json.Unmarshal([]byte(fromWalletStr), &fromWallet)
	if err != nil {
		return false
	}

	// load new wallet
	newFromWallet := Wallet{}
	err = json.Unmarshal([]byte(tr.Outputs[0].Object), &newFromWallet)
	if err != nil {
		return false
	}

	// load to wallet
	toWalletStr := tr.Inputs[1]
	toWallet := Wallet{}
	err = json.Unmarshal([]byte(toWalletStr), &toWallet)
	if err != nil {
		return false
	}

	// load new wallet
	newToWallet := Wallet{}
	err = json.Unmarshal([]byte(tr.Outputs[1].Object), &newToWallet)
	if err != nil {
		return false
	}

	// load the amount of coin to add to the wallet
	amount, err := strconv.ParseUint(tr.Parameters[0], 10, 32)
	if err != nil {
		return false
	}

	// ensure from wallet had enought funds
	if fromWallet.Balance-uint(amount) < 0 {
		return false
	}

	// now ensure that fromWallet.Balance-amount = newFromWallet.Balance
	if fromWallet.Balance-uint(amount) != newFromWallet.Balance {
		return false
	}

	// ensure newToWallet.Balance = toWallet.Balance+amount
	if toWallet.Balance+uint(amount) != newToWallet.Balance {
		return false
	}

	// finally check signature

	sigbytes, err := base64.StdEncoding.DecodeString(tr.Parameters[1])
	if err != nil {
		return false
	}

	// check signature
	data := fromWallet.Address + toWallet.Address + fmt.Sprintf("%v", amount)
	if !ed25519.Verify(ed25519.PublicKey(fromWallet.PubKey), []byte(data), sigbytes) {
		return false
	}

	return true

}

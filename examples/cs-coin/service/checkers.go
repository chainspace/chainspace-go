package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	sbacapi "chainspace.io/prototype/sbac/api"
	"golang.org/x/crypto/ed25519"
)

func (s *Service) CreateWalletChecker(tx *sbacapi.Transaction) bool {
	// check numbers of input / output object in the transaction
	if len(tx.Traces) != 1 {
		return false
	}
	tr := tx.Traces[0]
	if len(tr.InputObjectVersionIDs) != len(tr.OutputObjects) {
		return false
	}

	if len(tr.InputObjectVersionIDs) != 1 {
		return false
	}

	if len(tr.Parameters) != 1 {
		return false
	}

	return true
}

func (s *Service) AddFundsChecker(tx *sbacapi.Transaction) bool {
	// check numbers of input / output object in the transaction
	if len(tx.Traces) != 1 {
		return false
	}
	tr := tx.Traces[0]
	if len(tr.InputObjectVersionIDs) != len(tr.OutputObjects) {
		return false
	}

	if len(tr.InputObjectVersionIDs) != 1 {
		return false
	}

	// expect 2 params, 0 = amount 1 = signature
	if len(tr.Parameters) != 2 {
		return false
	}

	// load old wallet
	oldWalletStr, ok := tx.Mappings[tr.InputObjectVersionIDs[0]]
	if !ok {
		return false
	}
	oldWallet := Wallet{}
	err := json.Unmarshal([]byte(oldWalletStr), &oldWallet)
	if err != nil {
		return false
	}

	// load new wallet
	newWallet := Wallet{}
	err = json.Unmarshal([]byte(tr.OutputObjects[0].Object), &newWallet)
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

func (s *Service) TransferFundsChecker(tx *sbacapi.Transaction) bool {
	// check numbers of input / output object in the transaction
	if len(tx.Traces) != 1 {
		return false
	}
	tr := tx.Traces[0]
	if len(tr.InputObjectVersionIDs) != len(tr.OutputObjects) {
		return false
	}

	if len(tr.InputObjectVersionIDs) != 2 {
		return false
	}

	// expect 2 params, 0 = amount 1 = signature
	if len(tr.Parameters) != 2 {
		return false
	}

	// load from wallet
	fromWalletStr, ok := tx.Mappings[tr.InputObjectVersionIDs[0]]
	if !ok {
		return false
	}
	fromWallet := Wallet{}
	err := json.Unmarshal([]byte(fromWalletStr), &fromWallet)
	if err != nil {
		return false
	}

	// load new wallet
	newFromWallet := Wallet{}
	err = json.Unmarshal([]byte(tr.OutputObjects[0].Object), &newFromWallet)
	if err != nil {
		return false
	}

	// load to wallet
	toWalletStr, ok := tx.Mappings[tr.InputObjectVersionIDs[0]]
	if !ok {
		return false
	}
	toWallet := Wallet{}
	err = json.Unmarshal([]byte(toWalletStr), &toWallet)
	if err != nil {
		return false
	}

	// load new wallet
	newToWallet := Wallet{}
	err = json.Unmarshal([]byte(tr.OutputObjects[0].Object), &newToWallet)
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

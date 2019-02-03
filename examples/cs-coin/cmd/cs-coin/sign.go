package main

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ed25519"
)

func fileExists(path string) {
	fi, err := os.Stat(path)
	if err == nil {
		if fi.IsDir() {
			log.Fatalf("`%v` is not a file", path)
		}
		return
	}
	if os.IsNotExist(err) {
		log.Fatalf("could not find private key `%v`", path)
	}
	log.Fatalf("unexpected error, %v", err)

}

func signPayload(payload string, path string) {
	fp := filepath.Join(path, "key.priv")
	fileExists(fp)

	// load private key
	privfile, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Fatalf("unable to read private key, %v", err)
	}

	// private should be in base64 encoding in the file
	privkey, err := base64.StdEncoding.DecodeString(string(privfile))
	if err != nil {
		log.Fatalf("unable to load private key, invalid base64 value, %v", err)
	}

	// sign the payload then print it
	sig := ed25519.Sign(privkey, []byte(payload))
	b64sig := base64.StdEncoding.EncodeToString(sig)
	log.Printf("signature encoded to base64:")
	log.Printf("%v", b64sig)
}

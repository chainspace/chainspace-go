package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"chainspace.io/prototype/examples/cs-coin/api"
	"golang.org/x/crypto/ed25519"

	"github.com/gin-gonic/gin"
)

var (
	port       uint
	releaseMod bool
	genkeys    bool
	keysPath   string
	sign       string
)

func init() {
	flag.UintVar(&port, "port", 1789, "port the http server is listen to")
	flag.BoolVar(&releaseMod, "release-mod", false, "set the server into release-mod")
	flag.BoolVar(&genkeys, "genkeys", false, "generate signing keys")
	flag.StringVar(&keysPath, "keys-path", "", "output path of the signing keys")
	flag.StringVar(&sign, "sign", "", "a string to sign using the signing keys")
	flag.Parse()
}

func checkPath(path string) {
	fi, err := os.Stat(path)
	if err == nil {
		if !fi.IsDir() {
			log.Fatalf("`%v` is not a directory", path)
		}
		return
	}
	if os.IsNotExist(err) {
		log.Fatalf("directory `%v` does not exist", path)
	}
	log.Fatalf("unexpected error, %v", err)
}

func makeKeys(path string) {
	if len(path) > 0 {
		checkPath(path)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("unable to generate signing key, %v", err)
	}

	pubStr := base64.StdEncoding.EncodeToString(pub)
	err = ioutil.WriteFile(filepath.Join(path, "key.pub"), []byte(pubStr), 0644)
	if err != nil {
		log.Fatalf("unable to write public key, %v", err)
	}

	privStr := base64.StdEncoding.EncodeToString(priv)
	err = ioutil.WriteFile(filepath.Join(path, "key.priv"), []byte(privStr), 0644)
	if err != nil {
		log.Fatalf("unable to write private key, %v", err)
	}

	log.Printf("signing keys generated successfuly into `%v`", path)
}

func main() {
	if genkeys && len(sign) > 0 {
		log.Fatalf("cannot invoke genkeys and sign command at the same time")
	}
	if genkeys {
		makeKeys(keysPath)
		return
	}
	if len(sign) > 0 {
		signPayload(sign, keysPath)
		return
	}

	if releaseMod {
		gin.SetMode(gin.ReleaseMode)
	}
	router := api.New()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	wg := &sync.WaitGroup{}

	srv := &http.Server{
		Addr:    ":" + fmt.Sprintf("%d", port),
		Handler: router,
	}

	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		log.Printf("http server started on port %v", port)
		log.Printf("http server exited: %v", srv.ListenAndServe())
	}(wg)

	// for an exit signal
	_ = <-sigc

	// close our server
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("error on http server shutdown: %v", err)
	}

	// wait for the server to close properly
	wg.Wait()
}

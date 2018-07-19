package restsrv // import "chainspace.io/prototype/restsrv"

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/transactor/client"

	"github.com/rs/cors"
	"go.uber.org/zap"
)

type Config struct {
	Addr       string
	Port       int
	Top        *network.Topology
	MaxPayload config.ByteSize
}

type Service struct {
	port     int
	srv      *http.Server
	txclient transactorclient.Client
}

type resp struct {
	Data   interface{} `json:"data"`
	Status string      `json:"status"`
}

func response(rw http.ResponseWriter, status int, resp resp) {
	rw.Header().Add("Content-Type", "application/json")
	rw.WriteHeader(status)
	b, _ := json.Marshal(resp)
	rw.Write(b)
}

func fail(rw http.ResponseWriter, status int, data interface{}) {
	response(rw, status, resp{data, "fail"})
}

func error(rw http.ResponseWriter, status int, data interface{}) {
	response(rw, status, resp{data, "error"})
}

func success(rw http.ResponseWriter, status int, data interface{}) {
	response(rw, status, resp{data, "success"})
}

func (s *Service) object(rw http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.createObject(rw, r)
		return
	case http.MethodDelete:
		s.deleteObject(rw, r)
		return
	case http.MethodGet:
		s.queryObject(rw, r)
		return
	default:
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
}

func readKey(rw http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return nil, false
	}
	req := struct {
		Key string `json:"key"`
	}{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return nil, false
	}
	if len(req.Key) <= 0 {
		fail(rw, http.StatusBadRequest, "empty key")
		return nil, false
	}
	key, err := base64.StdEncoding.DecodeString(req.Key)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to b64decode: %v", err))
		return nil, false
	}
	return key, true
}

func (s *Service) queryObject(rw http.ResponseWriter, r *http.Request) {
	key, ok := readKey(rw, r)
	if !ok {
		return
	}
	_ = key
	success(rw, http.StatusOK, "query object")
}

func (s *Service) createObject(rw http.ResponseWriter, r *http.Request) {
	success(rw, http.StatusOK, "create object")
}

func (s *Service) deleteObject(rw http.ResponseWriter, r *http.Request) {
	key, ok := readKey(rw, r)
	if !ok {
		return
	}
	_ = key
	success(rw, http.StatusOK, "delete object")
}

func (s *Service) transaction(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
	}
	success(rw, http.StatusOK, "lol")
}

func (s *Service) makeServ(addr string, port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/object", s.object)
	mux.HandleFunc("/transaction", s.transaction)
	handler := cors.Default().Handler(mux)
	return &http.Server{
		Addr:         fmt.Sprintf("%v:%v", addr, port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

func New(cfg *Config) *Service {
	s := &Service{
		port:     cfg.Port,
		txclient: transactorclient.New(&transactorclient.Config{Top: cfg.Top, MaxPayload: cfg.MaxPayload}),
	}
	s.srv = s.makeServ(cfg.Addr, cfg.Port)
	go func() {
		log.Info("http server started", zap.Int("port", cfg.Port))
		log.Fatal("http server exited", zap.Error(s.srv.ListenAndServe()))
	}()
	return s
}

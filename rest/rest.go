package rest

import (
	"fmt"

	"chainspace.io/prototype/checker"
	checkerapi "chainspace.io/prototype/checker/api"
	checkerclient "chainspace.io/prototype/checker/client"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/kv"
	kvapi "chainspace.io/prototype/kv/api"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/sbac"
	sbacapi "chainspace.io/prototype/sbac/api"
	sbacclient "chainspace.io/prototype/sbac/client"

	"github.com/gin-gonic/gin"
)

// Config defines the values passed into the REST Service
type Config struct {
	Addr        string
	Checker     *checker.Service
	CheckerOnly bool
	Key         signature.KeyPair
	Port        int
	Top         *network.Topology
	MaxPayload  config.ByteSize
	SelfID      uint64
	Store       kv.Service
	SBAC        *sbac.Service
	SBACOnly    bool
}

// Service gives us a place to store values for our REST API
type Service struct {
	port        int
	router      *gin.Engine
	store       kv.Service
	top         *network.Topology
	maxPayload  config.ByteSize
	sbac        *sbac.Service
	client      sbacclient.Client
	checker     *checkerclient.Client
	checkerOnly bool
	sbacOnly    bool
	controllers []controller
}

// New returns a new REST API
func New(cfg *Config) *Service {
	var controllers []controller
	var txclient sbacclient.Client
	var checkrclt *checkerclient.Client

	if !cfg.CheckerOnly {
		clcfg := sbacclient.Config{
			NodeID:     cfg.SelfID,
			Top:        cfg.Top,
			MaxPayload: cfg.MaxPayload,
			Key:        cfg.Key,
		}

		checkrcfg := checkerclient.Config{
			NodeID:     cfg.SelfID,
			Top:        cfg.Top,
			MaxPayload: cfg.MaxPayload,
			Key:        cfg.Key,
		}
		checkrclt := checkerclient.New(&checkrcfg)

		txclient := sbacclient.New(&clcfg)
		sbacCfg := sbacapi.Config{
			Sbac:       cfg.SBAC,
			Checkerclt: checkrclt,
			ShardID:    0,
			NodeID:     cfg.SelfID,
			Checker:    cfg.Checker,
			Top:        cfg.Top,
			Sbacclt:    txclient,
		}
		controllers = append(controllers, kvapi.New(cfg.Store, cfg.SBAC))
		controllers = append(controllers, sbacapi.New(&sbacCfg))
	}

	if !cfg.SBACOnly {
		controllers = append(
			controllers, checkerapi.New(cfg.Checker, cfg.SelfID))
	}

	s := &Service{
		port:        cfg.Port,
		top:         cfg.Top,
		maxPayload:  cfg.MaxPayload,
		client:      txclient,
		store:       cfg.Store,
		sbac:        cfg.SBAC,
		checker:     checkrclt,
		controllers: controllers,
	}
	s.router = s.makeRouter(controllers...)

	go func() {
		log.Info("http server started", fld.Port(cfg.Port))
		log.Fatal("http server exited", fld.Err(s.router.Run(":"+fmt.Sprintf("%d", cfg.Port))))
	}()
	return s
}

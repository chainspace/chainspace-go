package rest

import (
	"fmt"

	"chainspace.io/prototype/checker"
	checkerApi "chainspace.io/prototype/checker/api"
	checkerClient "chainspace.io/prototype/checker/client"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/kv"
	kvApi "chainspace.io/prototype/kv/api"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/sbac"
	sbacApi "chainspace.io/prototype/sbac/api"
	sbacClient "chainspace.io/prototype/sbac/client"

	"github.com/gin-gonic/gin"
)

// Config defines the values passed into the REST Service
type Config struct {
	Addr        string
	Checker     *checker.Service
	CheckerOnly bool
	Key         signature.KeyPair
	MaxPayload  config.ByteSize
	Port        int
	SBAC        *sbac.ServiceSBAC
	SBACOnly    bool
	SelfID      uint64
	Store       kv.Service
	Top         *network.Topology
}

// Service gives us a place to store values for our REST API
type Service struct {
	checker     *checkerClient.Client
	checkerOnly bool
	client      sbacClient.Client
	controllers []controller
	maxPayload  config.ByteSize
	port        int
	router      *gin.Engine
	sbac        *sbac.ServiceSBAC
	sbacOnly    bool
	store       kv.Service
	top         *network.Topology
}

// New returns a new REST API
func New(cfg *Config) *Service {
	var controllers []controller
	var txClient sbacClient.Client
	var checkrclt *checkerClient.Client

	if !cfg.CheckerOnly {
		clcfg := sbacClient.Config{
			NodeID:     cfg.SelfID,
			Top:        cfg.Top,
			MaxPayload: cfg.MaxPayload,
			Key:        cfg.Key,
		}

		checkrcfg := checkerClient.Config{
			NodeID:     cfg.SelfID,
			Top:        cfg.Top,
			MaxPayload: cfg.MaxPayload,
			Key:        cfg.Key,
		}
		checkrclt = checkerClient.New(&checkrcfg)
		txClient = sbacClient.New(&clcfg)

		sbacCfg := sbacApi.Config{
			Checker:    cfg.Checker,
			Checkerclt: checkrclt,
			NodeID:     cfg.SelfID,
			Sbac:       cfg.SBAC,
			Sbacclt:    txClient,
			ShardID:    0,
			Top:        cfg.Top,
		}
		controllers = append(controllers, kvApi.NewController(cfg.Store, cfg.SBAC))
		controllers = append(controllers, sbacApi.NewController(&sbacCfg))
	}

	if !cfg.SBACOnly {
		controllers = append(controllers, checkerApi.NewController(cfg.Checker, cfg.SelfID))
	}

	s := &Service{
		checker:     checkrclt,
		client:      txClient,
		controllers: controllers,
		maxPayload:  cfg.MaxPayload,
		port:        cfg.Port,
		sbac:        cfg.SBAC,
		store:       cfg.Store,
		top:         cfg.Top,
	}
	s.router = s.makeRouter(controllers...)

	go func() {
		log.Info("http server started", fld.Port(cfg.Port))
		log.Fatal("http server exited", fld.Err(s.router.Run(":"+fmt.Sprintf("%d", cfg.Port))))
	}()
	return s
}

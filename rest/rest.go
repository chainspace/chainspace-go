package rest

import (
	"context"
	"fmt"

	"chainspace.io/chainspace-go/checker"
	checkerApi "chainspace.io/chainspace-go/checker/api"
	checkerClient "chainspace.io/chainspace-go/checker/client"
	"chainspace.io/chainspace-go/config"
	"chainspace.io/chainspace-go/internal/crypto/signature"
	"chainspace.io/chainspace-go/internal/log"
	"chainspace.io/chainspace-go/internal/log/fld"
	"chainspace.io/chainspace-go/kv"
	kvApi "chainspace.io/chainspace-go/kv/api"
	"chainspace.io/chainspace-go/network"
	"chainspace.io/chainspace-go/pubsub"
	pubsubApi "chainspace.io/chainspace-go/pubsub/api"
	"chainspace.io/chainspace-go/sbac"
	sbacApi "chainspace.io/chainspace-go/sbac/api"
	sbacClient "chainspace.io/chainspace-go/sbac/client"

	"github.com/gin-gonic/gin"
)

// Config defines the values passed into the REST Service
type Config struct {
	Addr        string
	Checker     checker.Service
	CheckerOnly bool
	Key         signature.KeyPair
	MaxPayload  config.ByteSize
	Port        int
	SBAC        *sbac.ServiceSBAC
	SBACOnly    bool
	SelfID      uint64
	Store       kv.Service
	Top         *network.Topology
	PS          pubsub.Server
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
	ps          pubsub.Server
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
		controllers = append(controllers, kvApi.New(cfg.Store, cfg.SBAC))
		controllers = append(controllers, sbacApi.New(&sbacCfg))

		// if pubsub is enabled add the pubsub api
		if cfg.PS != nil {
			controllers = append(controllers,
				pubsubApi.New(context.Background(), cfg.PS))
		}
	}

	if !cfg.SBACOnly {
		controllers = append(controllers, checkerApi.New(cfg.Checker, cfg.SelfID))
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

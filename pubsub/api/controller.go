package api

import (
	"context"
	"net/http"

	"chainspace.io/chainspace-go/internal/log"
	"chainspace.io/chainspace-go/internal/log/fld"
	"chainspace.io/chainspace-go/pubsub"
	"github.com/gin-gonic/gin"
)

type controller struct {
	ctx        context.Context
	service    Service
	wsupgrader WSUpgrader
}

// Controller is the websocket pub-sub controller
type Controller interface {
	RegisterRoutes(router *gin.Engine)
}

// NewWithOptions create a new controller with options
func NewWithServiceAndWSUpgrader(ctx context.Context, service Service, upgrader WSUpgrader) Controller {
	return &controller{
		ctx:        ctx,
		service:    service,
		wsupgrader: upgrader,
	}
}

func New(ctx context.Context, pbService pubsub.Server) Controller {
	return &controller{
		service:    NewService(ctx, pbService),
		wsupgrader: upgrader{},
	}
}

func (ctlr *controller) RegisterRoutes(router *gin.Engine) {
	router.GET("/api/pubsub/ws", ctlr.websocket)
}

// websocket Initiate a websocket connection
// @Summary Initiate a websocket connection in order to subscribed to object saved in chainspace by the current node
// @Description Iniate a websocket connection
// @ID websocket
// @Tags pubsub
// @Success 204
// @Failure 500 {object} api.Error
// @Router /api/pubsub/ws [get]
func (ctlr *controller) websocket(c *gin.Context) {
	wc, err := ctlr.wsupgrader.Upgrade(c.Writer, c.Request)
	if err != nil {
		log.Error("unable to upgrade ws connection", fld.Err(err))
		c.JSON(http.StatusInternalServerError, Error{err.Error()})
		return
	}

	if status, err := ctlr.service.Websocket(c.Request.RemoteAddr, wc); err != nil {
		log.Error("unexpected closed websocket connection", fld.Err(err))
		c.JSON(status, Error{err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

package api

import (
	"net/http"

	checkerclient "chainspace.io/prototype/checker/client"
	"chainspace.io/prototype/network"
	sbacclient "chainspace.io/prototype/sbac/client"

	"chainspace.io/prototype/checker"
	"chainspace.io/prototype/sbac"
	"github.com/gin-gonic/gin"
)

// Config ...
type Config struct {
	Sbac       *sbac.Service
	Checkerclt *checkerclient.Client
	Sbacclt    sbacclient.Client
	ShardID    uint64
	NodeID     uint64
	Checker    *checker.Service
	Top        *network.Topology
}

// Controller is the Key-Value controller
type controller struct {
	service *service
}

// Controller is the Key-Value controller
type Controller interface {
	AddTransaction(c *gin.Context)
	RegisterRoutes(router *gin.Engine)
}

// New returns a new kv.Controller
func New(config *Config) Controller {
	return &controller{newService(config)}
}

func (controller *controller) RegisterRoutes(router *gin.Engine) {
	router.POST("/api/sbac/object", controller.CreateObject)
	router.POST("/api/sbac/transaction", controller.AddTransaction)
	router.POST("/api/sbac/transaction-checked", controller.AddCheckedTransaction)
	router.GET("/api/sbac/states", controller.States)
}

// CreateObject does just what it says
// @Summary Create an object to a shard
// @Description TODO
// @ID createObject
// @Accept  json
// @Produce  json
// @Tags sbac
// @Param   object      body   api.ObjectRequest     true  "object"
// @Success 200 {object} api.ObjectIDResponse
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/sbac/object [post]
func (controller *controller) CreateObject(c *gin.Context) {
	req := ObjectRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error{err.Error()})
		return
	}

	id, status, err := controller.service.CreateObject(req.Object)
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, ObjectIDResponse{ID: id})
}

// AddTransaction does just what it says
// @Summary Add a transaction to a shard
// @Description TODO
// @ID addTransaction
// @Accept  json
// @Produce  json
// @Tags sbac
// @Param   transaction      body   api.Transaction     true  "transaction"
// @Success 200 {object} api.Object
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/sbac/transaction [post]
func (controller *controller) AddTransaction(c *gin.Context) {
	tx := Transaction{}
	if err := c.BindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, Error{err.Error()})
		return
	}

	obj, status, err := controller.service.Add(c.Request.Context(), &tx)
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, ObjectResponse{Object: obj})
}

// AddCheckedTransaction does just what it says
// @Summary Add a transaction which is already checked to a shard
// @Description TODO
// @ID addTransaction
// @Accept  json
// @Produce  json
// @Tags sbac
// @Param   transaction      body   api.Transaction     true  "transaction"
// @Success 200 {object} api.Object
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/sbac/transaction-checked [post]
func (controller *controller) AddCheckedTransaction(c *gin.Context) {
	tx := Transaction{}
	if err := c.BindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, Error{err.Error()})
		return
	}

	obj, status, err := controller.service.AddChecked(c.Request.Context(), &tx)
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, ObjectResponse{Object: obj})
}

// States get states
// @Summary Get the states
// @Description TODO
// @ID states
// @Accept  json
// @Produce  json
// @Tags sbac
// @Success 200 {object} api.Object
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/sbac/states [get]
func (controller *controller) States(c *gin.Context) {
	states, status, err := controller.service.States(c.Request.Context())
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, states)
}

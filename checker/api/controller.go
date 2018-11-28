package api

import (
	"net/http"

	"chainspace.io/prototype/checker"

	"github.com/gin-gonic/gin"
)

// Controller is the Key-Value controller
type controller struct {
	service *service
}

// Controller is the Key-Value controller
type Controller interface {
	Check(c *gin.Context)
	RegisterRoutes(router *gin.Engine)
}

// New returns a new kv.Controller
func New(checkr *checker.Service, nodeID uint64) Controller {
	return &controller{newService(checkr, nodeID)}
}

func (controller *controller) RegisterRoutes(router *gin.Engine) {
	router.POST("/api/checker/check", controller.Check)
}

// Check Checks something
// @Summary Checks something
// @Description Checks something
// @ID check
// @Accept  json
// @Produce  json
// @Tags checker
// @Param   transaction      body   api.Transaction     true  "transaction"
// @Success 200 {object} api.CheckTransactionResponse
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/checker/check [post]
func (controller *controller) Check(c *gin.Context) {
	tx := Transaction{}
	if err := c.BindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, Error{err.Error()})
		return
	}

	checkedRes, status, err := controller.service.Check(c.Request.Context(), &tx)
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, checkedRes)
}

package api

import (
	"net/http"

	"chainspace.io/prototype/kv"
	"chainspace.io/prototype/sbac"
	"github.com/gin-gonic/gin"
)

// Controller is the Key-Value controller
type controller struct {
	service Service
}

// Controller is the Key-Value controller
type Controller interface {
	// GetByLabel(c *gin.Context)
	RegisterRoutes(router *gin.Engine)
}

// NewWithService returns a new kv.Controller
func NewWithService(service Service) Controller {
	return &controller{service}
}

// New returns a new kv.Controller
func New(kvService kv.Service, sbacService sbac.Service) Controller {
	return &controller{NewService(kvService, sbacService)}
}

func (controller *controller) RegisterRoutes(router *gin.Engine) {
	router.GET("/api/kv/label/:label", controller.GetByLabel)
	router.GET("/api/kv/label/:label/version-id", controller.GetVersionIDByLabel)
	router.GET("/api/kv/prefix/:prefix", controller.GetByPrefix)
	router.GET("/api/kv/prefix/:prefix/version-id", controller.GetVersionIDByPrefix)
}

// GetByLabel Retrieves a key by its value
// @Summary Retrieve a key by its label
// @Description get string by label
// @ID getbyLabel
// @Accept  json
// @Produce  json
// @Tags kv
// @Param   label      path   string     true  "label"
// @Success 200 {object} api.ObjectResponse
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/kv/label/{label} [get]
func (controller *controller) GetByLabel(c *gin.Context) {
	label := c.Param("label")
	obj, status, err := controller.service.GetByLabel(label)
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, ObjectResponse{Object: obj})
}

// GetByPrefix Retrieves objects matching a key prefix
// @Summary See before
// @Description See before
// @ID getByPrefix
// @Accept json
// @Produce json
// @Tags kv
// @Param prefix path string true "prefix"
// @Success 200 {object} api.ListObjectsResponse
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/kv/prefix/{prefix} [get]
func (controller *controller) GetByPrefix(c *gin.Context) {
	prefix := c.Param("prefix")
	ls, status, err := controller.service.GetByPrefix(prefix)
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, ListObjectsResponse{Pairs: ls})
}

// GetVersionIDByLabel Retrieves a version ID by its value
// @Summary Retrieve a version ID by its label
// @Description get version ID by label
// @ID getVersionIDbyLabel
// @Accept  json
// @Produce  json
// @Tags kv
// @Param   label      path   string     true  "label"
// @Success 200 {object} api.VersionIDResponse
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/kv/label/{label}/version-id [get]
func (controller *controller) GetVersionIDByLabel(c *gin.Context) {
	label := c.Param("label")
	versionID, status, err := controller.service.GetVersionID(label)
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, VersionIDResponse{VersionID: versionID})
}

// GetVersionIDByPrefix Retrieves version IDs matching a prefix
// @Summary See before
// @Description see before
// @ID getVersionIDbyPrefix
// @Accept json
// @Produce json
// @Tags kv
// @Param prefix path string true "prefix"
// @Success 200 {object} api.ListVersionIDsResponse
// @Failure 400 {object} api.Error
// @Failure 404 {object} api.Error
// @Failure 500 {object} api.Error
// @Router /api/kv/prefix/{prefix}/version-id [get]
func (controller *controller) GetVersionIDByPrefix(c *gin.Context) {
	prefix := c.Param("prefix")
	versionIDs, status, err := controller.service.GetVersionIDByPrefix(prefix)
	if err != nil {
		c.JSON(status, Error{err.Error()})
		return
	}
	c.JSON(http.StatusOK, ListVersionIDsResponse{Pairs: versionIDs})
}

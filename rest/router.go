package rest

import (
	_ "chainspace.io/prototype/rest/docs" // needed by https://github.com/swaggo/gin-swagger

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
)

type controller interface {
	RegisterRoutes(*gin.Engine)
}

// @title Chainspace API
// @version 1.0
// @description Chainspace REST API endpoints

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

func (s *Service) makeRouter(controllers ...controller) *gin.Engine {
	// Set the router as the default one shipped with Gin
	router := gin.Default()
	// Add cors
	router.Use(cors.Default())

	// Serve Swagger frontend static files using gin-swagger middleware
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	for _, v := range controllers {
		v.RegisterRoutes(router)
	}

	return router
}

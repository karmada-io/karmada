package app

import "github.com/gin-gonic/gin"

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	api := r.Group("/api")
	v1 := api.Group("/v1")
	{
		//Join member cluster into karmada controller panel
		v1.POST("/join", Join)
	}
	return r
}

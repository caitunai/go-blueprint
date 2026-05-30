package route

import (
	"github.com/caitunai/go-blueprint/api/base"
	"github.com/caitunai/go-blueprint/api/handler"
)

func InitRoute(r *base.Router) {
	InitMiddleware()
	r.Use(AttemptAuth())

	initPackageHandler(r)
	initAPIHandler(r)
}

func initPackageHandler(r *base.Router) {
	r.GET("/", handler.HomePage)

	r.GET("/assets/*filepath", handler.ServeAssetFile)
	r.HEAD("/assets/*filepath", handler.ServeAssetFile)
	r.NoRoute(handler.ServeRootStaticFiles)
}

func initAPIHandler(r *base.Router) {
	api := r.Group("/api", apiAuthorized)
	api.GET("/", handler.APIHomePage)

	// Add CRUD services for db.User
	handler.UserControl(api.Group("/users"))
}

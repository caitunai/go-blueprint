package route

import (
	"net/http"

	"github.com/caitunai/go-blueprint/api/base"
	"github.com/caitunai/go-blueprint/api/handler"
	"github.com/caitunai/go-blueprint/storage"
)

func InitRoute(r *base.Router) {
	InitMiddleware()
	r.Use(AttemptAuth())

	initPackageHandler(r)
	initAPIHandler(r)
}

func initPackageHandler(r *base.Router) {
	r.GET("/", handler.HomePage)

	// Wechat domain validation
	r.StaticFileFS("/W98wUxrfSS.txt", "/W98wUxrfSS.txt", http.FS(storage.Static))
}

func initAPIHandler(r *base.Router) {
	api := r.Group("/api", apiAuthorized)
	api.GET("/", handler.APIHomePage)

	// Add CRUD services for db.User
	handler.UserControl(api.Group("/users"))
}

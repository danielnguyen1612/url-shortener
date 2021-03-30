package server

import (
	"net/http"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/danielnguyentb/url-shortener/core"
	"github.com/danielnguyentb/url-shortener/libs"
	"github.com/danielnguyentb/url-shortener/libs/render"
	"github.com/danielnguyentb/url-shortener/server/controllers"
)

func AddRoutes(r *core.Mux, log *zap.Logger) error {
	// Init gorm with mysql
	db, err := libs.NewMysqlWithViper(log)
	if err != nil {
		return errors.Wrap(err, "libs.NewMysqlWithViper")
	}

	// Init redis
	redis, err := libs.NewRedisFromViper(log)
	if err != nil {
		return errors.Wrap(err, "libs.NewRedisFromViper")
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]string{
			"status": "ok",
		})
	})

	urlCtrl, err := controllers.NewUrlController(log, redis, db)
	if err != nil {
		return errors.Wrap(err, "controllers.NewUrlController")
	}
	r.Post("/create", urlCtrl.CreateShorten)
	r.Get("/r/:code", urlCtrl.Redirect)

	adminCtrl, err := controllers.NewAdminController(log, redis, db)
	if err != nil {
		return errors.Wrap(err, "controllers.NewAdminController")
	}
	r.Get("/admin/list", adminCtrl.GetList)

	return nil
}

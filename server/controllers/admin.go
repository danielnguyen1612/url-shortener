package controllers

import (
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/danielnguyentb/url-shortener/libs"
	"github.com/danielnguyentb/url-shortener/libs/render"
	"github.com/danielnguyentb/url-shortener/server/models"
)

var (
	ErrNonAuth = errors.New("Non Auth")
)

type response struct {
	Success bool
	Items   []models.Url
}

type Admin struct {
	model *models.UrlModel
}

func (a *Admin) GetList(w http.ResponseWriter, r *http.Request) {
	log, err := a.parseRequestAndValidate(w, r)
	if err != nil {
		log.With(zap.Error(err)).Error("invalid request")
		return
	}

	var shortCode, keywords string
	queries := r.URL.Query()
	if len(queries.Get("code")) != 0 {
		shortCode = queries.Get("code")
	}

	if len(queries.Get("term")) != 0 {
		keywords = queries.Get("term")
	}

	items, err := a.model.GetList(shortCode, keywords)
	if err != nil {
		log.With(zap.Error(err)).Error("failed to get list by criteria")
		render.Status(r, http.StatusBadGateway)
		render.JSON(w, r, &response{
			Success: false,
		})
	}

	render.JSON(w, r, &response{
		Success: true,
		Items:   items,
	})
}

func (a *Admin) parseRequestAndValidate(w http.ResponseWriter, r *http.Request) (log *zap.Logger, err error) {
	log = libs.GetLogEntry(r)

	authHeader := r.Header.Get("Authorization")
	if len(authHeader) == 0 {
		render.Status(r, http.StatusForbidden)
		render.NoContent(w, r)
		err = ErrNonAuth
		return
	}

	return
}

func NewAdminController(log *zap.Logger, client *redis.Client, db *gorm.DB) (*Admin, error) {
	model, err := models.NewUrlModel(log, client, db)
	if err != nil {
		return nil, errors.Wrap(err, "NewUrlController")
	}

	return &Admin{
		model: model,
	}, nil
}

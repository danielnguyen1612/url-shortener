package controllers

import (
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/danielnguyentb/url-shortener/core"
	"github.com/danielnguyentb/url-shortener/libs"
	"github.com/danielnguyentb/url-shortener/libs/render"
	"github.com/danielnguyentb/url-shortener/server/models"
)

var (
	ErrNonAuth = errors.New("Non Auth")
)

const (
	keyAdmin           = "adminKey"
	keyAuthorizeHeader = "Authorization"
)

type AdminResponse struct {
	Success bool         `json:"success"`
	Items   []models.Url `json:"items"`
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
		render.JSON(w, r, &AdminResponse{
			Success: false,
		})
	}

	render.JSON(w, r, &AdminResponse{
		Success: true,
		Items:   items,
	})
}

func (a *Admin) Delete(w http.ResponseWriter, r *http.Request) {
	log, err := a.parseRequestAndValidate(w, r)
	if err != nil {
		log.With(zap.Error(err)).Error("invalid request")
		return
	}

	shortenCode := core.RouteContext(r.Context()).RouteParams.Get("code")

	if len(shortenCode) == 0 {
		render.Status(r, http.StatusNotFound)
		return
	}

	log = log.With(zap.String("code", shortenCode))

	// Find shorten item
	item, err := a.model.FindByShortCode(shortenCode, true)
	if err != nil && !errors.Is(err, models.ErrExpired) {
		if errors.Is(err, models.ErrNotFound) {
			render.Status(r, http.StatusNotFound)
			render.NoContent(w, r)
			return
		}

		log.With(zap.Error(err)).Error("fail to look up url with short code")
		render.Status(r, http.StatusBadRequest)
		render.NoContent(w, r)
		return
	}

	// Soft delete item
	if ok, err := a.model.Delete(item.Key); err != nil || !ok {
		render.Status(r, http.StatusBadRequest)
	}

	render.NoContent(w, r)
	return
}

func (a *Admin) parseRequestAndValidate(w http.ResponseWriter, r *http.Request) (log *zap.Logger, err error) {
	log = libs.GetLogEntry(r)

	authHeader := r.Header.Get(keyAuthorizeHeader)
	if len(authHeader) == 0 || authHeader != viper.GetString(keyAdmin) {
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

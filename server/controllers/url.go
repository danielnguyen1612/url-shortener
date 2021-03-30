package controllers

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
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

const (
	errorOccurred = "Error Occurred"
	keyBlacklist  = "blacklistUrls"
)

type Request struct {
	Url    string `valid:"required,url" json:"url"`
	Expire string `valid:"time,optional" json:"expire,omitempty"`
}

type Response struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ShortenUrl  string `json:"shorten_url"`
	ShortenCode string `json:"shorten_code"`
}

type Url struct {
	model *models.UrlModel
}

func (u *Url) CreateShorten(w http.ResponseWriter, r *http.Request) {
	log, req, err := u.parseRequestAndValidate(r)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Response{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	// Check black list url
	blackLists := viper.GetStringSlice(keyBlacklist)
	for _, blacklist := range blackLists {
		matched, err := regexp.Match(regexp.QuoteMeta(blacklist), []byte(req.Url))
		if err != nil {
			log.With(zap.String("blacklist_pattern", blacklist)).Error("failed to matching")
		}

		if matched {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, Response{
				Success: false,
				Message: "Url is in blacklists",
			})
		}
	}

	var expire *time.Time
	if len(req.Expire) != 0 {
		expireTime, _ := time.Parse(libs.TimeFormat, req.Expire)
		expire = &expireTime
	}

	// Generate new shorten url
	item, err := u.model.Generate(req.Url, expire)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Response{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	log.With(zap.String("shorten_code", item.Key)).Info("New shorten url generated")
	render.JSON(w, r, Response{
		Success: true,
		ShortenUrl: strings.Join([]string{
			viper.GetString("server.addr"),
			":",
			viper.GetString("server.port"),
			"/r/",
			item.Key,
		}, ""),
		ShortenCode: item.Key,
	})
}

func (u *Url) Redirect(w http.ResponseWriter, r *http.Request) {
	log := libs.GetLogEntry(r)
	shortenCode := core.RouteContext(r.Context()).RouteParams.Get("code")
	if len(shortenCode) == 0 {
		render.Status(r, http.StatusNotFound)
		return
	}

	log = log.With(zap.String("code", shortenCode))

	// Find shorten item
	item, err := u.model.FindByShortCode(shortenCode, true)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			render.Status(r, http.StatusNotFound)
			render.NoContent(w, r)
			return
		}

		if errors.Is(err, models.ErrExpired) {
			render.Status(r, http.StatusGone)
			render.NoContent(w, r)
			return
		}

		log.With(zap.Error(err)).Error("fail to look up url with short code")
		render.Status(r, http.StatusBadRequest)
		render.NoContent(w, r)
		return
	}

	// Redirect to origin url
	http.Redirect(w, r, item.Origin, http.StatusFound)
}

func (u *Url) parseRequestAndValidate(r *http.Request) (log *zap.Logger, req *Request, err error) {
	log = libs.GetLogEntry(r)
	req = &Request{}
	if err = libs.DecodeJSON(r.Body, req); err != nil {
		log.Info("create account error", zap.Error(err))
		return
	}

	log = log.With(zap.String("url", req.Url), zap.String("expire", req.Expire))
	_, err = govalidator.ValidateStruct(req)
	if err != nil {
		log.Error("request invalid", zap.Error(err))
		return
	}
	return
}

func NewUrlController(log *zap.Logger, client *redis.Client, db *gorm.DB) (*Url, error) {
	govalidator.TagMap["time"] = func(str string) bool {
		if len(str) == 0 {
			return true
		}

		return govalidator.IsTime(str, libs.TimeFormat)
	}

	model, err := models.NewUrlModel(log, client, db)
	if err != nil {
		return nil, errors.Wrap(err, "NewUrlController")
	}

	return &Url{
		model: model,
	}, nil
}

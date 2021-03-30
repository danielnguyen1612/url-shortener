package controllers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/danielnguyentb/url-shortener/core"
	"github.com/danielnguyentb/url-shortener/libs"
	"github.com/danielnguyentb/url-shortener/server/models"
)

func TestAdminCtrl(t *testing.T) {
	log := libs.InitLogging()
	adminKey := "aACsyFGAGwXLPmxXL7zqDTc35FRjKcAR"
	viper.SetDefault(keyAdmin, adminKey)

	log.Debug("Initialize db and redis connection")
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{
		//Logger: logger.Default.LogMode(logger.Info),
	})
	defer func() {
		require.NoError(t, os.Remove("test.db"))
	}()
	require.NoError(t, err)

	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	model, err := models.NewUrlModel(log, client, db)
	require.NoError(t, err)

	adminCtrl, err := NewAdminController(log, client, db)
	require.NoError(t, err)

	r := core.NewRouter()
	r.Get("/admin/list", adminCtrl.GetList)
	r.Delete("/admin/:code", adminCtrl.Delete)

	log.Debug("Request admin without token key")
	resp, _, err := testAdminHandler(log, r, "GET", "/admin/list", "", strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusForbidden)

	log.Debug("Request admin with token and empty item be expected")
	resp, res, err := testAdminHandler(log, r, "GET", "/admin/list", adminKey, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.True(t, res.Success)
	assert.Empty(t, res.Items)

	log.Debug("Create dummy shorten items")
	item1, _ := model.Generate("http://url-1.com", nil)
	item2, _ := model.Generate("http://url-2.com", nil)
	_, _ = model.Generate("http://url-3.com", nil)

	log.Debug("Request admin list with correct item returned")
	resp, res, err = testAdminHandler(log, r, "GET", "/admin/list", adminKey, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.True(t, res.Success)
	assert.NotEmpty(t, res.Items)
	assert.Equal(t, len(res.Items), 3)

	log.Debug("Request admin list and short-code criteria filtering with correct item returned")
	resp, res, err = testAdminHandler(log, r, "GET", strings.Join([]string{"/admin/list?code=", item1.Key}, ""), adminKey, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.True(t, res.Success)
	assert.NotEmpty(t, res.Items)
	assert.Equal(t, len(res.Items), 1)
	assert.Equal(t, res.Items[0].Origin, item1.Origin)

	log.Debug("Request admin list and term criteria filtering with correct item returned")
	resp, res, err = testAdminHandler(log, r, "GET", strings.Join([]string{"/admin/list?term=url-2.com"}, ""), adminKey, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.True(t, res.Success)
	assert.NotEmpty(t, res.Items)
	assert.Equal(t, len(res.Items), 1)
	assert.Equal(t, res.Items[0].Origin, item2.Origin)

	log.Debug("Soft delete shorten url with shorten-code")
	resp, _, err = testAdminHandler(log, r, "DELETE", strings.Join([]string{"/admin/", item1.Key}, ""), adminKey, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	log.Debug("Get soft-deleted item from admin list")
	resp, res, err = testAdminHandler(log, r, "GET", strings.Join([]string{"/admin/list?code=", item1.Key}, ""), adminKey, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.True(t, res.Success)
	assert.NotEmpty(t, res.Items)
	assert.Equal(t, len(res.Items), 1)
	assert.False(t, res.Items[0].Status)
}

func testAdminHandler(log *zap.Logger, h http.Handler, method, path, adminKey string, body io.Reader) (*http.Response, *AdminResponse, error) {
	r, _ := http.NewRequest(method, path, body)
	if len(adminKey) > 0 {
		r.Header.Add(keyAuthorizeHeader, adminKey)
	}

	w := httptest.NewRecorder()

	libs.NewZapLogEntry(log)(h).ServeHTTP(w, r)

	cT := w.Header().Get("Content-Type")
	resp := &AdminResponse{}
	if strings.Contains(cT, "application/json") {
		if err := json.Unmarshal(w.Body.Bytes(), resp); err != nil {
			return nil, nil, err
		}
	}

	return w.Result(), resp, nil
}

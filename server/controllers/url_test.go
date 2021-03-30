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
)

func TestUrlCtrl(t *testing.T) {
	log := libs.InitLogging()

	// Set blacklist for testing
	viper.SetDefault(keyBlacklist, []string{
		"google.com",
	})

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

	urlCtrl, err := NewUrlController(log, client, db)
	require.NoError(t, err)

	r := core.NewRouter()
	r.Post("/create", urlCtrl.CreateShorten)
	r.Get("/r/:code", urlCtrl.Redirect)

	log.Debug("Request create shorten with empty body, request should fail")
	req, err := json.Marshal(Request{})
	require.NoError(t, err)
	resp, body, err := testHandler(t, log, r, "POST", "/create", strings.NewReader(string(req)))
	require.NoError(t, err)
	require.NotNil(t, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, false, body.Success)

	log.Debug("Request create shorten with invalid url, request should fail")
	req, err = json.Marshal(Request{
		Url: "abababa",
	})
	require.NoError(t, err)
	resp, body, err = testHandler(t, log, r, "POST", "/create", strings.NewReader(string(req)))
	require.NoError(t, err)
	require.NotNil(t, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, false, body.Success)

	log.Debug("Request create shorten with invalid expire, request should fail")
	req, err = json.Marshal(Request{
		Url:    "http://google.com",
		Expire: "abababa",
	})
	require.NoError(t, err)
	resp, body, err = testHandler(t, log, r, "POST", "/create", strings.NewReader(string(req)))
	require.NoError(t, err)
	require.NotNil(t, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, false, body.Success)

	log.Debug("Request create shorten with valid shorten")
	req, err = json.Marshal(Request{
		Url:    "http://yahoo.com",
		Expire: "2022-01-01 00:00:00",
	})
	require.NoError(t, err)
	resp, body, err = testHandler(t, log, r, "POST", "/create", strings.NewReader(string(req)))
	require.NoError(t, err)
	require.NotNil(t, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body.Success)
	assert.NotEmpty(t, body.ShortenUrl)
	assert.NotEmpty(t, body.ShortenCode)

	log.Debug("Request to redirect url")
	resp, _, err = testHandler(t, log, r, "GET",
		strings.Join([]string{"/r/", body.ShortenCode}, ""), strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusFound)

	location, err := resp.Location()
	require.NoError(t, err)
	assert.NotEmpty(t, location.String())

	log.Debug("Request to non-exists shorten")
	resp, _, err = testHandler(t, log, r, "GET", "/r/non-exists", strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func testHandler(t *testing.T, log *zap.Logger, h http.Handler, method, path string, body io.Reader) (*http.Response, *Response, error) {
	r, _ := http.NewRequest(method, path, body)
	w := httptest.NewRecorder()

	libs.NewZapLogEntry(log)(h).ServeHTTP(w, r)

	cT := w.Header().Get("Content-Type")
	resp := &Response{}
	if strings.Contains(cT, "application/json") {
		if err := json.Unmarshal(w.Body.Bytes(), resp); err != nil {
			return nil, nil, err
		}
	}

	return w.Result(), resp, nil
}

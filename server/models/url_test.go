package models

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/danielnguyentb/url-shortener/libs"
)

type testCases struct {
	t     *testing.T
	log   *zap.Logger
	redis *redis.Client
	db    *gorm.DB
	model *UrlModel
}

func (c testCases) testGenerate() {
	expireDuration, err := time.ParseDuration("15m")
	require.NoError(c.t, err)
	expire := time.Now().Add(expireDuration)

	c.log.Debug("Generate new shorten item with full url")
	item, err := c.model.Generate("http://google.com", &expire)
	require.NoError(c.t, err)

	assert.NotEmpty(c.t, item.Key)
	assert.GreaterOrEqual(c.t, time.Now().Unix(), item.CreatedAt.Unix())
	assert.Equal(c.t, item.Hits, uint(0))
	assert.Greater(c.t, item.Expiry.Unix(), time.Now().Unix())

	c.log.Debug("New item should be stored in redis layer")
	cache := Url{}
	cacheString := c.redis.Get(context.Background(), item.GetCacheKey()).Val()
	require.NoError(c.t, cache.Unmarshall(cacheString))
	assert.Equal(c.t, item.ID, cache.ID)
	assert.Equal(c.t, item.Key, cache.Key)

	c.log.Debug("Generate new shorten url with wrong expire")
	expireDuration, err = time.ParseDuration("-1m")
	require.NoError(c.t, err)
	wrongExpire := time.Now().Add(expireDuration)
	item, err = c.model.Generate("http://google.com", &wrongExpire)
	require.Error(c.t, err)
	assert.Nil(c.t, item)

	c.log.Debug("Generate new shorten url with non-expiry")
	item, err = c.model.Generate("http://google.com", nil)
	require.NoError(c.t, err)
	assert.Nil(c.t, item.Expiry)
}

func (c testCases) testFindByShortCode() {
	expireDuration, err := time.ParseDuration("15m")
	require.NoError(c.t, err)
	expire := time.Now().Add(expireDuration)

	item, err := c.model.Generate("http://google.com", &expire)
	require.NoError(c.t, err)

	c.log.Debug("Find by short code with hit")
	lookup, err := c.model.FindByShortCode(item.Key, true)
	require.NoError(c.t, err)

	assert.Equal(c.t, lookup.Key, item.Key)

	c.log.Debug("Redis key should be generated")
	cache := Url{}
	cacheString := c.redis.Get(context.Background(), lookup.GetCacheKey()).Val()
	require.NoError(c.t, cache.Unmarshall(cacheString))
	assert.Equal(c.t, lookup.ID, cache.ID)
	assert.Equal(c.t, lookup.Key, cache.Key)

	c.log.Debug("The item should increase hit on db")
	dbItem := Url{}
	result := c.db.Model(&Url{}).Where("key = ?", lookup.Key).First(&dbItem)
	require.NoError(c.t, result.Error)
	assert.Greater(c.t, dbItem.Hits, lookup.Hits)

	c.log.Debug("Clear cache item on redis, should get from db instead")
	_, err = c.redis.Del(context.Background(), lookup.GetCacheKey()).Result()
	require.NoError(c.t, err)

	lookup, err = c.model.FindByShortCode(item.Key, false)
	fmt.Printf("%+v", err)
	require.NoError(c.t, err)
	assert.Equal(c.t, lookup.Key, item.Key)

	c.log.Debug("Clear cache, set item as expired. Error should be expected")
	_, err = c.redis.Del(context.Background(), lookup.GetCacheKey()).Result()
	require.NoError(c.t, err)

	pastTime, err := time.ParseDuration("-10m")
	require.NoError(c.t, err)

	require.NoError(c.t, c.db.Model(&Url{}).Where("key = ?", lookup.Key).Update("expiry", time.Now().Add(pastTime)).Error)
	lookup, err = c.model.FindByShortCode(item.Key, false)
	require.Error(c.t, err)
	assert.True(c.t, errors.Is(err, ErrExpired))

	c.log.Debug("Lookup with non-exists key")
	lookup, err = c.model.FindByShortCode("non-exists", false)
	require.Error(c.t, err)
	assert.True(c.t, errors.Is(err, ErrNotFound))
}

func (c testCases) testDeleteItem() {
	item, err := c.model.Generate("http://google.com", nil)
	require.NoError(c.t, err)

	c.log.Debug("Delete item by short code")
	ok, err := c.model.Delete(item.Key)
	require.NoError(c.t, err)
	assert.True(c.t, ok)

	c.log.Debug("Lookup item by short key, expired should be expected")
	lookup, err := c.model.FindByShortCode(item.Key, false)
	require.Error(c.t, err)
	require.True(c.t, errors.Is(err, ErrExpired))
	assert.NotNil(c.t, lookup)

	c.log.Debug("Delete item by non-exists key, not found should be expected")
	lookup, err = c.model.FindByShortCode("non-exists", false)
	require.Error(c.t, err)
	require.True(c.t, errors.Is(err, ErrNotFound))
	assert.Nil(c.t, lookup)
}

func (c testCases) testGetListItem() {
	c.log.Debug("Drop all existing items")
	require.NoError(c.t, c.db.Exec("DELETE FROM urls").Error)

	c.log.Debug("Get all exists, empty return expected")
	items, err := c.model.GetList("", "")
	require.NoError(c.t, err)
	assert.Empty(c.t, items)

	c.log.Debug("Insert new item")
	item1, err := c.model.Generate("https://google.com", nil)
	require.NoError(c.t, err)

	item2, err := c.model.Generate("https://yahoo.com", nil)
	require.NoError(c.t, err)

	c.log.Debug("Get all exists, should be 2")
	items, err = c.model.GetList("", "")
	require.NoError(c.t, err)
	assert.Equal(c.t, len(items), 2)

	c.log.Debug("Get list item by short code")
	items, err = c.model.GetList(item1.Key, "")
	require.NoError(c.t, err)
	assert.Equal(c.t, len(items), 1)
	assert.Equal(c.t, items[0].ID, item1.ID)

	c.log.Debug("Get list item by keyword")
	items, err = c.model.GetList("", "yahoo")
	require.NoError(c.t, err)
	assert.Equal(c.t, len(items), 1)
	assert.Equal(c.t, items[0].ID, item2.ID)
}

func TestUrl(t *testing.T) {
	log := libs.InitLogging()

	log.Debug("Initialize db and redis connection")
	db, err := gorm.Open(sqlite.Open("test.db"))
	defer func() {
		require.NoError(t, os.Remove("test.db"))
	}()
	require.NoError(t, err)

	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	urlModel, err := NewUrlModel(log, client, db)
	require.NoError(t, err)

	testCase := &testCases{
		t:     t,
		model: urlModel,
		redis: client,
		db:    db,
		log:   log,
	}

	testCase.testGenerate()
	testCase.testFindByShortCode()
	testCase.testDeleteItem()
	testCase.testGetListItem()
}

package models

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"codeberg.org/ac/base62"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Buffer for id to generate short code more than 4 characters
const idBuffer = 5000000
const keyExpireTime = "expire"

var (
	ErrNotFound = errors.New("Not Found")
	ErrExpired  = errors.New("Expired")
)

type Url struct {
	ID        int    `gorm:"primaryKey;autoIncrement"`
	Key       string `gorm:"index;not null;unique"`
	Origin    string `gorm:"not null"`
	Hits      uint   `gorm:"default:0"`
	Expiry    *time.Time
	CreatedAt time.Time `gorm:"autoCreateTime"`
	Status    bool      `gorm:"default:1"`
}

func (u Url) GetCacheKey() string {
	return strings.Join([]string{"item", u.Key}, "-")
}

// Marshall will encode item into JSON string
func (u Url) Marshall() (string, error) {
	b, err := json.Marshal(u)
	if err != nil {
		return "", errors.Wrap(err, "json.Marshal")
	}

	return string(b), nil
}

func (u *Url) Unmarshall(encoded string) error {
	if err := json.Unmarshal([]byte(encoded), u); err != nil {
		return errors.Wrap(err, "json.Unmarshal")
	}

	return nil
}

func (u Url) IsExpired() bool {
	return !u.Status || (u.Expiry != nil && time.Now().After(*u.Expiry))
}

type UrlModel struct {
	redis *redis.Client
	db    *gorm.DB
	log   *zap.Logger
}

func (u *UrlModel) Generate(url string, expire *time.Time) (*Url, error) {
	item := Url{
		Origin: url,
		Status: true,
		Key:    uuid.NewString(), // Key as temporary
	}

	if expire != nil {
		if time.Now().After(*expire) {
			return nil, errors.New("expire time is invalid")
		}

		item.Expiry = expire
	}

	if err := u.db.Transaction(func(tx *gorm.DB) error {
		if result := u.db.Create(&item); result.Error != nil {
			return errors.Wrap(result.Error, "u.db.Create")
		}

		// Generate short-code based on id
		item.Key = base62.Encode(uint32(item.ID + idBuffer))

		// Update item
		if result := u.db.Model(&item).Update("key", item.Key); result.Error != nil {
			return errors.Wrap(result.Error, "u.db.Update")
		}

		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "u.db.Transaction")
	}

	if err := u.cacheItem(item); err != nil {
		u.log.With(zap.Error(err)).Error("u.cacheItem")
	}

	return &item, nil
}

func (u *UrlModel) cacheItem(item Url) error {
	// Cache data
	cache, err := item.Marshall()
	if err != nil {
		return errors.Wrap(err, "can not marshall item into JSON type")
	}

	expire, err := time.ParseDuration("30m")
	if err != nil {
		return errors.Wrap(err, "time.ParseDuration")
	}

	if _, err := u.redis.Set(context.Background(), item.GetCacheKey(), cache, expire).Result(); err != nil {
		return errors.Wrap(err, "can not set item into redis")
	}

	return nil
}

func (u *UrlModel) hitItem(shortCode string) error {
	// Hit the item
	if result := u.db.Model(&Url{}).Where("key = ?", shortCode).UpdateColumn("hits", gorm.Expr("hits + ?", 1)); result.Error != nil {
		return errors.Wrap(result.Error, "u.db.Where.UpdateColumn")
	}

	return nil
}

func (u *UrlModel) FindByShortCode(shortCode string, hit bool) (*Url, error) {
	item := &Url{
		Key: shortCode,
	}

	// Get from cache
	cache, err := u.redis.Get(context.Background(), item.GetCacheKey()).Result()
	if err != nil && err != redis.Nil {
		return nil, errors.Wrap(err, "u.redis.Get")
	}
	if len(cache) != 0 {
		if err := item.Unmarshall(cache); err != nil {
			return nil, errors.Wrap(err, "item.Unmarshall")
		}

		if item.IsExpired() {
			return nil, ErrExpired
		}

		if err := u.hitItem(shortCode); err != nil {
			return nil, errors.Wrap(err, "u.hitItem")
		}

		return item, nil
	}

	if result := u.db.Where("key = ?", shortCode).First(item); result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, errors.Wrap(result.Error, "FindByShortCode")
	}

	if item.IsExpired() {
		return nil, ErrExpired
	}

	if hit {
		if err := u.hitItem(shortCode); err != nil {
			return nil, errors.Wrap(err, "u.hitItem")
		}
	}

	if err := u.cacheItem(*item); err != nil {
		u.log.With(zap.Error(err)).Error("u.cacheItem")
	}

	return item, nil
}

func (u *UrlModel) Delete(shortCode string) (bool, error) {
	item, err := u.FindByShortCode(shortCode, false)
	if err != nil {
		return false, errors.Wrap(err, "u.FindByShortCode")
	}

	item.Status = false
	if result := u.db.Save(item); result.Error != nil {
		return false, errors.Wrap(result.Error, "u.db.Save")
	}

	if err := u.cacheItem(*item); err != nil {
		u.log.With(zap.Error(err)).Error("u.cacheItem")
	}

	return true, nil
}

func (u *UrlModel) GetList(shortCode, keywords string) ([]Url, error) {
	model := u.db.Model(&Url{})
	if len(shortCode) != 0 {
		model.Where("key = ?", shortCode)
	}

	if len(keywords) != 0 {
		model.Where("origin LIKE ?", strings.Join([]string{"%", keywords, "%"}, ""))
	}

	var results []Url
	if err := model.Find(&results).Error; err != nil {
		return nil, errors.Wrap(err, "model.Find")
	}

	return results, nil
}

func NewUrlModel(log *zap.Logger, redis *redis.Client, db *gorm.DB) (*UrlModel, error) {
	// Automatically migrate model into db layer
	if err := db.AutoMigrate(&Url{}); err != nil {
		return nil, errors.Wrap(err, "NewUrlModel.AutoMigrate")
	}

	return &UrlModel{
		redis: redis,
		db:    db,
		log:   log,
	}, nil
}

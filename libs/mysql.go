package libs

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	keyMysqlHost = "mysql.host"
	keyMysqlUser = "mysql.user"
	keyMysqlPwd  = "mysql.pwd"
	keyMysqlDb   = "mysql.db"
	keyMysqlPort = "mysql.port"
)

func NewMysqlWithViper(log *zap.Logger) (*gorm.DB, error) {
	for _, key := range []string{keyMysqlHost, keyMysqlUser, keyMysqlPwd, keyMysqlDb} {
		if len(viper.GetString(key)) == 0 {
			return nil, errors.New(fmt.Sprintf("%s must be provided", key))
		}
	}

	port := viper.GetString(keyMysqlPort)
	if len(port) == 0 {
		port = "3306"
	}

	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local",
			viper.GetString(keyMysqlUser),
			viper.GetString(keyMysqlPwd),
			viper.GetString(keyMysqlHost),
			port,
			viper.GetString(keyMysqlDb),
		),
	}))
	if err != nil {
		return nil, errors.Wrap(err, "gorm.Open")
	}

	return db, nil
}

// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"cuniBTCReward/api/internal/config"
	"cuniBTCReward/pkg/gormz"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config   config.Config
	Database *gorm.DB
}

func NewServiceContext(c config.Config) *ServiceContext {
	gormConfig := &gorm.Config{}
	if c.SqlLog {
		gormConfig.Logger = gormz.NewGormLogger()
	}
	db, err := gorm.Open(mysql.Open(c.DataSource), gormConfig)
	logx.Must(err)

	return &ServiceContext{
		Config:   c,
		Database: db,
	}
}

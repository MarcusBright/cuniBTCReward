package crontab

import (
	"cuniBTCReward/pkg/gormz"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Test_Reward(t *testing.T) {
	db, err := gorm.Open(mysql.Open("root:123456@tcp(127.0.0.1:3317)/cunibtcreward?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{
		Logger: gormz.NewGormLogger(true),
	})
	if err != nil {
		return
	}
	totalReward, err := decimal.NewFromString("0.123")
	if err != nil {
		t.Log(err)
		return
	}
	totalReward = totalReward.Mul(decimal.New(1, 8))
	err = distributeExtraReward(db, "b2uniBTC", 1793163148, 11155111, totalReward, "0x84252bf3f7B55970bF01bd127649ded42e11bDC1", "0x32377EDBC67e99bA3395D89E1f93A0ff26595ec7")
	if err != nil {
		t.Log("err:", err)
	}
}

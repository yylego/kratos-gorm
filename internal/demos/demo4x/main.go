// demo4x: One-error form with TransactRun
// Demonstrates TransactRun, which takes a function to wrap transaction failures and returns one error
// The project recommends Transaction (see demo1x); use this form just when one error is what the code needs
//
// demo4x: 用 TransactRun 的单错误写法
// 演示 TransactRun: 传入一个包装事务失败的函数, 只返回一个错误
// 项目推荐用 Transaction (见 demo1x); 这种写法只在确实只想拿一个错误时用
package main

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"github.com/yylego/kratos-gorm/gormkratos"
	"github.com/yylego/must"
	"github.com/yylego/rese"
	"github.com/yylego/zaplog"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Guest represents guest data in the system
// Guest 表示系统中的访客数据
type Guest struct {
	ID   uint   `gorm:"primarykey"` // Auto-increment ID // 自增主键
	Name string `gorm:"not null"`   // Guest name, must set // 访客名称,必填
}

func main() {
	dsn := fmt.Sprintf("file:db-%s?mode=memory&cache=shared", uuid.New().String())
	db := rese.P1(gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}))
	defer rese.F0(rese.P1(db.DB()).Close)

	must.Done(db.AutoMigrate(&Guest{}))

	ctx := context.Background()

	erk := gormkratos.TransactRun(ctx, db, func(db *gorm.DB) *errors.Error {
		guest := &Guest{Name: "Chloe"}
		if err := db.Create(guest).Error; err != nil {
			return errors.Newf(500, "DB_ERROR", "create failed: %v", err)
		}
		zaplog.LOG.Debug("Created guest", zap.Uint("id", guest.ID), zap.String("name", guest.Name))
		return nil
	}, func(ctx context.Context, cause error) *errors.Error {
		zaplog.LOG.Warn("Transaction failed", zap.Error(cause))
		return errors.Newf(500, "TRANSACTION_ERROR", "transaction failed: %v", cause)
	})
	if erk != nil {
		zaplog.LOG.Error("Create guest failed", zap.Error(erk))
	}
}

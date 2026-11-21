// demo2x: Transaction rollback on business logic errors
// Demonstrates auto rollback when business logic returns errors
//
// demo2x: 业务逻辑错误时事务回滚
// 演示业务逻辑返回错误时的自动回滚
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

	erk := Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		guest := &Guest{Name: "Bob"}
		if err := db.Create(guest).Error; err != nil {
			return errors.Newf(500, "DB_ERROR", "create failed: %v", err)
		}
		zaplog.LOG.Debug("Created guest (then rollback)", zap.Uint("id", guest.ID), zap.String("name", guest.Name))
		return errors.Newf(400, "BAD_REQUEST", "validation failed")
	})
	zaplog.LOG.Error("Create guest rolled back", zap.Error(erk))

	var count int64
	must.Done(db.Model(&Guest{}).Count(&count).Error)
	zaplog.LOG.Debug("Guest count post rollback", zap.Int64("count", count))
}

// Transaction wraps gormkratos.Transaction with single-error-return pattern
// Returns business errors (erk) as-is, wraps database errors (err) as TRANSACTION_ERROR
//
// Transaction 用单错误返回模式包装 gormkratos.Transaction
// 直接返回业务错误 (erk),将数据库错误 (err) 包装为 TRANSACTION_ERROR
func Transaction(ctx context.Context, db *gorm.DB, run func(db *gorm.DB) *errors.Error) *errors.Error {
	erk, err := gormkratos.Transaction(ctx, db, run)
	if err != nil {
		if erk != nil {
			return erk
		}
		return errors.Newf(500, "TRANSACTION_ERROR", "transaction failed: %v", err)
	}
	return nil
}

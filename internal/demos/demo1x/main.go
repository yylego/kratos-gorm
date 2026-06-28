// demo1x: Basic transaction with success case
// Demonstrates the simplest usage of gormkratos.Transaction
//
// demo1x: 基础事务成功案例
// 演示 gormkratos.Transaction 的最简单用法
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

// Admin represents admin data in the system
// Admin 表示系统中的管理员数据
type Admin struct {
	ID   uint   `gorm:"primarykey"` // Auto-increment ID // 自增主键
	Name string `gorm:"not null"`   // Admin name, must set // 管理员名称,必填
}

func main() {
	dsn := fmt.Sprintf("file:db-%s?mode=memory&cache=shared", uuid.New().String())
	db := rese.P1(gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}))
	defer rese.F0(rese.P1(db.DB()).Close)

	must.Done(db.AutoMigrate(&Admin{}))

	ctx := context.Background()

	erk := Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		admin := &Admin{Name: "Alice"}
		if err := db.Create(admin).Error; err != nil {
			return ErrorServerDbError("create failed: %v", err)
		}
		zaplog.LOG.Debug("Created admin", zap.Uint("id", admin.ID), zap.String("name", admin.Name))
		return nil
	})
	if erk != nil {
		zaplog.LOG.Error("Error", zap.Error(erk))
	}
}

// ErrorServerDbError creates Kratos database operation errors
// ErrorServerDbError 创建 Kratos 数据库操作错误
func ErrorServerDbError(format string, args ...interface{}) *errors.Error {
	return errors.New(500, "DB_ERROR", fmt.Sprintf(format, args...))
}

// ErrorServerDbTransactionError creates Kratos transaction-level errors
// ErrorServerDbTransactionError 创建 Kratos 事务级错误
func ErrorServerDbTransactionError(format string, args ...interface{}) *errors.Error {
	return errors.New(500, "TRANSACTION_ERROR", fmt.Sprintf(format, args...))
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
		return ErrorServerDbTransactionError("transaction failed: %v", err)
	}
	return nil
}

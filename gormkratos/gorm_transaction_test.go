// Package gormkratos_test: Tests gormkratos transaction integration
// Validates two-error-return pattern and transaction execution
//
// gormkratos_test: gormkratos 事务集成的测试
// 验证双错误返回模式和事务执行
package gormkratos_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yylego/kratos-errors/errorskratos/must/erkrequire"
	"github.com/yylego/kratos-gorm/gormkratos"
	"github.com/yylego/kratos-gorm/internal/errorspb"
	"github.com/yylego/must"
	"github.com/yylego/rese"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates isolated in-memory SQLite database
// setupTestDB 创建独立的内存 SQLite 数据库
func setupTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:db-%s?mode=memory&cache=shared", uuid.New().String())
	db := rese.P1(gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}))
	t.Cleanup(func() {
		must.Done(rese.P1(db.DB()).Close())
	})
	return db
}

// TestTransactionSuccess tests success transaction execution
// TestTransactionSuccess 测试成功的事务执行
func TestTransactionSuccess(t *testing.T) {
	db := setupTestDB(t)

	// Student represents simple test data
	// Student 表示简单的测试数据
	type Student struct {
		ID   uint   `gorm:"primarykey"`           // Auto-increment PK // 自增PK
		Name string `gorm:"column:name;not null"` // Student name // 学生名称
	}

	require.NoError(t, db.AutoMigrate(&Student{}))

	ctx := context.Background()

	erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		student := &Student{Name: "test-student"}
		if err := db.Create(student).Error; err != nil {
			return errorspb.ErrorServerDbError("failed to create student: %v", err)
		}
		return nil
	})
	require.NoError(t, err)
	erkrequire.NoError(t, erk)

	// Check data was inserted
	// 检查数据已插入
	var count int64
	require.NoError(t, db.Model(&Student{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

// TestTransactionBusinessError tests business logic errors handling
// TestTransactionBusinessError 测试业务逻辑错误处理
func TestTransactionBusinessError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		return errorspb.ErrorBadRequest("student name is blank")
	})
	require.Error(t, err)
	erkrequire.Error(t, erk)
	require.True(t, errorspb.IsBadRequest(erk))
	// err is the business error itself, with no wrapping in between
	// err 就是这个业务错误本身, 中间没有再包一层
	require.Same(t, erk, err)
}

// TestTransactionTimeout tests context timeout handling
// TestTransactionTimeout 测试上下文超时处理
func TestTransactionTimeout(t *testing.T) {
	db := setupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		time.Sleep(100 * time.Millisecond) // Exceed timeout // 超过超时时间
		return nil
	})
	require.Error(t, err)
	erkrequire.NoError(t, erk)
}

// TestTransactionRollback tests when transaction does rollback
// TestTransactionRollback 测试事务回滚
func TestTransactionRollback(t *testing.T) {
	db := setupTestDB(t)

	// Guest represents simple test data
	// Guest 表示简单的测试数据
	type Guest struct {
		ID   uint   `gorm:"primarykey"`           // Auto-increment PK // 自增PK
		Name string `gorm:"column:name;not null"` // Guest name // 访客名称
	}

	require.NoError(t, db.AutoMigrate(&Guest{}))

	ctx := context.Background()

	erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		guest := &Guest{Name: "rollback-guest"}
		if err := db.Create(guest).Error; err != nil {
			return errorspb.ErrorServerDbError("failed to create guest: %v", err)
		}
		// Simulate business errors to cause rollback
		// 模拟业务错误导致回滚
		return errorspb.ErrorBadRequest("business validation failed")
	})
	require.Error(t, err)
	erkrequire.Error(t, erk)
	require.True(t, errorspb.IsBadRequest(erk))

	// Check data was rolled back, not in DB
	// 检查数据已回滚,未插入数据库
	var count int64
	require.NoError(t, db.Model(&Guest{}).Where("name = ?", "rollback-guest").Count(&count).Error)
	require.Equal(t, int64(0), count)
}

// TestTransactionNested tests nested transaction execution
// TestTransactionNested 测试嵌套事务执行
func TestTransactionNested(t *testing.T) {
	db := setupTestDB(t)

	// Invoice represents test data
	// Invoice 表示测试数据
	type Invoice struct {
		ID     uint   `gorm:"primarykey"`             // Auto-increment PK // 自增PK
		Status string `gorm:"column:status;not null"` // Status // 账单状态
	}

	require.NoError(t, db.AutoMigrate(&Invoice{}))

	ctx := context.Background()

	erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		invoice := &Invoice{Status: "pending"}
		if err := db.Create(invoice).Error; err != nil {
			return errorspb.ErrorServerDbError("failed to create invoice: %v", err)
		}

		// Nested transaction
		// 嵌套事务
		erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
			invoice.Status = "confirmed"
			if err := db.Save(invoice).Error; err != nil {
				return errorspb.ErrorServerDbError("failed to update invoice: %v", err)
			}
			return nil
		})
		if err != nil {
			if erk != nil {
				return erk
			}
			return errorspb.ErrorServerDbTransactionError("nested transaction failed: %v", err)
		}

		return nil
	})
	require.NoError(t, err)
	erkrequire.NoError(t, erk)

	// Check data was inserted and updated
	// 检查数据已插入并更新
	var invoice Invoice
	require.NoError(t, db.First(&invoice).Error)
	require.Equal(t, "confirmed", invoice.Status)
}

// TestTransactionNestedRollback tests nested transaction rollback
// TestTransactionNestedRollback 测试嵌套事务回滚
func TestTransactionNestedRollback(t *testing.T) {
	db := setupTestDB(t)

	// Product represents test data
	// Product 表示测试数据
	type Product struct {
		ID    uint   `gorm:"primarykey"`           // Auto-increment PK // 自增PK
		Name  string `gorm:"column:name;not null"` // Name // 产品名称
		Price int    `gorm:"column:price"`         // Price in cents // 产品价格(分)
	}

	require.NoError(t, db.AutoMigrate(&Product{}))

	ctx := context.Background()

	erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		product := &Product{Name: "phone", Price: 1000}
		if err := db.Create(product).Error; err != nil {
			return errorspb.ErrorServerDbError("failed to create product: %v", err)
		}

		// Nested transaction with rollback
		// 嵌套事务回滚
		erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
			product.Price = 900
			if err := db.Save(product).Error; err != nil {
				return errorspb.ErrorServerDbError("failed to update product: %v", err)
			}
			// Simulate nested business errors
			// 模拟嵌套业务错误
			return errorspb.ErrorBadRequest("nested validation failed")
		})
		if err != nil {
			if erk != nil {
				return erk
			}
			return errorspb.ErrorServerDbTransactionError("nested transaction failed: %v", err)
		}

		return nil
	})
	require.Error(t, err)
	erkrequire.Error(t, erk)
	require.True(t, errorspb.IsBadRequest(erk))

	// Check all data was rolled back
	// 检查所有数据已回滚
	var count int64
	require.NoError(t, db.Model(&Product{}).Count(&count).Error)
	require.Equal(t, int64(0), count)
}

// TestTransactionDifferentErrors tests handling different types of errors
// TestTransactionDifferentErrors 测试不同类型错误的处理
func TestTransactionDifferentErrors(t *testing.T) {
	t.Run("BadRequest", func(t *testing.T) {
		db := setupTestDB(t)
		ctx := context.Background()

		erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
			return errorspb.ErrorBadRequest("invalid input")
		})
		require.Error(t, err)
		erkrequire.Error(t, erk)
		require.True(t, errorspb.IsBadRequest(erk))
	})

	t.Run("ServerDbError", func(t *testing.T) {
		db := setupTestDB(t)
		ctx := context.Background()

		erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
			return errorspb.ErrorServerDbError("database operation failed")
		})
		require.Error(t, err)
		erkrequire.Error(t, erk)
		require.True(t, errorspb.IsServerDbError(erk))
	})
}

// TestTransactionWithTxOptions tests transaction with custom TxOptions
// TestTransactionWithTxOptions 测试使用自定义 TxOptions 的事务
func TestTransactionWithTxOptions(t *testing.T) {
	db := setupTestDB(t)

	// Article represents test data
	// Article 表示测试数据
	type Article struct {
		ID    uint `gorm:"primarykey"`   // Auto-increment PK // 自增PK
		Views int  `gorm:"column:views"` // Page views // 阅读数
	}

	require.NoError(t, db.AutoMigrate(&Article{}))

	ctx := context.Background()

	// Test with ReadCommitted isolation
	// 测试 ReadCommitted 隔离级别
	txOptions := &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	}

	erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		article := &Article{Views: 10000}
		if err := db.Create(article).Error; err != nil {
			return errorspb.ErrorServerDbError("failed to create article: %v", err)
		}
		return nil
	}, txOptions)
	require.NoError(t, err)
	erkrequire.NoError(t, erk)

	// Check data was inserted
	// 检查数据已插入
	var count int64
	require.NoError(t, db.Model(&Article{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

// newTxErk turns a failure of the transaction itself into the project's own Kratos error
// newTxErk 把事务本身的失败包成项目自己的 Kratos 错误
func newTxErk(ctx context.Context, cause error) *errors.Error {
	return errorspb.ErrorServerDbTransactionError("transaction failed: %v", cause)
}

// TestTransactRun tests TransactRun, which returns one Kratos error
// TestTransactRun 测试只返回一个 Kratos 错误的 TransactRun
func TestTransactRun(t *testing.T) {
	// Guest represents simple test data
	// Guest 表示简单的测试数据
	type Guest struct {
		ID   uint   `gorm:"primarykey"`           // Auto-increment PK // 自增PK
		Name string `gorm:"column:name;not null"` // Guest name // 访客名称
	}

	t.Run("success", func(t *testing.T) {
		db := setupTestDB(t)
		require.NoError(t, db.AutoMigrate(&Guest{}))

		erk := gormkratos.TransactRun(context.Background(), db, func(db *gorm.DB) *errors.Error {
			if err := db.Create(&Guest{Name: "alice"}).Error; err != nil {
				return errorspb.ErrorServerDbError("failed to create guest: %v", err)
			}
			return nil
		}, newTxErk)
		erkrequire.NoError(t, erk)

		var count int64
		require.NoError(t, db.Model(&Guest{}).Count(&count).Error)
		require.Equal(t, int64(1), count)
	})

	t.Run("run-error-comes-back-as-it-is", func(t *testing.T) {
		db := setupTestDB(t)
		require.NoError(t, db.AutoMigrate(&Guest{}))

		erk := gormkratos.TransactRun(context.Background(), db, func(db *gorm.DB) *errors.Error {
			if err := db.Create(&Guest{Name: "bob"}).Error; err != nil {
				return errorspb.ErrorServerDbError("failed to create guest: %v", err)
			}
			return errorspb.ErrorBadRequest("business validation failed")
		}, newTxErk)
		erkrequire.Error(t, erk)
		require.True(t, errorspb.IsBadRequest(erk))

		var count int64
		require.NoError(t, db.Model(&Guest{}).Count(&count).Error)
		require.Equal(t, int64(0), count)
	})

	t.Run("transaction-fault-goes-through-new-erk", func(t *testing.T) {
		db := setupTestDB(t)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// newErk gets the same ctx and the cause of the transaction fault
		// newErk 拿到同一个 ctx 和事务失败的原因
		var erkCtx context.Context
		var erkCause error
		erk := gormkratos.TransactRun(ctx, db, func(db *gorm.DB) *errors.Error {
			time.Sleep(100 * time.Millisecond) // Exceed timeout // 超过超时时间
			return nil
		}, func(ctx context.Context, cause error) *errors.Error {
			erkCtx, erkCause = ctx, cause
			return newTxErk(ctx, cause)
		})
		erkrequire.Error(t, erk)
		require.True(t, errorspb.IsServerDbTransactionError(erk))
		require.Equal(t, ctx, erkCtx)
		require.Error(t, erkCause)
	})

	t.Run("nested-run-error-rolls-back-both", func(t *testing.T) {
		db := setupTestDB(t)
		require.NoError(t, db.AutoMigrate(&Guest{}))

		ctx := context.Background()

		erk := gormkratos.TransactRun(ctx, db, func(db *gorm.DB) *errors.Error {
			if err := db.Create(&Guest{Name: "chloe"}).Error; err != nil {
				return errorspb.ErrorServerDbError("failed to create guest: %v", err)
			}

			// Nested TransactRun: the business error comes back as it is, newTxErk stays unused
			// 嵌套的 TransactRun: 业务错误原样返回, 用不到 newTxErk
			return gormkratos.TransactRun(ctx, db, func(db *gorm.DB) *errors.Error {
				if err := db.Create(&Guest{Name: "dave"}).Error; err != nil {
					return errorspb.ErrorServerDbError("failed to create guest: %v", err)
				}
				return errorspb.ErrorBadRequest("nested validation failed")
			}, newTxErk)
		}, newTxErk)
		erkrequire.Error(t, erk)
		require.True(t, errorspb.IsBadRequest(erk))

		var count int64
		require.NoError(t, db.Model(&Guest{}).Count(&count).Error)
		require.Equal(t, int64(0), count)
	})
}

// Package gormkratos: GORM transaction integration with Kratos
// Provides two-error-return pattern to distinguish business logic errors and database errors
//
// gormkratos: GORM 事务与 Kratos 集成
// 提供双错误返回模式, 区分业务逻辑错误和数据库错误
package gormkratos

import (
	"context"
	"database/sql"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/yylego/must"
	"gorm.io/gorm"
)

// Transaction executes a function in database transaction
// Returns two errors to distinguish transaction errors and business logic errors:
//   - erk: Business logic errors (Kratos errors)
//   - err: Database transaction errors
//
// IMPORTANT:
// When err != nil and erk != nil, erk contains the specific business reason.
// Return erk first since it has more business context (reason and code) than what the raw transaction throws.
//
// Error combinations:
//   - err != nil, erk != nil: Business logic error caused rollback
//   - err != nil, erk == nil: The transaction itself failed, such as begin / commit failing, the context timing out
//   - no err: no erk, both succeeded
//
// Transaction writes no logs: both errors come back unchanged, the code that calls it decides what to log.
//
// Recommended usage pattern (MUST follow this pattern):
//
//	erk, err := gormkratos.Transaction(ctx, db, run)
//	if err != nil {
//	    if erk != nil {
//	        return erk
//	    }
//	    return newTxErk(err) // the project's own function that wraps err into its error code
//	}
//
// Transaction 在数据库事务中执行函数
// 返回两个错误以区分事务错误和业务逻辑错误:
//   - erk: 业务逻辑错误 (Kratos 错误)
//   - err: 数据库事务错误
//
// 重要:
// 当 err != nil 且 erk != nil 时, erk 包含业务层的具体原因.
// 需要优先返回 erk, 因为它比底层事务抛出的错误更有业务错误原因和错误码信息.
//
// 错误组合:
//   - err != nil, erk != nil: 业务逻辑错误导致回滚
//   - err != nil, erk == nil: 事务本身失败, 比如开启、提交失败或上下文超时
//   - err 为空: erk 也必然为空, 两者都成功
//
// Transaction 不记日志: 两个错误都原样返回, 记不记日志由调用方决定.
//
// 推荐用法 (必须遵循此模式):
//
//	erk, err := gormkratos.Transaction(ctx, db, run)
//	if err != nil {
//	    if erk != nil {
//	        return erk
//	    }
//	    return newTxErk(err) // 项目自己的函数，把 err 包成项目的错误码
//	}
func Transaction(
	ctx context.Context,
	db *gorm.DB,
	run func(db *gorm.DB) *errors.Error,
	options ...*sql.TxOptions,
) (erk *errors.Error, err error) {
	// Execute transaction with context and options
	// 使用上下文和选项执行事务
	if err = db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		if erk = run(db); erk != nil {
			return erk // Business errors cause rollback // 业务错误导致回滚
		}
		return nil
	}, options...); err != nil {
		if erk != nil {
			// Business error caused rollback, return both errors
			// 业务错误导致回滚, 返回两个错误
			return erk, err
		}
		// The transaction itself failed, return the cause as it is
		// 事务本身失败, 原样返回原因
		return nil, err
	}

	// Transaction succeeded
	// 事务成功
	return nil, nil
}

// TransactRun runs Transaction and returns one Kratos error, handling the two errors with the nested pattern:
//   - run returns an error: that error comes back as it is
//   - the transaction itself fails: newErk turns the cause into the Kratos error to return
//
// Use Transaction first: TransactRun makes each invocation pass an error-building function that has
// nothing to do with the business steps, and nested transactions repeat it at each depth.
//
//	erk := gormkratos.TransactRun(ctx, db, run, newTxErk) // newTxErk(ctx context.Context, cause error) *errors.Error
//
// newErk gets the same ctx and the cause, so it is the place to write a log when one is wanted.
// newErk must not be nil and must not return nil: TransactRun panics in both cases, since a nil
// result would report a failed transaction as success.
//
// TransactRun 调用 Transaction, 按嵌套写法处理它的两个错误, 只返回一个 Kratos 错误:
//   - run 返回错误: 原样返回这个错误
//   - 事务本身失败: 用 newErk 把原因包成要返回的 Kratos 错误
//
// 优先用 Transaction: TransactRun 每次调用都要传一个跟业务无关的包错误函数, 嵌套事务还要一层层重复
//
// newErk 拿到同一个 ctx 和失败原因, 想记日志就写在这个函数里.
// newErk 不能为空, 也不能返回空: 这两种情况 TransactRun 都会 panic, 因为返回空会把失败的事务报成成功
func TransactRun(
	ctx context.Context,
	db *gorm.DB,
	run func(db *gorm.DB) *errors.Error,
	newErk func(ctx context.Context, cause error) *errors.Error,
	options ...*sql.TxOptions,
) *errors.Error {
	must.True(newErk != nil)

	erk, err := Transaction(ctx, db, run, options...)
	if err != nil {
		if erk != nil {
			return erk
		}
		return must.Full(newErk(ctx, err))
	}
	return nil
}

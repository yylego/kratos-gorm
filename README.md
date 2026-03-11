[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/kratos-gorm/release.yml?branch=main&label=BUILD)](https://github.com/yylego/kratos-gorm/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/kratos-gorm)](https://pkg.go.dev/github.com/yylego/kratos-gorm)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/kratos-gorm/main.svg)](https://coveralls.io/github/yylego/kratos-gorm?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.25+-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/github/release/yylego/kratos-gorm.svg)](https://github.com/yylego/kratos-gorm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/kratos-gorm)](https://goreportcard.com/report/github.com/yylego/kratos-gorm)

# kratos-gorm

GORM transaction integration with Kratos, provides transaction functions with two-error-return pattern.

---

<!-- TEMPLATE (EN) BEGIN: LANGUAGE NAVIGATION -->

## CHINESE README

[中文说明](README.zh.md)

<!-- TEMPLATE (EN) END: LANGUAGE NAVIGATION -->

## Main Features

🎯 **Two-Error Pattern**: Distinguishes business logic errors and database transaction errors
⚡ **Context Support**: Built-in context timeout and cancellation handling
🔄 **Auto Rollback**: Transaction rollback on business logic errors
🌍 **Kratos Integration**: Smooth integration with Kratos microservice framework
📋 **Simple API**: Clean and concise transaction wrap functions

## Install

```bash
go get github.com/yylego/kratos-gorm/gormkratos
```

## Usage

### Basic Transaction

This example shows the simplest use of gormkratos.Transaction.

```go
package main

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
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

type Admin struct {
	ID   uint   `gorm:"primarykey"`
	Name string `gorm:"not null"`
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

func ErrorServerDbError(format string, args ...interface{}) *errors.Error {
	return errors.New(500, "DB_ERROR", fmt.Sprintf(format, args...))
}

func ErrorServerDbTransactionError(format string, args ...interface{}) *errors.Error {
	return errors.New(500, "TRANSACTION_ERROR", fmt.Sprintf(format, args...))
}

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
```

⬆️ **Source:** [Source](internal/demos/demo1x/main.go)

### Transaction Rollback

This example shows auto rollback when business logic returns errors.

```go
package main

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
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

type Guest struct {
	ID   uint   `gorm:"primarykey"`
	Name string `gorm:"not null"`
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
			return ErrorServerDbError("create failed: %v", err)
		}
		zaplog.LOG.Debug("Created guest (then rollback)", zap.Uint("id", guest.ID), zap.String("name", guest.Name))
		return ErrorBadRequest("validation failed")
	})
	zaplog.LOG.Error("Error", zap.Error(erk))

	var count int64
	db.Model(&Guest{}).Count(&count)
	zaplog.LOG.Debug("Guest count post rollback", zap.Int64("count", count))
}

func ErrorServerDbError(format string, args ...interface{}) *errors.Error {
	return errors.New(500, "DB_ERROR", fmt.Sprintf(format, args...))
}

func ErrorBadRequest(format string, args ...interface{}) *errors.Error {
	return errors.New(400, "BAD_REQUEST", fmt.Sprintf(format, args...))
}

func ErrorServerDbTransactionError(format string, args ...interface{}) *errors.Error {
	return errors.New(500, "TRANSACTION_ERROR", fmt.Sprintf(format, args...))
}

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
```

⬆️ **Source:** [Source](internal/demos/demo2x/main.go)

### Multiple Operations

This example shows combining create and update in one atomic transaction.

```go
package main

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
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

type Product struct {
	ID    uint   `gorm:"primarykey"`
	Name  string `gorm:"not null"`
	Price int
}

func main() {
	dsn := fmt.Sprintf("file:db-%s?mode=memory&cache=shared", uuid.New().String())
	db := rese.P1(gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}))
	defer rese.F0(rese.P1(db.DB()).Close)

	must.Done(db.AutoMigrate(&Product{}))

	ctx := context.Background()

	erk := Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		product := &Product{Name: "Laptop", Price: 5000}
		if err := db.Create(product).Error; err != nil {
			return ErrorServerDbError("create failed: %v", err)
		}
		zaplog.LOG.Debug("Created product", zap.Uint("id", product.ID), zap.String("name", product.Name), zap.Int("price", product.Price))

		product.Price = 4500
		if err := db.Updates(product).Error; err != nil {
			return ErrorServerDbError("update failed: %v", err)
		}
		zaplog.LOG.Debug("Updated product", zap.Uint("id", product.ID), zap.String("name", product.Name), zap.Int("price", product.Price))
		return nil
	})
	if erk != nil {
		zaplog.LOG.Error("Error", zap.Error(erk))
	}
}

func ErrorServerDbError(format string, args ...interface{}) *errors.Error {
	return errors.New(500, "DB_ERROR", fmt.Sprintf(format, args...))
}

func ErrorServerDbTransactionError(format string, args ...interface{}) *errors.Error {
	return errors.New(500, "TRANSACTION_ERROR", fmt.Sprintf(format, args...))
}

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
```

⬆️ **Source:** [Source](internal/demos/demo3x/main.go)

## Error Handling

The `gormkratos.Transaction` function returns two errors to help distinguish between different types:

1. **Business Logic Errors** (`erk *errors.Error`): Kratos framework errors from business logic
2. **Database Transaction Errors** (`err error`): Database transaction errors

> **⚠️ IMPORTANT:**
>
> When `err != nil` and `erk != nil`, `erk` contains the specific business reason.
> Return `erk` first since it has more business context (reason and code) than what the raw transaction throws.

### Recommended Usage Pattern

**Always use this pattern:**

```go
erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    // your business logic
    return nil
})
if err != nil {
    if erk != nil {
        return erk
    }
    return YourTransactionError("transaction failed: %v", err)
}
```

### Scenarios

**When err != nil:**

- `erk != nil`: Business logic error caused rollback (use `erk`)
- `erk == nil`: Database commit failed (wrap `err`)

**When err == nil:**

- `erk` also nil, both succeeded

## Examples

### Basic Two-Error Return

**Direct use of gormkratos.Transaction:**

```go
erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    user := &User{Name: "test"}
    if err := db.Create(user).Error; err != nil {
        return errorspb.ErrorServerDbError("create failed: %v", err)
    }
    return nil
})
```

**Check business errors:**

```go
if erk != nil {
    // Handle Kratos business errors
    log.Printf("Business logic failed: %v", erk)
}
```

**Check database errors:**

```go
if err != nil {
    // Handle database transaction errors
    log.Printf("Database transaction failed: %v", err)
}
```

### With Transaction Options

**Set transaction isolation:**

```go
import "database/sql"

erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    // Transaction logic with custom isolation
    return nil
}, &sql.TxOptions{
    Isolation: sql.LevelReadCommitted,
    ReadOnly:  false,
})
```

### Multiple Operations in One Transaction

**Combine create and update:**

```go
erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    product := &Product{Name: "Laptop", Price: 5000}
    if err := db.Create(product).Error; err != nil {
        return ErrorServerDbError("create failed: %v", err)
    }

    product.Price = 4500
    if err := db.Updates(product).Error; err != nil {
        return ErrorServerDbError("update failed: %v", err)
    }
    return nil
})
```

### Context Timeout Handling

**Auto rollback on timeout:**

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    // Long-running operations
    time.Sleep(10 * time.Second) // Exceed timeout
    return nil
})
// err will contain timeout errors
```

<!-- TEMPLATE (EN) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-11-25 03:52:28.131064 +0000 UTC -->

## 📄 License

MIT License - see [LICENSE](LICENSE).

---

## 💬 Contact & Feedback

Contributions are welcome! Report bugs, suggest features, and contribute code:

- 🐛 **Mistake reports?** Open an issue on GitHub with reproduction steps
- 💡 **Fresh ideas?** Create an issue to discuss
- 📖 **Documentation confusing?** Report it so we can improve
- 🚀 **Need new features?** Share the use cases to help us understand requirements
- ⚡ **Performance issue?** Help us optimize through reporting slow operations
- 🔧 **Configuration problem?** Ask questions about complex setups
- 📢 **Follow project progress?** Watch the repo to get new releases and features
- 🌟 **Success stories?** Share how this package improved the workflow
- 💬 **Feedback?** We welcome suggestions and comments

---

## 🔧 Development

New code contributions, follow this process:

1. **Fork**: Fork the repo on GitHub (using the webpage UI).
2. **Clone**: Clone the forked project (`git clone https://github.com/yourname/repo-name.git`).
3. **Navigate**: Navigate to the cloned project (`cd repo-name`)
4. **Branch**: Create a feature branch (`git checkout -b feature/xxx`).
5. **Code**: Implement the changes with comprehensive tests
6. **Testing**: (Golang project) Ensure tests pass (`go test ./...`) and follow Go code style conventions
7. **Documentation**: Update documentation to support client-facing changes
8. **Stage**: Stage changes (`git add .`)
9. **Commit**: Commit changes (`git commit -m "Add feature xxx"`) ensuring backward compatible code
10. **Push**: Push to the branch (`git push origin feature/xxx`).
11. **PR**: Open a merge request on GitHub (on the GitHub webpage) with detailed description.

Please ensure tests pass and include relevant documentation updates.

---

## 🌟 Support

Welcome to contribute to this project via submitting merge requests and reporting issues.

**Project Support:**

- ⭐ **Give GitHub stars** if this project helps you
- 🤝 **Share with teammates** and (golang) programming friends
- 📝 **Write tech blogs** about development tools and workflows - we provide content writing support
- 🌟 **Join the ecosystem** - committed to supporting open source and the (golang) development scene

**Have Fun Coding with this package!** 🎉🎉🎉

<!-- TEMPLATE (EN) END: STANDARD PROJECT FOOTER -->

---

## GitHub Stars

[![Stargazers](https://starchart.cc/yylego/kratos-gorm.svg?variant=adaptive)](https://starchart.cc/yylego/kratos-gorm)

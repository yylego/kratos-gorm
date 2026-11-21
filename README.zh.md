<!-- TEMPLATE (ZH) BEGIN: BADGES -->

[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/kratos-gorm/release.yml?branch=main&label=BUILD)](https://github.com/yylego/kratos-gorm/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/kratos-gorm)](https://pkg.go.dev/github.com/yylego/kratos-gorm)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/kratos-gorm/main.svg)](https://coveralls.io/github/yylego/kratos-gorm?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.27%2B-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/github/release/yylego/kratos-gorm.svg)](https://github.com/yylego/kratos-gorm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/kratos-gorm)](https://goreportcard.com/report/github.com/yylego/kratos-gorm)

<!-- TEMPLATE (ZH) CLOSE: BADGES -->

# kratos-gorm

GORM 事务与 Kratos 集成, 提供双错误返回模式的事务调用函数。

---

<!-- TEMPLATE (ZH) BEGIN: LANGUAGE NAVIGATION -->

## 英文文档

[ENGLISH README](README.md)

<!-- TEMPLATE (ZH) CLOSE: LANGUAGE NAVIGATION -->

## 主要特性

- 🎯 **双错误模式**: 区分业务逻辑错误和数据库事务错误
- ⚡ **上下文支持**: 内置上下文超时和取消处理
- 🔄 **自动回滚**: 业务逻辑错误时的事务自动回滚
- 🌍 **Kratos 集成**: 与 Kratos 微服务框架的顺畅集成
- 📋 **简洁 API**: 干净简洁的事务封装函数

## 安装

```bash
go get github.com/yylego/kratos-gorm/gormkratos
```

## 使用方法

### 基础事务

此示例展示 gormkratos.Transaction 的最简单用法。

```go
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
			return errors.Newf(500, "DB_ERROR", "create failed: %v", err)
		}
		zaplog.LOG.Debug("Created admin", zap.Uint("id", admin.ID), zap.String("name", admin.Name))
		return nil
	})
	if erk != nil {
		zaplog.LOG.Error("Create admin failed", zap.Error(erk))
	}
}

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
```

⬆️ **源码:** [源码](internal/demos/demo1x/main.go)

### 事务回滚

此示例展示业务逻辑返回错误时的自动回滚。

```go
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
```

⬆️ **源码:** [源码](internal/demos/demo2x/main.go)

### 多个操作

此示例展示在一个原子事务中组合创建和更新操作。

```go
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
			return errors.Newf(500, "DB_ERROR", "create failed: %v", err)
		}
		zaplog.LOG.Debug("Created product", zap.Uint("id", product.ID), zap.String("name", product.Name), zap.Int("price", product.Price))

		product.Price = 4500
		if err := db.Updates(product).Error; err != nil {
			return errors.Newf(500, "DB_ERROR", "update failed: %v", err)
		}
		zaplog.LOG.Debug("Updated product", zap.Uint("id", product.ID), zap.String("name", product.Name), zap.Int("price", product.Price))
		return nil
	})
	if erk != nil {
		zaplog.LOG.Error("Save product failed", zap.Error(erk))
	}
}

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
```

⬆️ **源码:** [源码](internal/demos/demo3x/main.go)

### 单个错误返回

此示例展示 `TransactRun`：传入一个包装事务失败的函数，只返回一个错误。推荐的写法仍是 `Transaction`（见第一个示例），这种写法只在确实只想拿一个错误时用。

```go
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
```

⬆️ **源码:** [源码](internal/demos/demo4x/main.go)

## 错误处理

`gormkratos.Transaction` 函数返回两个错误以帮助区分不同类型：

1. **业务逻辑错误** (`erk *errors.Error`): 来自业务逻辑的 Kratos 框架错误
2. **数据库事务错误** (`err error`): 数据库事务错误

> **⚠️ 重要:**
>
> 当 `err != nil` 且 `erk != nil` 时, `erk` 包含业务层的具体原因.
> 需要优先返回 `erk`, 因为它比底层事务抛出的错误更有业务错误原因和错误码信息.

本包不记日志：两个错误都原样返回，记不记日志由调用方决定。业务错误是预期内的结果，一个 400 的校验失败不会被本包记成一条 ERROR 日志。

### 最佳实践

**始终使用此模式:**

```go
erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    // 你的业务逻辑
    return nil
})
if err != nil {
    if erk != nil {
        return erk
    }
    return newTxErk(err) // newTxErk：项目自己的函数，把 err 包成项目的错误码
}
```

这是在大量调用中摸索出来的写法，好在三点：

- **错误处理集中在一个块里**：外层 `if err != nil` 表示"这次事务失败"，失败的处理全在这个块里，成功的路径在块外。
- **块里分清两类错误**：`erk != nil` 是传进去的函数返回的业务错误，原样返回；`erk == nil` 是事务本身失败（开启、提交、上下文超时），由调用方包成自己项目的错误码。
- **调用时不用额外传参**：库不知道调用方的错误枚举，也不要求每次调用都传一个包错误的函数；怎么包由调用方在这个块里自己决定。

### 单错误写法

`TransactRun` 只返回一个错误：多传一个函数，告诉它事务本身失败时包成什么错误，内部就是调用 `Transaction` 再套上面的嵌套写法。这种写法不如上面的 `Transaction` 好，只在确实只想拿一个错误时用。

```go
erk := gormkratos.TransactRun(ctx, db, run, newTxErk) // newTxErk(ctx context.Context, cause error) *errors.Error，想记日志就写在这个函数里
if erk != nil {
    return erk
}
```

完整可运行的例子见 [demo4x](internal/demos/demo4x/main.go)。

它不如 `Transaction` 的地方：每次调用都要多带一个跟业务无关的参数，嵌套事务还要一层层重复，读的人会停下来琢磨这个函数是干什么的。

### 淘汰写法

- **先判断 `erk`、再判断 `err`，两个判断平铺**：结果一样，但把失败处理拆成了两个并列的块，读的人要自己想明白两者的关系；嵌套写法把失败处理整个包在一个块里。

### 场景

**当 err != nil:**

- `erk != nil`: 业务逻辑错误导致回滚 (使用 `erk`)
- `erk == nil`: 事务本身失败, 比如开启、提交失败或上下文超时 (包装 `err`)

**当 err == nil:**

- `erk` 也是 nil，两者都成功

## 示例

### 基础双错误返回

**直接使用 gormkratos.Transaction:**

```go
erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    student := &Student{Name: "test"}
    if err := db.Create(student).Error; err != nil {
        return errors.Newf(500, "DB_ERROR", "创建失败: %v", err)
    }
    return nil
})
```

**按嵌套写法处理两个错误 (见最佳实践):**

```go
if err != nil {
    if erk != nil {
        return erk // 传进去的函数返回的业务错误
    }
    return newTxErk(err) // 事务本身失败
}
```

### 使用事务选项

**设置事务隔离级别:**

```go
import "database/sql"

erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    // 自定义隔离级别的事务逻辑
    return nil
}, &sql.TxOptions{
    Isolation: sql.LevelReadCommitted,
    ReadOnly:  false,
})
```

### 单个事务中的多个操作

**组合创建和更新:**

```go
erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    product := &Product{Name: "Laptop", Price: 5000}
    if err := db.Create(product).Error; err != nil {
        return errors.Newf(500, "DB_ERROR", "创建失败: %v", err)
    }

    product.Price = 4500
    if err := db.Updates(product).Error; err != nil {
        return errors.Newf(500, "DB_ERROR", "更新失败: %v", err)
    }
    return nil
})
```

### 上下文超时处理

**超时时自动回滚:**

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
    // 长时间运行的操作
    time.Sleep(10 * time.Second) // 超过超时时间
    return nil
})
// err 将包含超时错误
```

<!-- TEMPLATE (ZH) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-11-25 03:52:28.131064 +0000 UTC -->

## 📄 许可证类型

MIT 许可证 - 详见 [LICENSE](LICENSE)。

---

## 💬 联系与反馈

非常欢迎贡献代码！报告 BUG、建议功能、贡献代码：

- 🐛 **问题报告？** 在 GitHub 上提交问题并附上重现步骤
- 💡 **新颖思路？** 创建 issue 讨论
- 📖 **文档疑惑？** 报告问题，帮助我们完善文档
- 🚀 **需要功能？** 分享使用场景，帮助理解需求
- ⚡ **性能瓶颈？** 报告慢操作，协助解决性能问题
- 🔧 **配置困扰？** 询问复杂设置的相关问题
- 📢 **关注进展？** 关注仓库以获取新版本和功能
- 🌟 **成功案例？** 分享这个包如何改善工作流程
- 💬 **反馈意见？** 欢迎提出建议和意见

---

## 🔧 代码贡献

新代码贡献，请遵循此流程：

1. **Fork**：在 GitHub 上 Fork 仓库（使用网页界面）
2. **克隆**：克隆 Fork 的项目（`git clone https://github.com/yourname/repo-name.git`）
3. **导航**：进入克隆的项目（`cd repo-name`）
4. **分支**：创建功能分支（`git checkout -b feature/xxx`）
5. **编码**：实现您的更改并编写全面的测试
6. **测试**：（Golang 项目）确保测试通过（`go test ./...`）并遵循 Go 代码风格约定
7. **文档**：面向用户的更改需要更新文档
8. **暂存**：暂存更改（`git add .`）
9. **提交**：提交更改（`git commit -m "Add feature xxx"`）确保向后兼容的代码
10. **推送**：推送到分支（`git push origin feature/xxx`）
11. **PR**：在 GitHub 上打开 Merge Request（在 GitHub 网页上）并提供详细描述

请确保测试通过并包含相关的文档更新。

---

## 🌟 项目支持

非常欢迎通过提交 Merge Request 和报告问题来贡献此项目。

**项目支持：**

- ⭐ **给予星标**如果项目对您有帮助
- 🤝 **分享项目**给团队成员和（golang）编程朋友
- 📝 **撰写博客**关于开发工具和工作流程 - 我们提供写作支持
- 🌟 **加入生态** - 致力于支持开源和（golang）开发场景

**祝你用这个包编程愉快！** 🎉🎉🎉

<!-- TEMPLATE (ZH) CLOSE: STANDARD PROJECT FOOTER -->

# 从 xorm 迁移到 gorm 完成

## 迁移概述

本项目已成功从 xorm ORM 迁移到 gorm ORM。

## 主要变更

### 1. 依赖更新 (go.mod)
- 移除: `github.com/go-xorm/xorm` 和 `github.com/go-xorm/core`
- 添加: 
  - `gorm.io/gorm v1.25.12`
  - `gorm.io/driver/mysql v1.5.7`
  - `gorm.io/driver/postgres v1.5.11`
- 更新: `github.com/go-sql-driver/mysql` 和 `github.com/lib/pq` 到最新版本

### 2. 模型文件更新

所有模型文件已从 xorm 标签迁移到 gorm 标签：

#### model.go
- 使用 `gorm.DB` 替代 `xorm.Engine`
- 使用 gorm 的驱动程序 (mysql.Open, postgres.Open)
- 使用 gorm 的配置和命名策略
- 保持了连接池配置和 keepalive 机制

#### user.go
- 标签从 `xorm` 改为 `gorm`
- `Created/Updated` 字段改为 `CreatedAt/UpdatedAt` 并使用 `autoCreateTime/autoUpdateTime`
- 所有数据库操作使用 gorm API

#### host.go
- 标签迁移到 gorm
- 查询方法使用 gorm 链式调用

#### task.go
- 标签迁移到 gorm
- `Deleted` 字段改为 `DeletedAt gorm.DeletedAt` 支持软删除
- 复杂查询使用 gorm 的 Table、Joins、Group 等方法

#### task_host.go
- 标签迁移到 gorm
- JOIN 查询使用 gorm 语法

#### task_log.go
- 标签迁移到 gorm
- 时间字段使用 gorm 的自动时间戳

#### login_log.go
- 标签迁移到 gorm
- `Created` 改为 `CreatedAt`

#### setting.go
- 标签迁移到 gorm
- 所有 CRUD 操作使用 gorm API

#### migration.go
- 使用 `gorm.DB` 替代 `xorm.Session`
- 使用 `Db.AutoMigrate()` 创建表
- 使用 `Db.Migrator()` 进行表结构变更
- 使用 `Db.Transaction()` 处理事务

## 主要 API 对应关系

| xorm | gorm |
|------|------|
| `Db.Insert()` | `Db.Create()` |
| `Db.Id(id).Get()` | `Db.First(&model, id)` |
| `Db.Where().Find()` | `Db.Where().Find()` |
| `Db.Table().ID(id).Update()` | `Db.Model().Where("id = ?", id).Updates()` |
| `Db.Id(id).Delete()` | `Db.Delete(&model, id)` |
| `Db.Count()` | `Db.Model().Count(&count)` |
| `Db.Desc().Limit().Find()` | `Db.Order("id DESC").Limit().Find()` |
| `session.Begin()` | `Db.Transaction(func(tx *gorm.DB) error {})` |
| `Db.Sync2()` | `Db.AutoMigrate()` |
| `Db.IsTableExist()` | `Db.Migrator().HasTable()` |

## 字段标签变更

| xorm | gorm |
|------|------|
| `xorm:"pk autoincr"` | `gorm:"primaryKey;autoIncrement"` |
| `xorm:"varchar(32) notnull"` | `gorm:"type:varchar(32);not null"` |
| `xorm:"unique"` | `gorm:"uniqueIndex"` |
| `xorm:"index"` | `gorm:"index"` |
| `xorm:"default 0"` | `gorm:"default:0"` |
| `xorm:"created"` | `gorm:"autoCreateTime"` |
| `xorm:"updated"` | `gorm:"autoUpdateTime"` |
| `xorm:"deleted"` | `gorm.DeletedAt` (软删除) |
| `xorm:"-"` | `gorm:"-"` |

## 测试建议

迁移完成后，建议进行以下测试：

1. **数据库连接测试**
   - 测试 MySQL 连接
   - 测试 PostgreSQL 连接

2. **CRUD 操作测试**
   - 用户管理（创建、查询、更新、删除）
   - 任务管理
   - 主机管理
   - 日志查询

3. **复杂查询测试**
   - 任务列表（带分页、筛选）
   - 任务与主机的关联查询
   - 统计查询

4. **事务测试**
   - 数据库升级流程
   - 批量操作

5. **迁移测试**
   - 首次安装
   - 版本升级

## 注意事项

1. **时间字段**: gorm 使用 `CreatedAt/UpdatedAt` 作为约定字段名，自动处理时间戳
2. **软删除**: 使用 `gorm.DeletedAt` 类型自动支持软删除
3. **事务**: gorm 使用 `Transaction()` 方法，自动处理提交和回滚
4. **表前缀**: 通过 `NamingStrategy` 配置表前缀
5. **日志**: 开发模式下自动开启 SQL 日志

## 兼容性

- Go 1.23.0+
- MySQL 5.7+
- PostgreSQL 9.6+

## 回滚方案

如需回滚到 xorm，请：
1. 恢复 `go.mod` 中的 xorm 依赖
2. 恢复所有 `internal/models/*.go` 文件
3. 运行 `go mod tidy`

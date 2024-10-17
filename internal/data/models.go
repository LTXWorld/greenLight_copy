package data

// 用于作为一个统一的入口点，用于管理和组织所有数据模型，app启动时可以将所有的数据模型注入到app中
import (
	"database/sql"
	"errors"
)

// 定义一个自定义错误，当Get寻找一个不存在于数据库中的movie时会返回
var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

// 新建一个Models struct 包裹着MovieModel,可以向其中添加其他模型
type Models struct {
	Movies      MovieModel
	Users       UserModel
	Tokens      TokenModel
	Permissions PermissionModel
}

// 为了方便使用，写一个New方法初始化一个Modles结构体
func NewModels(db *sql.DB) Models {
	return Models{
		Movies:      MovieModel{DB: db},
		Users:       UserModel{DB: db},
		Tokens:      TokenModel{DB: db},
		Permissions: PermissionModel{DB: db},
	}
}

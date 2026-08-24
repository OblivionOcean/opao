package model

// GStmtUser 三方基准共用模型（gorm 直用 / gen 代码生成）
type GStmtUser struct {
	ID   int64 `gorm:"primaryKey;autoIncrement"`
	Name string
	Age  int
}

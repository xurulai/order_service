package model

// ORM
// struct -> table

// Order 表示一个订单的结构体，用于映射数据库中的订单表。
type Order struct {
	BaseModel             // 嵌入默认的7个字段：ID、创建时间、更新时间、创建者、更新者、版本号、是否删除
	OrderId       int64  `gorm:"column:order_id;type:bigint(20);not_null"` // 订单ID，唯一标识一个订单。
	UserId        int64  `gorm:"column:user_id;type:bigint(20);not_null"`  // 用户ID，标识订单所属的用户。
	PayAmount     int64  `gorm:"column:pay_amount;type:bigint(20);not_null;default:0"` // 支付金额，表示订单的总金额（单位：分）。
	Status        string  `gorm:"column:receive_name;type:varchar(255);not_null;default:''"`           // 订单状态，例如：100-创建订单/待支付，200-已支付，300-交易关闭，400-完成。
	ReceiveAddress string `gorm:"column:receive_address;type:varchar(128);not_null;default:''"` // 收货地址，用户指定的收货地址。
	ReceiveName   string `gorm:"column:receive_name;type:varchar(128);not_null;default:''"`     // 收货人姓名，用户指定的收货人姓名。
	ReceivePhone  string `gorm:"column:receive_phone;type:varchar(11);not_null;default:''"`     // 收货人电话，用户指定的收货人电话。
}

// TableName 声明表名
// 为 Order 结构体指定数据库表名，确保 GORM 在操作时使用正确的表名。
func (Order) TableName() string {
	return "xx_order" // 数据库中对应的表名为 "xx_order"。
}

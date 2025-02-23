package model

type OrderDetail struct {
	BaseModel        // 嵌入默认的7个字段：ID、创建时间、更新时间、创建者、更新者、版本号、是否删除
	OrderId   int64  `gorm:"column:order_id;type:bigint(20);not_null"`             // 订单ID，关联的订单。
	GoodsId   int64  `gorm:"column:goods_id;type:bigint(20);not_null"`             // 商品ID，订单中包含的商品。
	UserId    int64  `gorm:"column:user_id;type:bigint(20);not_null"`              // 用户ID，订单所属的用户。
	Num       int64  `gorm:"column:num;type:bigint(20);not_null"`                  // 商品数量，用户购买的商品数量。
	Title     string `gorm:"column:title;type:varchar(255);not_null;default:''"`   // 商品名称，商品的标题。
	Price     string  `gorm:"column:price;type:varchar(20);not_null;default:0"`     // 销售价格（单位：分）。
	Brief     string `gorm:"column:brief;type:varchar(255);not_null;default:''"`   // 商品简介，商品的简要描述。
	PayAmount int64  `gorm:"column:pay_amount;type:bigint(20);not_null;default:0"` // 支付金额（单位：分），实际支付的金额。
}

func (OrderDetail) TableName() string {
	return "xx_order_detail"
}

package snowflake

import (
	"errors"
	"time"

	sf "github.com/bwmarrin/snowflake" // 导入 snowflake 库
)

const (
	_dafaultStartTime = "2020-12-31" // 默认的起始时间，用于计算时间戳偏移
)

var node *sf.Node // 全局的 Snowflake 节点实例

// Init 初始化 Snowflake 算法组件
// startTime: 自定义的起始时间（格式为 "2006-01-02"）
// machineID: 机器 ID，用于区分不同实例
func Init(startTime string, machineID int64) (err error) {
	// 检查机器 ID 是否有效
	if machineID < 0 {
		return errors.New("snowflake need machineID")
	}

	// 如果未指定起始时间，则使用默认值
	if len(startTime) == 0 {
		startTime = _dafaultStartTime
	}

	// 解析起始时间
	var st time.Time
	st, err = time.Parse("2006-01-02", startTime)
	if err != nil {
		return // 如果解析失败，返回错误
	}

	// 设置 Snowflake 的起始时间戳偏移
	sf.Epoch = st.UnixNano() / 1000000 // 将起始时间转换为毫秒级时间戳
	node, err = sf.NewNode(machineID) // 创建 Snowflake 节点实例
	return
}

// GenID 生成一个 Snowflake ID（int64 类型）
func GenID() int64 {
	return node.Generate().Int64() // 调用 Snowflake 节点的 Generate 方法生成 ID
}

// GenIDStr 生成一个 Snowflake ID（字符串类型）
func GenIDStr() string {
	return node.Generate().String() // 调用 Snowflake 节点的 Generate 方法生成 ID，并转换为字符串
}         
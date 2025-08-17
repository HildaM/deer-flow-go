package checkpoint

import (
	"context"

	"github.com/cloudwego/eino/compose"
)

// DeerCheckPoint DeerGo的全局状态存储点，
// 实现CheckPointStore接口，用checkPointID进行索引
// 此处粗略使用map实现，工程上可以用工业存储组件实现
type checkpoint struct {
	buf map[string][]byte // map映射存储
}

func (c *checkpoint) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	data, ok := c.buf[checkPointID]
	return data, ok, nil
}

func (c *checkpoint) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	c.buf[checkPointID] = checkPoint
	return nil
}

// 创建一个全局状态存储点实例并返回
var checkpointImpl = checkpoint{
	buf: make(map[string][]byte),
}

// NewCheckPoint 创建一个全局状态存储点实例并返回
func NewCheckPoint() compose.CheckPointStore {
	return &checkpointImpl
}

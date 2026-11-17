package kanban

import "context"

// 本文件只在测试时编译，把包内的未导出能力暴露给外部测试包 kanban_test。

// PublishForTest 直接向唤醒器广播，用来测试订阅和观察者。
func PublishForTest(h *Hub, p Published) error {
	return h.publish(context.Background(), p)
}

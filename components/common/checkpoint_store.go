package common

import (
	"context"
	"sync"
)

// InMemoryCheckPointStore 内存检查点存储实现
// 提供线程安全的检查点数据存储和检索功能
type InMemoryCheckPointStore struct {
	m sync.Map
}

// NewInMemoryCheckPointStore 创建新的内存检查点存储
func NewInMemoryCheckPointStore() *InMemoryCheckPointStore {
	return &InMemoryCheckPointStore{}
}

// Get 获取检查点数据
func (s *InMemoryCheckPointStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	if v, ok := s.m.Load(checkPointID); ok {
		return v.([]byte), true, nil
	}
	return nil, false, nil
}

// Set 设置检查点数据
func (s *InMemoryCheckPointStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	s.m.Store(checkPointID, checkPoint)
	return nil
}

// Clear 清除所有检查点数据
func (s *InMemoryCheckPointStore) Clear() {
	s.m.Range(func(key, value interface{}) bool {
		s.m.Delete(key)
		return true
	})
}

// Size 返回存储的检查点数量
func (s *InMemoryCheckPointStore) Size() int {
	count := 0
	s.m.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
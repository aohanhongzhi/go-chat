package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type Sequence struct {
	redis *redis.Client
}

func NewSequence(redis *redis.Client) *Sequence {
	return &Sequence{redis: redis}
}

func (s *Sequence) Redis() *redis.Client {
	return s.redis
}

func (s *Sequence) Name(userId int, receiverId int) string {

	if userId == 0 {
		return fmt.Sprintf("im:sequence:msg:%d", receiverId)
	}

	if receiverId < userId {
		receiverId, userId = userId, receiverId
	}

	return fmt.Sprintf("im:sequence:msg:%d_%d", userId, receiverId)
}

// Init 初始化发号器
func (s *Sequence) Init(ctx context.Context, userId int, receiverId int, value int64) error {
	return s.redis.SetEX(ctx, s.Name(userId, receiverId), value, 12*time.Hour).Err()
}

// Get 获取消息时序ID
func (s *Sequence) Get(ctx context.Context, userId int, receiverId int) int64 {

	name := s.Name(userId, receiverId)

	return s.redis.Incr(ctx, name).Val()
}

// BatchGet 批量获取消息时序ID
func (s *Sequence) BatchGet(ctx context.Context, userId int, receiverId int, num int64) []int64 {

	value := s.redis.IncrBy(ctx, s.Name(userId, receiverId), num).Val()

	items := make([]int64, 0, num)
	for i := num; i > 0; i-- {
		items = append(items, int64(value-i+1))
	}

	return items
}

package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
)

type RedisSessionOption func(*redisSessionConfig)

type redisSessionConfig struct {
	password string
	db       int
	ttl      time.Duration
}

func WithRedisPassword(password string) RedisSessionOption {
	return func(c *redisSessionConfig) { c.password = password }
}

func WithRedisDB(db int) RedisSessionOption {
	return func(c *redisSessionConfig) { c.db = db }
}

func WithRedisTTL(ttl time.Duration) RedisSessionOption {
	return func(c *redisSessionConfig) { c.ttl = ttl }
}

type RedisSession struct {
	client *redis.Client
	key    string
	ttl    time.Duration
}

func NewRedisSession(addr, key string, opts ...RedisSessionOption) *RedisSession {
	cfg := redisSessionConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.password,
		DB:       cfg.db,
	})

	return &RedisSession{
		client: client,
		key:    key,
		ttl:    cfg.ttl,
	}
}

func (s *RedisSession) Save(ctx context.Context, mem memory.MemoryBase) error {
	msgs := mem.GetMessages()
	data, err := json.Marshal(msgs)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}

	if s.ttl > 0 {
		return s.client.Set(ctx, s.key, data, s.ttl).Err()
	}
	return s.client.Set(ctx, s.key, data, 0).Err()
}

func (s *RedisSession) Load(ctx context.Context, mem memory.MemoryBase) error {
	data, err := s.client.Get(ctx, s.key).Bytes()
	if err != nil {
		return fmt.Errorf("get from redis: %w", err)
	}

	var msgs []*message.Msg
	if err := json.Unmarshal(data, &msgs); err != nil {
		return fmt.Errorf("unmarshal messages: %w", err)
	}

	if err := mem.Add(ctx, msgs...); err != nil {
		return fmt.Errorf("add messages to memory: %w", err)
	}
	return nil
}

func (s *RedisSession) Close() error {
	return s.client.Close()
}

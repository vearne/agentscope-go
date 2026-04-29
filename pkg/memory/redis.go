package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vearne/agentscope-go/pkg/message"
)

const (
	defaultSessionID = "default_session"
	defaultUserID     = "default_user"
)

// RedisMemory is the Redis-based implementation of memory storage.
// It supports session and user context isolation, with optional key prefix and TTL.
type RedisMemory struct {
	client   *redis.Client
	sessionID string
	userID    string
	keyPrefix string
	keyTTL    time.Duration
}

// RedisMemoryOption is a function type for configuring RedisMemory.
type RedisMemoryOption func(*RedisMemory)

// WithSessionID sets the session ID for the memory.
func WithSessionID(sessionID string) RedisMemoryOption {
	return func(rm *RedisMemory) {
		rm.sessionID = sessionID
	}
}

// WithRedisUserID sets the user ID for the memory.
func WithRedisUserID(userID string) RedisMemoryOption {
	return func(rm *RedisMemory) {
		rm.userID = userID
	}
}

// WithKeyPrefix sets the key prefix for all Redis keys.
func WithKeyPrefix(prefix string) RedisMemoryOption {
	return func(rm *RedisMemory) {
		rm.keyPrefix = prefix
	}
}

// WithKeyTTL sets the TTL for all Redis keys.
func WithKeyTTL(ttl time.Duration) RedisMemoryOption {
	return func(rm *RedisMemory) {
		rm.keyTTL = ttl
	}
}

// NewRedisMemory creates a new Redis-based memory storage.
func NewRedisMemory(client *redis.Client, opts ...RedisMemoryOption) *RedisMemory {
	rm := &RedisMemory{
		client:    client,
		sessionID: defaultSessionID,
		userID:    defaultUserID,
	}

	for _, opt := range opts {
		opt(rm)
	}

	return rm
}

// GetClient returns the underlying Redis client.
func (rm *RedisMemory) GetClient() *redis.Client {
	return rm.client
}

// sessionKey returns the Redis key for the session message list.
func (rm *RedisMemory) sessionKey() string {
	return fmt.Sprintf("%suser_id:%s:session:%s:messages", rm.keyPrefix, rm.userID, rm.sessionID)
}

// markKey returns the Redis key for a specific mark.
func (rm *RedisMemory) markKey(mark string) string {
	return fmt.Sprintf("%suser_id:%s:session:%s:mark:%s", rm.keyPrefix, rm.userID, rm.sessionID, mark)
}

// messageKey returns the Redis key for a specific message.
func (rm *RedisMemory) messageKey(msgID string) string {
	return fmt.Sprintf("%suser_id:%s:session:%s:msg:%s", rm.keyPrefix, rm.userID, rm.sessionID, msgID)
}

// marksIndexKey returns the Redis key for the marks index set.
func (rm *RedisMemory) marksIndexKey() string {
	return fmt.Sprintf("%suser_id:%s:session:%s:marks_index", rm.keyPrefix, rm.userID, rm.sessionID)
}

// refreshTTL refreshes the TTL for all session keys if keyTTL is set.
func (rm *RedisMemory) refreshTTL(ctx context.Context) error {
	if rm.keyTTL == 0 {
		return nil
	}

	pattern := fmt.Sprintf("%suser_id:%s:session:%s:*", rm.keyPrefix, rm.userID, rm.sessionID)
	iter := rm.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := rm.client.Expire(ctx, iter.Val(), rm.keyTTL).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// getAllMarkKeys returns all mark keys in the current session.
func (rm *RedisMemory) getAllMarkKeys(ctx context.Context) ([]string, error) {
	marksIndexKey := rm.marksIndexKey()

	marks, err := rm.client.SMembers(ctx, marksIndexKey).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	if len(marks) > 0 {
		markKeys := make([]string, 0, len(marks))
		for _, mark := range marks {
			markKeys = append(markKeys, rm.markKey(mark))
		}
		return markKeys, nil
	}

	sessionExists, err := rm.client.Exists(ctx, rm.sessionKey()).Result()
	if err != nil {
		return nil, err
	}
	if sessionExists == 0 {
		return []string{}, nil
	}

	pattern := rm.markKey("*")
	var markKeys []string
	iter := rm.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		markKeys = append(markKeys, iter.Val())
	}

	if len(markKeys) > 0 {
		pipe := rm.client.Pipeline()
		for _, key := range markKeys {
			pipe.SAdd(ctx, marksIndexKey, key[len(rm.markKey("")):])
		}
		_, _ = pipe.Exec(ctx)
	}

	return markKeys, iter.Err()
}

// Add adds message(s) into the memory storage.
func (rm *RedisMemory) Add(ctx context.Context, msgs ...*message.Msg) error {
	return rm.AddWithMarks(ctx, msgs, nil)
}

// AddWithMarks adds message(s) into the memory storage with specified marks.
func (rm *RedisMemory) AddWithMarks(ctx context.Context, msgs []*message.Msg, marks []string) error {
	if len(msgs) == 0 {
		return nil
	}

	pipe := rm.client.Pipeline()

	for _, msg := range msgs {
		msgData, err := json.Marshal(msg)
		if err != nil {
			return err
		}

		pipe.RPush(ctx, rm.sessionKey(), msg.ID)
		pipe.Set(ctx, rm.messageKey(msg.ID), msgData, 0)

		for _, mark := range marks {
			pipe.RPush(ctx, rm.markKey(mark), msg.ID)
			pipe.SAdd(ctx, rm.marksIndexKey(), mark)
		}
	}

	if rm.keyTTL > 0 {
		pipe.Expire(ctx, rm.sessionKey(), rm.keyTTL)
		for _, msg := range msgs {
			pipe.Expire(ctx, rm.messageKey(msg.ID), rm.keyTTL)
		}
		for _, mark := range marks {
			pipe.Expire(ctx, rm.markKey(mark), rm.keyTTL)
		}
		pipe.Expire(ctx, rm.marksIndexKey(), rm.keyTTL)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetMessages returns all messages without filtering.
func (rm *RedisMemory) GetMessages() []*message.Msg {
	ctx := context.Background()
	msgs, _ := rm.GetMemory(ctx, "", "", false)
	return msgs
}

// GetMemory retrieves messages with optional mark filtering.
func (rm *RedisMemory) GetMemory(ctx context.Context, mark string, excludeMark string, prependSummary bool) ([]*message.Msg, error) {
	var msgIDs []string

	if mark == "" {
		msgIDs, _ = rm.client.LRange(ctx, rm.sessionKey(), 0, -1).Result()
	} else {
		msgIDs, _ = rm.client.LRange(ctx, rm.markKey(mark), 0, -1).Result()
	}

	if excludeMark != "" {
		excludeMsgIDs, _ := rm.client.LRange(ctx, rm.markKey(excludeMark), 0, -1).Result()
		excludeSet := make(map[string]struct{})
		for _, id := range excludeMsgIDs {
			excludeSet[id] = struct{}{}
		}
		filtered := make([]string, 0, len(msgIDs))
		for _, id := range msgIDs {
			if _, exists := excludeSet[id]; !exists {
				filtered = append(filtered, id)
			}
		}
		msgIDs = filtered
	}

	var messages []*message.Msg
	if len(msgIDs) > 0 {
		msgKeys := make([]string, 0, len(msgIDs))
		for _, id := range msgIDs {
			msgKeys = append(msgKeys, rm.messageKey(id))
		}

		msgDataList, _ := rm.client.MGet(ctx, msgKeys...).Result()
		for _, data := range msgDataList {
			if data != nil {
				var msg message.Msg
				if strData, ok := data.(string); ok {
					if err := json.Unmarshal([]byte(strData), &msg); err == nil {
						messages = append(messages, &msg)
					}
				}
			}
		}
	}

	_ = rm.refreshTTL(ctx)

	if prependSummary {
		summary, _ := rm.client.Get(ctx, rm.compressedSummaryKey()).Result()
		if summary != "" {
			result := make([]*message.Msg, 0, len(messages)+1)
			result = append(result, message.NewMsg("user", summary, "user"))
			result = append(result, messages...)
			return result, nil
		}
	}

	return messages, nil
}

// compressedSummaryKey returns the Redis key for the compressed summary.
func (rm *RedisMemory) compressedSummaryKey() string {
	return fmt.Sprintf("%suser_id:%s:session:%s:compressed_summary", rm.keyPrefix, rm.userID, rm.sessionID)
}

// Clear clears all messages from the storage.
func (rm *RedisMemory) Clear(ctx context.Context) error {
	msgIDs, _ := rm.client.LRange(ctx, rm.sessionKey(), 0, -1).Result()

	markKeys, err := rm.getAllMarkKeys(ctx)
	if err != nil {
		return err
	}

	pipe := rm.client.Pipeline()

	for _, msgID := range msgIDs {
		pipe.Del(ctx, rm.messageKey(msgID))
	}

	pipe.Del(ctx, rm.sessionKey())

	for _, markKey := range markKeys {
		pipe.Del(ctx, markKey)
	}

	pipe.Del(ctx, rm.marksIndexKey())
	pipe.Del(ctx, rm.compressedSummaryKey())

	_, err = pipe.Exec(ctx)
	return err
}

// Delete removes message(s) from the storage by their IDs.
func (rm *RedisMemory) Delete(ctx context.Context, msgIDs []string) (int, error) {
	if len(msgIDs) == 0 {
		return 0, nil
	}

	markKeys, err := rm.getAllMarkKeys(ctx)
	if err != nil {
		return 0, err
	}

	pipe := rm.client.Pipeline()

	for _, msgID := range msgIDs {
		pipe.LRem(ctx, rm.sessionKey(), 0, msgID)
		pipe.Del(ctx, rm.messageKey(msgID))

		for _, markKey := range markKeys {
			pipe.LRem(ctx, markKey, 0, msgID)
		}
	}

	results, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	removedCount := 0
	opsPerMsg := 2 + len(markKeys)
	for i := 0; i < len(results); i += opsPerMsg {
		if lremResult, ok := results[i].(*redis.IntCmd); ok {
			if count, err := lremResult.Result(); err == nil && count > 0 {
				removedCount++
			}
		}
	}

	return removedCount, nil
}

// DeleteByMark removes messages from the memory by their marks.
func (rm *RedisMemory) DeleteByMark(ctx context.Context, marks []string) (int, error) {
	if len(marks) == 0 {
		return 0, nil
	}

	totalRemoved := 0

	for _, mark := range marks {
		markKey := rm.markKey(mark)
		msgIDs, _ := rm.client.LRange(ctx, markKey, 0, -1).Result()

		if len(msgIDs) > 0 {
			removed, err := rm.Delete(ctx, msgIDs)
			if err != nil {
				return 0, err
			}
			totalRemoved += removed
		}

		_ = rm.client.Del(ctx, markKey)
		_ = rm.client.SRem(ctx, rm.marksIndexKey(), mark)
	}

	_ = rm.refreshTTL(ctx)

	return totalRemoved, nil
}

// Size returns the number of messages in the storage.
func (rm *RedisMemory) Size() int {
	ctx := context.Background()
	size, _ := rm.client.LLen(ctx, rm.sessionKey()).Result()
	_ = rm.refreshTTL(ctx)
	return int(size)
}

// ToStrList converts messages to a list of strings.
func (rm *RedisMemory) ToStrList() []string {
	ctx := context.Background()
	msgs, _ := rm.GetMemory(ctx, "", "", false)
	result := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, msg.GetTextContent())
	}
	return result
}

// UpdateCompressedSummary updates the compressed summary.
func (rm *RedisMemory) UpdateCompressedSummary(ctx context.Context, summary string) error {
	err := rm.client.Set(ctx, rm.compressedSummaryKey(), summary, 0).Err()
	if err != nil {
		return err
	}
	if rm.keyTTL > 0 {
		_ = rm.client.Expire(ctx, rm.compressedSummaryKey(), rm.keyTTL)
	}
	return nil
}

// UpdateMessagesMark updates marks of messages.
func (rm *RedisMemory) UpdateMessagesMark(ctx context.Context, newMark string, oldMark string, msgIDs []string) (int, error) {
	sourceKey := rm.sessionKey()
	if oldMark != "" {
		sourceKey = rm.markKey(oldMark)
	}

	markMsgIDs, _ := rm.client.LRange(ctx, sourceKey, 0, -1).Result()

	removingAllFromOldMark := oldMark != "" && (len(msgIDs) == 0 || allInSet(markMsgIDs, msgIDs))

	if len(msgIDs) > 0 {
		msgIDSet := make(map[string]struct{})
		for _, id := range msgIDs {
			msgIDSet[id] = struct{}{}
		}
		filtered := make([]string, 0, len(markMsgIDs))
		for _, id := range markMsgIDs {
			if _, exists := msgIDSet[id]; exists {
				filtered = append(filtered, id)
			}
		}
		markMsgIDs = filtered
	}

	if len(markMsgIDs) == 0 {
		return 0, nil
	}

	existingIDsSet := make(map[string]struct{})
	var newMarkKey string
	if newMark != "" {
		newMarkKey = rm.markKey(newMark)
		existingIDs, _ := rm.client.LRange(ctx, newMarkKey, 0, -1).Result()
		for _, id := range existingIDs {
			existingIDsSet[id] = struct{}{}
		}
	}

	pipe := rm.client.Pipeline()
	updatedCount := 0

	for _, msgID := range markMsgIDs {
		if oldMark != "" {
			pipe.LRem(ctx, rm.markKey(oldMark), 0, msgID)
		}

		if newMark != "" {
			if _, exists := existingIDsSet[msgID]; !exists {
				pipe.RPush(ctx, newMarkKey, msgID)
				existingIDsSet[msgID] = struct{}{}
				pipe.SAdd(ctx, rm.marksIndexKey(), newMark)
			}
		}

		if oldMark != "" || newMark != "" {
			updatedCount++
		}
	}

	if oldMark != "" && removingAllFromOldMark {
		pipe.Del(ctx, rm.markKey(oldMark))
		pipe.SRem(ctx, rm.marksIndexKey(), oldMark)
	}

	if rm.keyTTL > 0 {
		if newMark != "" {
			pipe.Expire(ctx, newMarkKey, rm.keyTTL)
		}
	}

	_, err := pipe.Exec(ctx)
	return updatedCount, err
}

// allInSet checks if all markMsgIDs are in msgIDs set.
func allInSet(markMsgIDs []string, msgIDs []string) bool {
	if len(msgIDs) == 0 {
		return true
	}
	msgIDSet := make(map[string]struct{})
	for _, id := range msgIDs {
		msgIDSet[id] = struct{}{}
	}
	for _, id := range markMsgIDs {
		if _, exists := msgIDSet[id]; !exists {
			return false
		}
	}
	return true
}

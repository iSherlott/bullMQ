package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/iSherlott/bullMQ/state"
)

type Options struct {
	Addr          string
	Password      string
	DB            int
	KeyPrefix     string
	UseTLS        bool
	TLSServerName string
}

type Store struct {
	client    *goredis.Client
	keyPrefix string
}

func NewStore(options Options) *Store {
	opts := &goredis.Options{
		Addr:     options.Addr,
		Password: options.Password,
		DB:       options.DB,
	}

	if options.UseTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: options.TLSServerName,
		}
	}

	client := goredis.NewClient(opts)

	return &Store{client: client, keyPrefix: options.KeyPrefix}
}

func (s *Store) Publish(ctx context.Context, queueName string, jobName string, payload []byte) (string, error) {
	if queueName == "" {
		return "", errors.New("queue_name is required")
	}

	keys := state.NewKeys(s.keyPrefix, queueName)

	jobIDNumber, err := s.client.Incr(ctx, keys.JobID()).Result()
	if err != nil {
		return "", err
	}

	jobID := strconv.FormatInt(jobIDNumber, 10)
	jobKey := keys.Job(jobID)

	payloadAsJSON, err := json.Marshal(json.RawMessage(payload))
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()

	_, err = s.client.TxPipelined(ctx, func(p goredis.Pipeliner) error {
		p.HSet(ctx, jobKey,
			"name", jobName,
			"data", payloadAsJSON,
			"timestamp", now.UnixMilli(),
			"status", string(state.Waiting),
			"attempts_made", 0,
		)
		p.LPush(ctx, keys.Waiting(), jobID)
		return nil
	})
	if err == nil {
		return jobID, nil
	}

	if isWrongTypeError(err) {
		// If the queue already exists with old key types, normalize and retry once.
		if normalizeErr := s.EnsureQueueSchema(ctx, queueName); normalizeErr != nil {
			return "", normalizeErr
		}
		_, retryErr := s.client.TxPipelined(ctx, func(p goredis.Pipeliner) error {
			p.HSet(ctx, jobKey,
				"name", jobName,
				"data", payloadAsJSON,
				"timestamp", now.UnixMilli(),
				"status", string(state.Waiting),
				"attempts_made", 0,
			)
			p.LPush(ctx, keys.Waiting(), jobID)
			return nil
		})
		if retryErr != nil {
			return "", retryErr
		}
		return jobID, nil
	}

	return "", err
}

// PopWaiting blocks until there is a job ID available in the waiting list.
func (s *Store) PopWaiting(ctx context.Context, queueName string) (string, error) {
	if queueName == "" {
		return "", errors.New("queue_name is required")
	}

	keys := state.NewKeys(s.keyPrefix, queueName)

	for {
		result, err := s.client.BRPop(ctx, 0*time.Second, keys.Waiting()).Result()
		if err == nil {
			if len(result) != 2 {
				continue
			}
			return result[1], nil
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", ctx.Err()
		}

		if isWrongTypeError(err) {
			if normalizeErr := s.EnsureQueueSchema(ctx, queueName); normalizeErr != nil {
				return "", normalizeErr
			}
			continue
		}

		return "", err
	}
}

func (s *Store) GetJobHash(ctx context.Context, queueName string, jobID string) (map[string]string, error) {
	keys := state.NewKeys(s.keyPrefix, queueName)
	return s.client.HGetAll(ctx, keys.Job(jobID)).Result()
}

func (s *Store) MarkProcessing(ctx context.Context, queueName string, jobID string) error {
	keys := state.NewKeys(s.keyPrefix, queueName)
	jobKey := keys.Job(jobID)

	if err := s.addToProcessing(ctx, keys.Processing(), jobID); err != nil {
		return err
	}

	_, err := s.client.HSet(ctx, jobKey,
		"status", string(state.Processing),
		"processing_at", time.Now().UTC().UnixMilli(),
	).Result()
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) UnmarkProcessing(ctx context.Context, queueName string, jobID string) error {
	keys := state.NewKeys(s.keyPrefix, queueName)
	return s.removeFromProcessing(ctx, keys.Processing(), jobID)
}

func (s *Store) MarkCompleted(ctx context.Context, queueName string, jobID string) error {
	keys := state.NewKeys(s.keyPrefix, queueName)
	jobKey := keys.Job(jobID)
	now := time.Now().UTC().UnixMilli()

	if err := s.removeFromProcessing(ctx, keys.Processing(), jobID); err != nil {
		return err
	}

	if err := s.addToCompleted(ctx, keys.Completed(), jobID, now); err != nil {
		return err
	}

	_, err := s.client.HSet(ctx, jobKey,
		"status", string(state.Completed),
		"completed_at", now,
	).Result()
	return err
}

func (s *Store) IncrementAttempts(ctx context.Context, queueName string, jobID string) (int, error) {
	keys := state.NewKeys(s.keyPrefix, queueName)
	attempts, err := s.client.HIncrBy(ctx, keys.Job(jobID), "attempts_made", 1).Result()
	return int(attempts), err
}

func (s *Store) UpdateLastError(ctx context.Context, queueName string, jobID string, errMsg string) error {
	keys := state.NewKeys(s.keyPrefix, queueName)
	_, err := s.client.HSet(ctx, keys.Job(jobID),
		"last_error", errMsg,
		"last_error_at", time.Now().UTC().UnixMilli(),
	).Result()
	return err
}

func (s *Store) RequeueWaiting(ctx context.Context, queueName string, jobID string) error {
	keys := state.NewKeys(s.keyPrefix, queueName)
	jobKey := keys.Job(jobID)

	_, err := s.client.TxPipelined(ctx, func(p goredis.Pipeliner) error {
		p.HSet(ctx, jobKey, "status", string(state.Waiting))
		p.LPush(ctx, keys.Waiting(), jobID)
		return nil
	})
	if err == nil {
		return nil
	}

	if isWrongTypeError(err) {
		if normalizeErr := s.EnsureQueueSchema(ctx, queueName); normalizeErr != nil {
			return normalizeErr
		}
		_, retryErr := s.client.TxPipelined(ctx, func(p goredis.Pipeliner) error {
			p.HSet(ctx, jobKey, "status", string(state.Waiting))
			p.LPush(ctx, keys.Waiting(), jobID)
			return nil
		})
		return retryErr
	}

	return err
}

func (s *Store) MarkFailed(ctx context.Context, queueName string, jobID string, reason string) error {
	keys := state.NewKeys(s.keyPrefix, queueName)
	jobKey := keys.Job(jobID)
	failedAt := time.Now().UTC().UnixMilli()

	if err := s.addToFailed(ctx, keys.Failed(), jobID, reason, failedAt); err != nil {
		return err
	}

	_, err := s.client.HSet(ctx, jobKey,
		"status", string(state.Failed),
		"failed_at", failedAt,
		"failed_reason", reason,
	).Result()
	return err
}

func (s *Store) EnsureQueueSchema(ctx context.Context, queueName string) error {
	if queueName == "" {
		return errors.New("queue_name is required")
	}

	keys := state.NewKeys(s.keyPrefix, queueName)

	// Waiting/Processing/Paused expected to be lists.
	if err := s.ensureList(ctx, keys.Waiting()); err != nil {
		return fmt.Errorf("ensure waiting list: %w", err)
	}
	if err := s.ensureList(ctx, keys.Processing()); err != nil {
		return fmt.Errorf("ensure processing list: %w", err)
	}
	if err := s.ensureList(ctx, keys.Paused()); err != nil {
		return fmt.Errorf("ensure paused list: %w", err)
	}

	// Completed/Failed expected to be zsets.
	if err := s.ensureZSet(ctx, keys.Completed()); err != nil {
		return fmt.Errorf("ensure completed zset: %w", err)
	}
	if err := s.ensureZSet(ctx, keys.Failed()); err != nil {
		return fmt.Errorf("ensure failed zset: %w", err)
	}

	return nil
}

func (s *Store) ensureList(ctx context.Context, key string) error {
	keyType, err := s.client.Type(ctx, key).Result()
	if err != nil {
		return err
	}
	if keyType == "none" || keyType == "list" {
		return nil
	}

	backupKey := fmt.Sprintf("%s:legacy:%d", key, time.Now().UTC().UnixNano())
	if err := s.client.Rename(ctx, key, backupKey).Err(); err != nil {
		// If the key disappeared, we can proceed.
		if errors.Is(err, goredis.Nil) {
			return nil
		}
		return err
	}

	var members []string
	switch keyType {
	case "set":
		members, err = s.client.SMembers(ctx, backupKey).Result()
	case "zset":
		members, err = s.client.ZRange(ctx, backupKey, 0, -1).Result()
	case "list":
		members, err = s.client.LRange(ctx, backupKey, 0, -1).Result()
	default:
		// Unknown type; keep backup only.
		return nil
	}
	if err != nil {
		return err
	}

	// Rebuild list so that BRPOP returns the oldest element.
	// If we LPUSH from oldest->newest, the newest ends up on the left.
	if len(members) > 0 {
		args := make([]any, 0, len(members))
		for _, member := range members {
			args = append(args, member)
		}
		if _, err := s.client.LPush(ctx, key, args...).Result(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) ensureZSet(ctx context.Context, key string) error {
	keyType, err := s.client.Type(ctx, key).Result()
	if err != nil {
		return err
	}
	if keyType == "none" || keyType == "zset" {
		return nil
	}

	backupKey := fmt.Sprintf("%s:legacy:%d", key, time.Now().UTC().UnixNano())
	if err := s.client.Rename(ctx, key, backupKey).Err(); err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil
		}
		return err
	}

	now := time.Now().UTC().UnixMilli()
	var members []string
	switch keyType {
	case "list":
		members, err = s.client.LRange(ctx, backupKey, 0, -1).Result()
	case "set":
		members, err = s.client.SMembers(ctx, backupKey).Result()
	case "zset":
		members, err = s.client.ZRange(ctx, backupKey, 0, -1).Result()
	default:
		return nil
	}
	if err != nil {
		return err
	}

	if len(members) == 0 {
		return nil
	}

	items := make([]goredis.Z, 0, len(members))
	for idx, m := range members {
		member := m
		if pipe := strings.Index(member, "|"); pipe >= 0 {
			member = member[:pipe]
		}
		items = append(items, goredis.Z{Score: float64(now + int64(idx)), Member: member})
	}

	_, err = s.client.ZAdd(ctx, key, items...).Result()
	return err
}

func (s *Store) addToProcessing(ctx context.Context, processingKey string, jobID string) error {
	_, err := s.client.LPush(ctx, processingKey, jobID).Result()
	if !isWrongTypeError(err) {
		return err
	}

	if normalizeErr := s.ensureList(ctx, processingKey); normalizeErr != nil {
		return normalizeErr
	}

	_, err = s.client.LPush(ctx, processingKey, jobID).Result()
	return err
}

func (s *Store) removeFromProcessing(ctx context.Context, processingKey string, jobID string) error {
	_, err := s.client.LRem(ctx, processingKey, 1, jobID).Result()
	if !isWrongTypeError(err) {
		return err
	}

	if normalizeErr := s.ensureList(ctx, processingKey); normalizeErr != nil {
		return normalizeErr
	}

	_, err = s.client.LRem(ctx, processingKey, 1, jobID).Result()
	return err
}

func (s *Store) addToCompleted(ctx context.Context, completedKey string, jobID string, timestamp int64) error {
	_, err := s.client.ZAdd(ctx, completedKey, goredis.Z{Score: float64(timestamp), Member: jobID}).Result()
	if !isWrongTypeError(err) {
		return err
	}

	if normalizeErr := s.ensureZSet(ctx, completedKey); normalizeErr != nil {
		return normalizeErr
	}

	_, err = s.client.ZAdd(ctx, completedKey, goredis.Z{Score: float64(timestamp), Member: jobID}).Result()
	return err
}

func (s *Store) addToFailed(ctx context.Context, failedKey string, jobID string, failedReason string, timestamp int64) error {
	_, err := s.client.ZAdd(ctx, failedKey, goredis.Z{Score: float64(timestamp), Member: jobID}).Result()
	if !isWrongTypeError(err) {
		return err
	}

	if normalizeErr := s.ensureZSet(ctx, failedKey); normalizeErr != nil {
		return normalizeErr
	}

	_, err = s.client.ZAdd(ctx, failedKey, goredis.Z{Score: float64(timestamp), Member: jobID}).Result()
	return err
}

func isWrongTypeError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToUpper(err.Error()), "WRONGTYPE")
}

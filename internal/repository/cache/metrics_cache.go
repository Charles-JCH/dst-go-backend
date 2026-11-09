package cache

import (
	"context"
	"encoding/json"
	"errors"
	"game-panel/internal/model/entity"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

const (
	// serverMetricsPrefix 每台服务器一个有序集合，score 为采样时间（毫秒），member 为 JSON
	serverMetricsPrefix = "server_metrics:"
	// MetricsRetention 资源使用情况保留时长，更早的采样点写入时删除
	MetricsRetention = time.Hour
)

type MetricsCache interface {
	Add(ctx context.Context, serverID uint64, metrics entity.ServerMetrics) error
	// List 按时间升序返回 since 之后的采样点
	List(ctx context.Context, serverID uint64, since time.Time) ([]entity.ServerMetrics, error)
	// Latest 返回每台服务器最近一个采样点，没有数据的服务器不在结果中
	Latest(ctx context.Context, serverIDs []uint64) (map[uint64]entity.ServerMetrics, error)
	Delete(ctx context.Context, serverID uint64) error
}

type metricsCache struct {
	client *redis.Client
}

func NewMetricsCache(client *redis.Client) MetricsCache {
	return &metricsCache{client: client}
}

// Add 写入一个采样点，同时删除超过保留时长的点；停止上报的服务器整个 key 随过期时间删除
func (c *metricsCache) Add(ctx context.Context, serverID uint64, metrics entity.ServerMetrics) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	key := metricsKey(serverID)
	expired := metrics.Time.Add(-MetricsRetention).UnixMilli()
	_, err = c.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZAdd(ctx, key, redis.Z{Score: float64(metrics.Time.UnixMilli()), Member: data})
		pipe.ZRemRangeByScore(ctx, key, "-inf", "("+strconv.FormatInt(expired, 10))
		pipe.Expire(ctx, key, MetricsRetention)
		return nil
	})
	return err
}

func (c *metricsCache) List(ctx context.Context, serverID uint64, since time.Time) ([]entity.ServerMetrics, error) {
	members, err := c.client.ZRangeByScore(ctx, metricsKey(serverID), &redis.ZRangeBy{
		Min: strconv.FormatInt(since.UnixMilli(), 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}
	list := make([]entity.ServerMetrics, 0, len(members))
	for _, member := range members {
		var m entity.ServerMetrics
		if err := json.Unmarshal([]byte(member), &m); err != nil {
			continue // 格式不对的点直接跳过
		}
		list = append(list, m)
	}
	return list, nil
}

func (c *metricsCache) Latest(ctx context.Context, serverIDs []uint64) (map[uint64]entity.ServerMetrics, error) {
	result := make(map[uint64]entity.ServerMetrics, len(serverIDs))
	if len(serverIDs) == 0 {
		return result, nil
	}

	cmds := make([]*redis.StringSliceCmd, len(serverIDs))
	_, err := c.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i, id := range serverIDs {
			cmds[i] = pipe.ZRange(ctx, metricsKey(id), -1, -1)
		}
		return nil
	})
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	for i, cmd := range cmds {
		members, err := cmd.Result()
		if err != nil || len(members) == 0 {
			continue
		}
		var m entity.ServerMetrics
		if err := json.Unmarshal([]byte(members[0]), &m); err == nil {
			result[serverIDs[i]] = m
		}
	}
	return result, nil
}

func (c *metricsCache) Delete(ctx context.Context, serverID uint64) error {
	return c.client.Del(ctx, metricsKey(serverID)).Err()
}

func metricsKey(serverID uint64) string {
	return serverMetricsPrefix + strconv.FormatUint(serverID, 10)
}

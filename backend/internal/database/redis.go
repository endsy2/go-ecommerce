package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"ecommerce/backend/internal/config"
)

// NewRedis creates the Redis client and probes it once.
//
// Unlike Postgres, a failed ping is NOT fatal. Redis is a cache here, not a
// source of truth, so the app can still serve correct (if slower) responses
// without it. The client is returned either way and the readiness endpoint
// reports the real state, letting a load balancer decide what to do.
func NewRedis(ctx context.Context, cfg config.RedisConfig) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:        cfg.Addr(),
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: cfg.DialTimeout,
	})

	pingCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		slog.Warn("redis is unreachable at startup; continuing in degraded mode",
			"addr", cfg.Addr(),
			"error", err,
		)
		return client
	}

	slog.Info("connected to redis", "addr", cfg.Addr(), "db", cfg.DB)
	return client
}

// PingRedis reports whether Redis is currently reachable. Used by the readiness
// probe, which must reflect live state rather than what was true at startup.
func PingRedis(ctx context.Context, client *redis.Client) error {
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("pinging redis: %w", err)
	}
	return nil
}

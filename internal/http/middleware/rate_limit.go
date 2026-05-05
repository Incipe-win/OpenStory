package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func IPRateLimit(rdb *redis.Client, limit int, log zerolog.Logger) gin.HandlerFunc {
	return fixedWindowLimit(rdb, limit, log, func(c *gin.Context) string {
		return "ip:" + c.ClientIP()
	})
}

func UserRateLimit(rdb *redis.Client, limit int, log zerolog.Logger) gin.HandlerFunc {
	return fixedWindowLimit(rdb, limit, log, func(c *gin.Context) string {
		userID := GetUserID(c)
		if userID.String() == "00000000-0000-0000-0000-000000000000" {
			return ""
		}
		return "user:" + userID.String()
	})
}

func fixedWindowLimit(rdb *redis.Client, limit int, log zerolog.Logger, keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil || limit <= 0 {
			c.Next()
			return
		}
		base := keyFn(c)
		if base == "" {
			c.Next()
			return
		}
		window := time.Now().UTC().Unix() / 60
		key := fmt.Sprintf("openstory:rate:%s:%d", base, window)
		count, err := rdb.Incr(context.Background(), key).Result()
		if err != nil {
			log.Warn().Err(err).Str("key", key).Msg("rate limit check skipped")
			c.Next()
			return
		}
		if count == 1 {
			_ = rdb.Expire(context.Background(), key, 2*time.Minute).Err()
		}
		if count > int64(limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
				"code": "RATE_LIMITED", "message": "rate limit exceeded",
			}})
			c.Abort()
			return
		}
		c.Next()
	}
}

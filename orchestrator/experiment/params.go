package experiment

import (
	"context"
	"fmt"
	"slices"

	"github.com/rs/zerolog"
	"github.com/zaporter/branch-by-branch/orchestrator"
)

func setParams(ctx context.Context, params map[string]any) error {
	rdb, err := orchestrator.ConnectToRedis(ctx)
	if err != nil {
		return err
	}
	logger := zerolog.Ctx(ctx)
	defer rdb.Close()
	for key, value := range params {
		if !slices.Contains(orchestrator.AllRouterKeys, orchestrator.RedisKey(key)) {
			return fmt.Errorf("key %s not found in router keys", key)
		}
		if err := rdb.Set(ctx, key, value, 0).Err(); err != nil {
			return err
		}
		logger.Info().Msgf("%s = %v", key, value)
	}
	return nil
}

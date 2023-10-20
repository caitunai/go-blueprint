package db

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type Logger struct {
	SourceField           string
	LogLevel              logger.LogLevel
	SlowThreshold         time.Duration
	SkipErrRecordNotFound bool
}

func NewLogger() *Logger {
	return &Logger{
		SkipErrRecordNotFound: true,
		SlowThreshold:         time.Millisecond * 100,
		LogLevel:              logger.Warn,
	}
}

func (l *Logger) LogMode(lvl logger.LogLevel) logger.Interface {
	return &Logger{
		SlowThreshold:         l.SlowThreshold,
		SourceField:           l.SourceField,
		SkipErrRecordNotFound: l.SkipErrRecordNotFound,
		LogLevel:              lvl,
	}
}

func (l *Logger) Info(ctx context.Context, s string, args ...any) {
	log.Ctx(ctx).Info().Msgf(s, args...)
}

func (l *Logger) Warn(ctx context.Context, s string, args ...any) {
	log.Ctx(ctx).Warn().Msgf(s, args...)
}

func (l *Logger) Error(ctx context.Context, s string, args ...any) {
	log.Ctx(ctx).Error().Msgf(s, args...)
}

func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	sql, _ := fc()
	fields := map[string]any{
		"namespace": "gorm",
		"sql":       sql,
		"duration":  elapsed,
	}
	if l.SourceField != "" {
		fields[l.SourceField] = utils.FileWithLineNum()
	}
	if err != nil && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.SkipErrRecordNotFound) && l.LogLevel >= logger.Error {
		log.Ctx(ctx).Error().Err(err).Fields(fields).Msg("[GORM] error query")
		return
	}

	if l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= logger.Warn {
		log.Ctx(ctx).Warn().Fields(fields).Msgf("[GORM] slow query")
		return
	}

	if l.LogLevel == logger.Info {
		log.Ctx(ctx).Info().Fields(fields).Msgf("[GORM] query")
	}
}

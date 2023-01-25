package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/redhat-et/apex/internal/util"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type zapLogger struct {
	logger                    *zap.SugaredLogger
	SlowThreshold             time.Duration
	LogLevel                  logger.LogLevel
	IgnoreRecordNotFoundError bool
}

func NewLogger(sugar *zap.SugaredLogger) *zapLogger {
	return &zapLogger{
		logger:                    sugar,
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
	}
}

func (z *zapLogger) LogMode(level logger.LogLevel) logger.Interface {
	return &zapLogger{
		logger:                    z.logger,
		SlowThreshold:             z.SlowThreshold,
		LogLevel:                  level,
		IgnoreRecordNotFoundError: z.IgnoreRecordNotFoundError,
	}
}

func (z zapLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	if z.LogLevel >= logger.Info {
		util.WithTrace(ctx, z.logger).Infof(msg, args...)
	}
}

func (z zapLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if z.LogLevel >= logger.Warn {
		util.WithTrace(ctx, z.logger).Warnf(msg, args...)
	}
}

func (z zapLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	if z.LogLevel >= logger.Warn {
		util.WithTrace(ctx, z.logger).Errorf(msg, args...)
	}
}

func (z zapLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if z.LogLevel <= logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && z.LogLevel >= logger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !z.IgnoreRecordNotFoundError):
		sql, rows := fc()
		util.WithTrace(ctx, z.logger).With(
			"line_number", utils.FileWithLineNum(),
			"error", err.Error(),
			"rows", rows,
			"elapsed", float64(elapsed.Nanoseconds())/1e6,
		).Debugf(sql)
	case elapsed > z.SlowThreshold && z.SlowThreshold != 0 && z.LogLevel >= logger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", z.SlowThreshold)
		util.WithTrace(ctx, z.logger).With(
			"line_number", utils.FileWithLineNum(),
			"slow", slowLog,
			"rows", rows,
			"elapsed", float64(elapsed.Nanoseconds())/1e6,
		).Warnf(sql)
	case z.LogLevel == logger.Info:
		sql, rows := fc()
		util.WithTrace(ctx, z.logger).With(
			"line_number", utils.FileWithLineNum(),
			"rows", rows,
			"elapsed", float64(elapsed.Nanoseconds())/1e6,
		).Infof(sql)
	}
}

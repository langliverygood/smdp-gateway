package logger

import (
	"context"
	"io"
	"os"
	"smdp-gateway/config"
	"time"

	"github.com/kataras/iris/v12"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogConfig 日志配置
type LogConfig struct {
	FileName             string
	MaxAgeByHour         int64
	RotationTimeByMinute int64
	Level                string
	JSONEncoder          bool
}

type logger struct {
	Logger *zap.Logger
	Writer io.Writer
}

var defaultLogger logger

// TraceIDKey 追踪关键字，用于定位同一次请求的日志
const TraceIDKey = "traceID"

// Init 初始化日志
func Init() error {
	logConfig := LogConfig{
		FileName:             config.Global().Log.FileName,
		RotationTimeByMinute: config.Global().Log.RotationTimeByMinute,
		MaxAgeByHour:         config.Global().Log.MaxAgeByHour,
		Level:                config.Global().Log.Level,
		JSONEncoder:          config.Global().Log.JSONEncoder,
	}

	err := defaultLogger.initCore(logConfig, false, false)
	if err != nil {
		return err
	}

	return nil
}

// Sync 日志落盘
func Sync() {
	defaultLogger.Sync()
}

// Default 默认日志对象
func Default(ctx ...any) *zap.Logger {
	if len(ctx) > 0 {
		contextValue, ok := ctx[0].(context.Context)
		if ok && contextValue != nil {
			if traceID, ok := contextValue.Value(TraceIDKey).(string); ok && len(traceID) > 0 {
				return defaultLogger.Logger.With(zap.String(TraceIDKey, traceID))
			}
			return defaultLogger.Logger
		}

		irisValue, ok := ctx[0].(iris.Context)
		if ok && irisValue != nil {
			if traceID, ok := irisValue.Value(TraceIDKey).(string); ok && len(traceID) > 0 {
				return defaultLogger.Logger.With(zap.String(TraceIDKey, traceID))
			}
			return defaultLogger.Logger
		}
	}
	return defaultLogger.Logger
}

func (l *logger) initCore(config LogConfig, caller, pid bool) error {
	writer, err := getWriter(config.FileName, config.MaxAgeByHour, config.RotationTimeByMinute)
	if err != nil {
		return err
	}

	var level zapcore.Level
	if err = level.UnmarshalText([]byte(config.Level)); err != nil {
		return err
	}
	levelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= level
	})
	encoderLogConfig := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.CapitalLevelEncoder,
		TimeKey:     "time",
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02T15:04:05.000+0800"))
		},
		CallerKey:    "file",
		EncodeCaller: zapcore.ShortCallerEncoder,
		EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendInt64(int64(d) / 1000000)
		},
	}

	var encoder zapcore.Encoder
	if config.JSONEncoder {
		encoder = zapcore.NewJSONEncoder(encoderLogConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderLogConfig)
	}

	if caller {
		l.Logger = zap.New(
			zapcore.NewCore(encoder, zapcore.AddSync(writer), levelEnabler),
			zap.AddCaller())
	} else {
		l.Logger = zap.New(
			zapcore.NewCore(encoder, zapcore.AddSync(writer), levelEnabler))
	}
	if pid {
		l.Logger = l.Logger.With(zap.Int("pid", os.Getpid()))
	}
	l.Writer = writer
	return nil
}

func getWriter(fileName string, maxAgeByHour int64, rotationTimeByMinute int64) (io.Writer, error) {
	hook, err := rotatelogs.New(
		fileName+".%Y%m%d%H%M",
		rotatelogs.WithLinkName(fileName),
		rotatelogs.WithMaxAge(time.Hour*time.Duration(maxAgeByHour)),
		rotatelogs.WithRotationTime(time.Minute*time.Duration(rotationTimeByMinute)),
	)
	return hook, err
}

func (l *logger) Sync() error {
	return l.Logger.Sync()
}

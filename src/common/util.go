package common

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func BriefHash(hash string) string {
	return fmt.Sprintf("[%s...%s]", hash[:8], hash[len(hash)-8:])
}

func CheckError(err error) {
	if err != nil {
		zap.S().Fatalf("Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func SetupLogger(level string) (*zap.Logger, *zap.SugaredLogger) {
	al := zap.NewAtomicLevel()
	var opts []zap.Option
	switch strings.ToUpper(level) {
	case "DEV":
		al.SetLevel(zap.DebugLevel)
		opts = append(opts, zap.AddCaller())
	case "DEBUG":
		al.SetLevel(zap.DebugLevel)
	case "INFO":
		al.SetLevel(zap.InfoLevel)
	case "ERROR":
		al.SetLevel(zap.ErrorLevel)
	case "WARN":
		al.SetLevel(zap.WarnLevel)
	case "FATAL":
		al.SetLevel(zap.FatalLevel)
	default:
		al.SetLevel(zap.InfoLevel)
	}
	ec := zap.NewDevelopmentEncoderConfig()
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(ec), zapcore.Lock(os.Stdout), al)
	logger := zap.New(core)
	zap.ReplaceGlobals(logger.WithOptions(opts...))
	return logger, logger.Sugar()
}

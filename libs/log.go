package libs

import (
	"fmt"
	"net/http"
	"time"

	shutdown "github.com/klauspost/shutdown2"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/danielnguyentb/url-shortener/middlewares"
)

var isInitialized bool

// InitLogging initialize a zap Logger that is docker friendly and a function to trap error and send panic trace if
func InitLogging() *zap.Logger {
	if isInitialized {
		panic("Already initialized (InitLogging)")
	}

	viper.SetDefault("log.level", zap.DebugLevel.String())
	viper.SetDefault("log.timestamp", true)

	var (
		level    zapcore.Level
		levelStr = viper.GetString("log.level")
		levelErr error
		timeKey  string
	)

	if viper.GetBool("log.timestamp") {
		timeKey = "ts"
	}

	if levelErr = level.UnmarshalText([]byte(levelStr)); levelErr != nil {
		level = zap.DebugLevel
	}

	conf := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: true,
		Sampling:          nil,
		Encoding:          "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        timeKey,
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	log, err := conf.Build()
	if err != nil {
		panic(err)
	}

	if levelErr != nil {
		log.Error("Couldn't parse logging level, switch to debug", zap.String("level", levelStr), zap.Error(levelErr))
	}

	sl := log.Sugar()
	shutdown.SetLogPrinter(sl.Infof)

	return log
}

type structuredLogger struct {
	logger *zap.Logger
}

func (s *structuredLogger) NewLogEntry(r *http.Request) middlewares.LogEntry {
	entry := &loggerEntry{logger: s.logger}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	fields := []zapcore.Field{
		zap.String("ts", time.Now().UTC().Format(time.RFC1123)),
		zap.String("http_scheme", scheme),
		zap.String("http_proto", r.Proto),
		zap.String("http_method", r.Method),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("user_agent", r.UserAgent()),
		zap.String("uri", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
	}

	entry.logger = entry.logger.With(fields...)
	entry.logger.Info("request started")

	return entry
}

type loggerEntry struct {
	logger *zap.Logger
}

func (l *loggerEntry) Write(elapsed time.Duration) {
	l.logger.
		With(zap.Float64("resp_elapsed_ms", float64(elapsed.Nanoseconds())/1000000.0)).
		Info("request completed")
}

func (l *loggerEntry) Panic(v interface{}, stack []byte) {
	l.logger.With([]zapcore.Field{
		zap.String("stack", string(stack)),
		zap.String("panic", fmt.Sprintf("%+v", v)),
	}...).Panic("panic")
}

// Helper method used to create log middleware with zap logger
func NewZapLogEntry(log *zap.Logger) func(next http.Handler) http.Handler {
	return middlewares.RequestLogger(&structuredLogger{logger: log})
}

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.
func GetLogEntry(r *http.Request) *zap.Logger {
	entry := middlewares.GetLogEntry(r).(*loggerEntry)
	return entry.logger
}

// RecoverLog log the panic as an error
func RecoverLog(log *zap.Logger, f func()) {
	defer func() {
		var field zapcore.Field
		err := recover()
		switch rval := err.(type) {
		case nil:
			return
		case error:
			field = zap.Error(rval)
		default:
			field = zap.String("error_raw", fmt.Sprint(rval))
		}
		log.Error("Recovered panic", field)
	}()

	f()
}

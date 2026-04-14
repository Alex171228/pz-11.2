package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"

	"pz1.2/shared/logger"
)

// AccessLog logs each completed HTTP request. Optional staticFields (e.g. instance_id for load-balanced replicas)
// are added to the logger and to every "request completed" line.
func AccessLog(base *zap.Logger, staticFields ...zap.Field) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			rid := GetRequestID(r.Context())
			fields := append([]zap.Field{zap.String("request_id", rid)}, staticFields...)
			reqLog := base.With(fields...)
			ctx := logger.WithContext(r.Context(), reqLog)

			next.ServeHTTP(rw, r.WithContext(ctx))

			reqLog.Info("request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rw.statusCode),
				zap.Float64("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
				zap.String("remote_ip", r.RemoteAddr),
			)
		})
	}
}

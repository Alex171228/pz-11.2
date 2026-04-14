package http

import "net/http"

// InstanceIDMiddleware adds X-Instance-ID to every response (including /metrics).
func InstanceIDMiddleware(instanceID string, next http.Handler) http.Handler {
	if instanceID == "" {
		instanceID = "tasks-default"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Instance-ID", instanceID)
		next.ServeHTTP(w, r)
	})
}

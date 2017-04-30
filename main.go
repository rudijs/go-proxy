package main

import (
	"net/http"
	"net/url"
	"strings"

	"time"

	"net/http/httputil"

	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type contextKey string

func (c contextKey) String() string {
	return string(c)
}

var (
	contextKeyLatencyStart = contextKey("latencyStart")
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

//https://gist.github.com/Boerworz/b683e46ae0761056a636
//https://github.com/prometheus/client_golang/blob/master/examples/simple/main.go

func main() {

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   "localhost:3000",
	})

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", decorate(proxy, wrapHandlerWithLogging, latency))
	// http.Handle("/", decorate(proxy, wrapHandlerWithLogging, latency, auth))
	log.Fatal(http.ListenAndServe(":8080", nil))

}

type decorator func(http.Handler) http.Handler

func decorate(f http.Handler, d ...decorator) http.Handler {
	decorated := f
	for _, decorateFn := range d {
		// fmt.Printf("Decorating %v", runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name())
		// fmt.Printf(" with %v\n", runtime.FuncForPC(reflect.ValueOf(decorateFn).Pointer()).Name())
		decorated = decorateFn(decorated)
	}
	return decorated
}

func auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		username, password, ok := req.BasicAuth()
		if ok != true {
			log.WithFields(log.Fields{
				"username": username,
				"password": password,
			}).Error("not authorized")
			http.Error(w, "Not authorized", 401)
			return
		}
		if username != "user" || password != "secret" {
			log.WithFields(log.Fields{
				"username": username,
				"password": password,
			}).Error("not authorized")
			http.Error(w, "Not authorized", 401)
			return
		}
		log.WithFields(log.Fields{
			"username": username,
		}).Info("authorized")
		next.ServeHTTP(w, req)
	})
}

func latency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// defer func(start time.Time) {
		// 	fmt.Println("latency:", time.Since(start).Nanoseconds())
		// }(time.Now())
		ctx := context.WithValue(req.Context(), contextKeyLatencyStart, time.Now())
		req = req.WithContext(ctx)
		next.ServeHTTP(w, req)

	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func newLoggngResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func wrapHandlerWithLogging(wrappedHander http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		lrw := newLoggngResponseWriter(w)

		wrappedHander.ServeHTTP(lrw, req)

		headers := make(map[string]string)

		for k, v := range req.Header {
			headers[k] = strings.Join(v, ",")
		}

		request := log.Fields{
			"host":       req.Host,
			"requestUri": req.RequestURI,
			"remoteAddr": req.RemoteAddr,
			"method":     req.Method,
			"headers":    headers,
		}

		ctx := req.Context()
		latencyStart := ctx.Value(contextKeyLatencyStart).(time.Time)
		//milliseconds
		latency := time.Since(latencyStart).Nanoseconds() / 1000000

		response := log.Fields{
			"status":  lrw.statusCode,
			"latency": latency,
		}

		log.WithFields(log.Fields{
			"request":  request,
			"response": response,
		}).Info()

	})
}

package main

import (
	"net/http"
	"net/url"
	"strings"

	"time"

	"net/http/httputil"

	log "github.com/Sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

func main() {

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   "localhost:3000",
	})

	log.Fatal(http.ListenAndServe(":8080", decorate(proxy, requestLogging, timing, auth)))

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

func timing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// fmt.Println("timer start")
		defer func(start time.Time) {
			log.Info("timing:", time.Since(start).Nanoseconds())

		}(time.Now())
		next.ServeHTTP(w, req)
	})
}

func requestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		headers := make(map[string]string)

		for k, v := range req.Header {
			headers[k] = strings.Join(v, ",")
		}

		// log.Println("Start")
		next.ServeHTTP(w, req)
		// log.Println("Stop")

		log.WithFields(log.Fields{
			"host":       req.Host,
			"requestUri": req.RequestURI,
			"remoteAddr": req.RemoteAddr,
			"method":     req.Method,
			"headers":    headers,
		}).Info()

	})
}

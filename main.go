package main

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	logger := logrus.WithFields(logrus.Fields{
		"service": "mini-auth-proxy",
	})

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.mini-auth-proxy")
	viper.AddConfigPath(".")

	viper.WatchConfig()
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	server := startServer()

	viper.OnConfigChange(func(e fsnotify.Event) {
		logger.Println("Config file changed:", e.Name)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Warn(err)
		}

		server = startServer()
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Waiting for SIGINT (pkill -2)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Warn(err)
	}
}

func startServer() *http.Server {

	logger := logrus.WithFields(logrus.Fields{
		"service": "mini-auth-proxy",
	})

	target := viper.GetString("target")
	token := viper.GetString("token")
	addr := viper.GetString("addr")

	router := httprouter.New()
	origin, _ := url.Parse(target)
	path := "/*catchall"

	reverseProxy := httputil.NewSingleHostReverseProxy(origin)

	reverseProxy.Director = func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", origin.Host)
		req.URL.Scheme = origin.Scheme
		req.URL.Host = origin.Host

		if req.Header.Get("Authorization") == "" {
			req.Header.Add("Authorization", "Bearer "+token)
		}

		wildcardIndex := strings.IndexAny(path, "*")
		proxyPath := singleJoiningSlash(origin.Path, req.URL.Path[wildcardIndex:])
		if strings.HasSuffix(proxyPath, "/") && len(proxyPath) > 1 {
			proxyPath = proxyPath[:len(proxyPath)-1]
		}
		req.URL.Path = proxyPath
		req.Host = origin.Host
	}

	router.Handle("GET", path, func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reverseProxy.ServeHTTP(w, r)
	})

	server := &http.Server{Addr: addr, Handler: handlers.LoggingHandler(logger.Writer(), router)}

	logger.Info("listening on ", addr, " forwarding requests to: "+target)

	go func() { logger.Warn(server.ListenAndServe()) }()

	return server
}

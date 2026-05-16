// Copyright 2017 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	httppkg "github.com/65658dsf/StellarCore/pkg/util/http"
	netpkg "github.com/65658dsf/StellarCore/pkg/util/net"
)

func (svr *Service) registerRouteHandlers(helper *httppkg.RouterRegisterHelper) {
	helper.Router.HandleFunc("/healthz", httppkg.MakeHTTPHandlerFunc(svr.apiController.Healthz)).Methods("GET")
	subRouter := helper.Router.NewRoute().Subrouter()

	subRouter.Use(helper.AuthMiddleware.Middleware)

	if svr.cfg.EnablePrometheus {
		subRouter.Handle("/metrics", promhttp.Handler())
	}

	subRouter.HandleFunc("/api/serverinfo", httppkg.MakeHTTPHandlerFunc(svr.apiController.APIServerInfo)).Methods("GET")
	subRouter.HandleFunc("/api/clients", httppkg.MakeHTTPHandlerFunc(svr.apiController.APIClientList)).Methods("GET")
	subRouter.HandleFunc("/api/clients/{key}", httppkg.MakeHTTPHandlerFunc(svr.apiController.APIClientDetail)).Methods("GET")
	subRouter.HandleFunc("/api/proxy/{type}", httppkg.MakeHTTPHandlerFunc(svr.apiController.APIProxyByType)).Methods("GET")
	subRouter.HandleFunc("/api/proxy/{type}/{name}", httppkg.MakeHTTPHandlerFunc(svr.apiController.APIProxyByTypeAndName)).Methods("GET")
	subRouter.HandleFunc("/api/proxies/{name}", httppkg.MakeHTTPHandlerFunc(svr.apiController.APIProxyByName)).Methods("GET")
	subRouter.HandleFunc("/api/proxies", httppkg.MakeHTTPHandlerFunc(svr.apiController.DeleteProxies)).Methods("DELETE")
	subRouter.HandleFunc("/api/traffic/{name}", httppkg.MakeHTTPHandlerFunc(svr.apiController.APIProxyTraffic)).Methods("GET")
	subRouter.HandleFunc("/api/traffic", httppkg.MakeHTTPHandlerFunc(svr.apiController.APIAllProxiesTraffic)).Methods("GET")
	subRouter.HandleFunc("/api/traffic/trend", httppkg.MakeHTTPHandlerFunc(svr.apiController.APITrafficTrend)).Methods("GET")
	subRouter.HandleFunc("/api/client/kick", httppkg.MakeHTTPHandlerFunc(svr.apiController.KickClient)).Methods("POST")
	subRouter.HandleFunc("/api/config", httppkg.MakeHTTPHandlerFunc(svr.apiController.GetConfig)).Methods("GET")
	subRouter.HandleFunc("/api/restart", httppkg.MakeHTTPHandlerFunc(svr.apiController.RestartService)).Methods("POST")
	subRouter.HandleFunc("/api/logs", httppkg.MakeHTTPHandlerFunc(svr.apiController.GetLogs)).Methods("GET")
	subRouter.HandleFunc("/api/update", httppkg.MakeHTTPHandlerFunc(svr.apiController.UpdateService)).Methods("POST")

	subRouter.Handle("/favicon.ico", http.FileServer(helper.AssetsFS)).Methods("GET")
	subRouter.PathPrefix("/static/").Handler(
		netpkg.MakeHTTPGzipHandler(http.StripPrefix("/static/", http.FileServer(helper.AssetsFS))),
	).Methods("GET")

	subRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
	})
}

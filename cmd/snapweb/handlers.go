/*
 * Copyright (C) 2014, 2015, 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/snapcore/snapweb/snappy/app"
	"github.com/snapcore/snapweb/snappy/snapdclient"
)

type branding struct {
	Name    string
	Subname string
}

type templateData struct {
	Branding     branding
	SnapdVersion string
}

var newSnapdClient = newSnapdClientImpl

func unixDialer(socketPath string) func(string, string) (net.Conn, error) {
	file, err := os.OpenFile(socketPath, os.O_RDWR, 0666)
	if err == nil {
		file.Close()
	}

	return func(_, _ string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}
}

func newSnapdClientImpl() snapdclient.SnapdClient {
	return snapdclient.NewClientAdapter()
}

func getSnappyVersion() string {
	c := newSnapdClient()

	verInfo, err := c.ServerVersion()
	if err != nil {
		return "snapd"
	}

	return fmt.Sprintf("snapd %s (series %s)", verInfo.Version, verInfo.Series)
}

type timeInfoResponse struct {
	DateTime  int64  `json:"dateTime,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	NTP       bool   `json:"ntp,omitempty"`
	NTPServer string `json:"ntpServer,omitempty"`
}

func handleTimeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		values, err := getTimeInfo()
		if err != nil {
			log.Printf("Error fetching time related information: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		info := timeInfoResponse{
			DateTime:  values["DateTime"].(int64),
			Timezone:  values["Timezone"].(string),
			Offset:    values["Offset"].(int),
			NTP:       values["NTP"].(bool),
			NTPServer: values["NTPServer"].(string),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(info); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Error encoding time informaiton: %v", err)
		}
	} else if r.Method == "PATCH" {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			log.Printf("handleTimeInfo(POST): invalid content")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}

		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("Error decoding time patch", err)
			return
		}

		var timePatch map[string]interface{}
		err = json.Unmarshal(data, &timePatch)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Printf("handleTimeInfo(POST): Error decoding time data: %v", err)
			return
		}

		err = setTimeInfo(timePatch)
		if err != nil {
			log.Printf("handleTimeInfo: failed to set time information; %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type deviceInfoResponse struct {
	DeviceName string   `json:"deviceName"`
	Brand      string   `json:"brand"`
	Model      string   `json:"model"`
	Serial     string   `json:"serial"`
	OS         string   `json:"operatingSystem"`
	Interfaces []string `json:"interfaces"`
	Uptime     string   `json:"uptime"`
}

func handleDeviceInfo(w http.ResponseWriter, r *http.Request) {
	c := newSnapdClient()

	modelInfo, err := snapdclient.GetModelInfo(c)
	if err != nil {
		log.Println(fmt.Sprintf("handleDeviceInfo: error retrieving model info: %s", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var info deviceInfoResponse
	info.Brand = modelInfo["Brand"].(string)
	info.Model = modelInfo["Model"].(string)
	info.Serial = modelInfo["Serial"].(string)
	info.OS = modelInfo["OS"].(string)
	info.Interfaces = modelInfo["Interfaces"].([]string)
	info.Uptime = modelInfo["Uptime"].(string)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Println(fmt.Sprintf("handleDeviceInfo: error serializing json: %s", err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func handleSections(w http.ResponseWriter, r *http.Request) {
	c := newSnapdClient()

	sections, err := c.Sections()
	if err != nil {
		log.Println(fmt.Sprintf("handleSections: error retrieving sections info: %s", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sections); err != nil {
		log.Println(fmt.Sprintf("handleSections: error serializing json: %s", err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type deviceAction struct {
	ActionType string `json:"actionType"`
}

func handleDeviceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Printf("handleDeviceAction: invalid method")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Printf("handleDeviceAction: invalid content")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	var action deviceAction
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&action); err != nil {
		log.Printf("handleDeviceAction: failed to decode json: %v", err)
		w.WriteHeader(http.StatusBadRequest)
	}

	// XXX: user valid and user has permission
	if action.ActionType == "restart" {
		cmd := exec.Command("reboot")
		if err := cmd.Run(); err != nil {
			log.Printf("handleDeviceAction: failed to reboot: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else if action.ActionType == "power-off" {
		cmd := exec.Command("poweroff")
		if err := cmd.Run(); err != nil {
			log.Printf("handleDeviceAction: failed to reboot: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		log.Printf("handleDeviceAction: invalid device action type: %s", action.ActionType)
		w.WriteHeader(http.StatusBadRequest)
	}
}

func initURLHandlers(log *log.Logger, config snappy.Config) http.Handler {
	log.Println("Initializing HTTP handlers...")

	handler := http.NewServeMux()

	// API
	handler.Handle("/api/", makeAPIHandler("/api/", config))

	// Resources
	handler.Handle("/public/", loggingHandler(http.FileServer(http.Dir(filepath.Join(os.Getenv("SNAP"), "www")))))

	if iconDir, relativePath, err := snappy.IconDir(); err == nil {
		handler.Handle(fmt.Sprintf("/%s/", relativePath), loggingHandler(http.FileServer(http.Dir(filepath.Join(iconDir, "..")))))
	} else {
		log.Println("Issues while getting icon dir:", err)
	}

	handler.HandleFunc("/", makeMainPageHandler())

	return NewFilterHandlerFromConfig(handler, config)
}

// Name of the cookie transporting the access token
const (
	SnapwebCookieName = "SM"
)

func tokenFilename() string {
	return filepath.Join(os.Getenv("SNAP_DATA"), "token.txt")
}

// SimpleCookieCheck is a simple authorization mechanism
func SimpleCookieCheck(w http.ResponseWriter, r *http.Request) error {
	cookie, _ := r.Cookie(SnapwebCookieName)
	if cookie != nil {
		token, err := ioutil.ReadFile(tokenFilename())
		if err == nil {
			if string(token) == cookie.Value {
				// the auth-token and the cookie do match
				// we can continue with the request
				return nil
			}
		}
	}
	return errors.New("Unauthorized")
}

func validateToken(w http.ResponseWriter, r *http.Request) {
	// We only get here when the Cookie is valid, send an empty response
	// to keep the model happy
	hdr := w.Header()
	hdr.Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "{}")
}

func loggingHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}

func getBranding() branding {
	return branding{
		Name:    "Ubuntu",
		Subname: "",
	}
}

func makeMainPageHandler() http.HandlerFunc {
	b := getBranding()
	v := getSnappyVersion()

	return func(w http.ResponseWriter, r *http.Request) {
		data := templateData{
			Branding:     b,
			SnapdVersion: v,
		}

		if err := renderLayout("index.html", &data, w); err != nil {
			log.Println(err)
		}
	}
}

func renderLayout(html string, data *templateData, w http.ResponseWriter) error {
	htmlPath := filepath.Join(os.Getenv("SNAP"), "www", "templates", html)
	if _, err := os.Stat(htmlPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	layoutPath := filepath.Join(os.Getenv("SNAP"), "www", "templates", "base.html")
	t, err := template.ParseFiles(layoutPath, htmlPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return t.Execute(w, *data)
}

func redirHandler(config snappy.Config) http.Handler {
	redir := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req,
			"https://"+strings.Replace(req.Host, httpAddr, httpsAddr, -1),
			http.StatusSeeOther)
	})

	return NewFilterHandlerFromConfig(redir, config)
}

// NewFilterHandlerFromConfig creates a new http.Handler with an integrated NetFilter
func NewFilterHandlerFromConfig(handler http.Handler, config snappy.Config) http.Handler {
	if config.DisableIPFilter {
		return handler
	}

	f := snappy.NewFilter()

	for _, net := range config.AllowNetworks {
		f.AllowNetwork(net)
	}

	for _, ifname := range config.AllowInterfaces {
		f.AddLocalNetworkForInterface(ifname)
	}

	// if nothing was specified, default to allowing all local networks
	if (len(config.AllowNetworks) == 0) &&
		(len(config.AllowInterfaces) == 0) {
		logger.Println("Allowing local network access by default")
		f.AddLocalNetworks()
	}

	return f.FilterHandler(handler)
}

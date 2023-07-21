package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/toqueteos/webbrowser"

	"github.com/hvaghani221/helper/plugins"
)

//go:embed static
var staticFiles embed.FS

//go:embed api.json
var pluginList []byte

func loadPlugins(path string) (map[string]string, error) {
	var apis []struct {
		Name string `json:"name"`
		Url  string `json:"spec_url"`
	}

	list := pluginList

	if path != "" {

		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}

		list, err = io.ReadAll(file)
		if err != nil {
			return nil, err
		}
	}

	if err := json.Unmarshal(list, &apis); err != nil {
		return nil, err
	}

	res := make(map[string]string)
	for _, api := range apis {
		if api.Name == "" && api.Url == "" {
			continue
		}
		res[api.Name] = api.Url
	}

	return res, nil
}

func main() {
	address := flag.String("address", "localhost:8080", "Address to listen on")
	path := flag.String("path", "", "Path to api json file. If empty, uses default APIs")

	flag.Parse()

	urls, err := loadPlugins(*path)
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()
	ph := plugins.NewPlugin(staticFiles, urls)
	sh := plugins.NewAPIServer(*address, "/apis", urls)

	r.Use(loggingMiddleware())

	r.HandleFunc("/plugin", ph.PageHandler)
	r.HandleFunc("/plugin/{api}", ph.PluginHandler)
	r.HandleFunc("/schema/{api}", sh.ServeSchema)
	r.PathPrefix("/apis/{api}").PathPrefix("/").HandlerFunc(sh.APIRequester)

	errchan := make(chan error)
	go func() { errchan <- http.ListenAndServe(":8080", r) }()

	_ = webbrowser.Open("http://" + *address + "/plugin")
	err = <-errchan
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}

func loggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Println(r.RequestURI)
			next.ServeHTTP(w, r)
		})
	}
}

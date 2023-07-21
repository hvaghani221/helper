package plugins

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

type apiServer struct {
	mu          sync.Mutex
	files       map[string][]byte
	urls        map[string]string
	hostAndPort string
	basePath    string
	remoteURL   map[string]string
}

func NewAPIServer(hostAndPort, basePath string, urls map[string]string) *apiServer {
	return &apiServer{
		files:       make(map[string][]byte),
		urls:        urls,
		mu:          sync.Mutex{},
		hostAndPort: hostAndPort,
		basePath:    basePath,
		remoteURL:   make(map[string]string),
	}
}

func (s *apiServer) ServeSchema(w http.ResponseWriter, r *http.Request) {
	fileName := mux.Vars(r)["api"]
	if fileName == "" || s.urls[fileName] == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	file, ok := s.files[fileName]
	if !ok {
		f, err := s.prepareSchema(fileName)
		if err != nil {
			log.Fatal(err)
		}
		file = f
	}

	w.Header().Add("Content-Type", "text/plain")
	_, _ = io.Copy(w, bytes.NewReader(file))
}

func (s *apiServer) APIRequester(w http.ResponseWriter, r *http.Request) {
	fileName := mux.Vars(r)["api"]
	if fileName == "" || s.urls[fileName] == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.Lock()
	if s.remoteURL[fileName] == "" {
		if _, err := s.prepareSchema(fileName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	s.mu.Unlock()

	path, ok := strings.CutPrefix(r.URL.Path, s.basePath+"/"+fileName)
	if !ok {
		http.Error(w, "Invalid URL: "+r.URL.Path, http.StatusInternalServerError)
	}

	remoteUrl := s.remoteURL[fileName] + path
	req, err := http.NewRequest(r.Method, remoteUrl, r.Body)
	if err != nil {
		log.Println("proxy error:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	req.URL.RawQuery = r.URL.RawQuery
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("proxy remote error:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)
}

func (s *apiServer) prepareSchema(name string) ([]byte, error) {
	doc, err := s.downloadSchema(s.urls[name])
	if err != nil {
		return nil, err
	}

	remoteURL := doc.Servers[0]
	// localURL := fmt.Sprintf("%s://%s%s/%s", "http", s.hostAndPort, s.basePath, name)
	localURL := fmt.Sprintf("%s/%s", s.basePath, name)

	doc.Servers = openapi3.Servers{
		&openapi3.Server{
			URL:         localURL,
			Description: remoteURL.Description,
		},
	}

	s.remoteURL[name] = remoteURL.URL

	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	s.files[name] = data
	return data, nil
}

func (s *apiServer) downloadSchema(url string) (*openapi3.T, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if strings.HasSuffix(url, ".yaml") || strings.HasSuffix(url, ".yml") {
		return loadYaml(resp.Body)
	} else if strings.HasSuffix(url, ".json") {
		return loadJson(resp.Body)
	}

	return nil, errors.New("unsupported file type")
}

func loadJson(reader io.Reader) (*openapi3.T, error) {
	var schema map[string]any
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes, &schema); err != nil {
		return nil, err
	}

	var doc *openapi3.T

	if schema["swagger"] == "2.0" {
		var o2 openapi2.T
		if err := json.Unmarshal(bytes, &o2); err != nil {
			return nil, err
		}
		if doc, err = openapi2conv.ToV3(&o2); err != nil {
			return nil, err
		}
	} else {
		if doc, err = openapi3.NewLoader().LoadFromData(bytes); err != nil {
			return nil, err
		}
	}
	return doc, nil
}

func loadYaml(reader io.Reader) (*openapi3.T, error) {
	var schema map[string]any
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(bytes, &schema); err != nil {
		return nil, err
	}

	var doc *openapi3.T

	if schema["swagger"] == "2.0" {
		var o2 openapi2.T
		if err := yaml.Unmarshal(bytes, &o2); err != nil {
			return nil, err
		}
		if doc, err = openapi2conv.ToV3(&o2); err != nil {
			return nil, err
		}
	} else {
		if doc, err = openapi3.NewLoader().LoadFromData(bytes); err != nil {
			return nil, err
		}
	}
	return doc, nil
}

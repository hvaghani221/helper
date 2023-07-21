package plugins

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	"golang.org/x/exp/slices"
)

type plugin struct {
	apis  map[string]string
	tmpl  *template.Template
	names []string
}

func NewPlugin(fs embed.FS, apis map[string]string) *plugin {
	tmpl := template.Must(template.ParseFS(fs, "static/plugin.html"))
	names := make([]string, 0, len(apis))
	for keys := range apis {
		names = append(names, keys)
	}

	slices.Sort(names)

	return &plugin{
		apis:  apis,
		tmpl:  tmpl,
		names: names,
	}
}

func (p *plugin) PageHandler(w http.ResponseWriter, r *http.Request) {
	_ = p.tmpl.Execute(w, map[string]any{"names": p.names})
}

func (p *plugin) PluginHandler(w http.ResponseWriter, r *http.Request) {
	api := mux.Vars(r)["api"]

	if p.apis[api] == "" {
		http.NotFound(w, r)
		return
	}

	url := fmt.Sprintf("/schema/%s", api)
	_ = p.tmpl.Execute(w, map[string]any{"url": url, "names": p.names})
}

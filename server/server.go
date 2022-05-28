package server

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"development.thatwebsite.xyz/gokrazy/acron/config"
	"development.thatwebsite.xyz/gokrazy/acron/templates"
	"gopkg.in/yaml.v3"
)

type Server struct {
	mux *http.ServeMux
	cfg *config.Config
}

func New(cfg *config.Config) *Server {
	return &Server{
		mux: http.NewServeMux(),
		cfg: cfg,
	}
}

func (s *Server) Routes() {
	s.mux.HandleFunc("/config", s.handleConfig)
	s.mux.HandleFunc("/logs", s.handleLogs)
	s.mux.HandleFunc("/", s.handleIndex)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func newTemplate() *template.Template {
	return template.New("base.html").Funcs(template.FuncMap{
		"getLastRunOn": func(j *config.Job) string {
			if val := j.GetLastRunOn(); val != nil {
				return val.Format("2006-01-02 15:04:05")
			}
			return "N/A"
		},
		"coalesce": func(a, b string) string {
			if a == "" {
				return b
			}
			return a
		},
		"toPrettyJSON": func(a interface{}) string {
			if s, err := json.MarshalIndent(a, "", "  "); err == nil {
				return string(s[:])
			} else {
				return err.Error()
			}
		},
		"toPrettyYAML": func(a interface{}) string {
			if s, err := yaml.Marshal(a); err == nil {
				return string(s[:])
			} else {
				return err.Error()
			}
		},
		"getLastLog": func(j *config.Job, outputType string) string {
			return j.GetLastRunLog(outputType)
		},
		"getLastTimeTaken": func(j *config.Job) string {
			return j.GetLastRunDuration().Truncate(time.Second).String()
		},
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {

	t, err := newTemplate().ParseFS(templates.FS, "base.html", "list.html")
	if err != nil {
		log.Println("error parsing: ", err)
		return
	}

	if err := t.ExecuteTemplate(w, "base.html", s.cfg); err != nil {
		log.Println(err)
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	t, err := newTemplate().ParseFS(templates.FS, "base.html", "config.html")
	if err != nil {
		log.Println("error parsing: ", err)
		return
	}

	if err := t.ExecuteTemplate(w, "base.html", s.cfg); err != nil {
		log.Println(err)
	}
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	t, err := newTemplate().ParseFS(templates.FS, "base.html", "logs.html")
	if err != nil {
		log.Println("error parsing: ", err)
		return
	}
	idxString := r.URL.Query().Get("idx")
	idx, err := strconv.Atoi(idxString)
	if err != nil {
		log.Println(err)
		return
	}

	if err := t.ExecuteTemplate(w, "base.html", s.cfg.Jobs[idx]); err != nil {
		log.Println(err)
	}
}

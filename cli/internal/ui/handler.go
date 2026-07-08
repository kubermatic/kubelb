/*
Copyright 2026 The KubeLB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ui

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"k8c.io/kubelb/pkg/conversion"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed templates/*.html
var templateFS embed.FS

// Server handles HTTP requests for the web UI
type Server struct {
	client    client.Client
	opts      *conversion.Options
	templates *template.Template
}

// NewServer creates a new UI server
func NewServer(k8sClient client.Client, opts *conversion.Options) (*Server, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &Server{
		client:    k8sClient,
		opts:      opts,
		templates: tmpl,
	}, nil
}

// Handler returns the HTTP handler for the UI
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static pages
	mux.HandleFunc("GET /", s.handleIndex)

	// API endpoints
	mux.HandleFunc("GET /api/ingresses", s.handleListIngresses)
	mux.HandleFunc("GET /api/ingresses/{namespace}/{name}/preview", s.handlePreview)
	mux.HandleFunc("POST /api/ingresses/{namespace}/{name}/convert", s.handleConvert)
	mux.HandleFunc("DELETE /api/ingresses/{namespace}/{name}", s.handleDeleteIngress)
	mux.HandleFunc("POST /api/ingresses/{namespace}/{name}/skip", s.handleSkipIngress)
	mux.HandleFunc("POST /api/ingresses/convert", s.handleBatchConvert)
	mux.HandleFunc("POST /api/ingresses/preview", s.handleBatchPreview)
	mux.HandleFunc("GET /api/routes", s.handleListRoutes)
	mux.HandleFunc("GET /api/annotations", s.handleAnnotations)
	mux.HandleFunc("GET /api/resources/{kind}/{namespace}/{name}", s.handleResourcePreview)
	mux.HandleFunc("DELETE /api/resources/{kind}/{namespace}/{name}", s.handleResourceDelete)

	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	if err := s.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

package pages

import (
	"net/http"
	"html/template"
)

type Context struct {
	r      *http.Request
	Locale string
	Page   string
	route  *Route
	html   template.HTML
	data   map[string]interface{}
}

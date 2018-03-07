package pages

import (
	"html/template"
)

type Context struct {
	Page  string
	html  template.HTML
	data  map[string]interface{}
}

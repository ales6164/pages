package pages

import (
	"html/template"
)

type Context struct {
	Vars  map[string]string
	Query map[string]interface{}
	Page  string

	html template.HTML
	/*data map[string]interface{}*/
}

package pages

import (
	"path"
	"github.com/gorilla/mux"
	"bytes"
	"net/http"
	"io/ioutil"
	"github.com/cbroglie/mustache"
	"github.com/PuerkitoBio/goquery"
	"errors"
	"regexp"
)

type Pages struct {
	*mux.Router
	*Options
	*Manifest
	partials   mustache.StaticProvider
	templates  map[string]*mustache.Template
	documents  map[string]*goquery.Document
	routeCount int
}

type Options struct {
	base         string
	IsRendering  bool
	JsonFilePath string
}

var (
	DefaultOutlet = "#outlet"
)

func New(opt *Options) (*Pages, error) {
	p := &Pages{
		Options:   opt,
		Router:    mux.NewRouter(),
		Manifest:  new(Manifest),
		templates: map[string]*mustache.Template{},
		documents: map[string]*goquery.Document{},
	}

	// read manifest
	err := readAndUnmarshal(p.JsonFilePath, p.Manifest)
	if err != nil {
		return p, err
	}

	// set base path from calling script absolute path and settings.json dir
	p.base = path.Dir(p.JsonFilePath)

	// parse templates
	var partials = map[string]string{}
	for _, imp := range p.Imports {
		if !path.IsAbs(imp.URL) {
			imp.URL = path.Join(p.base, imp.URL)
		}

		// read templates and load into map

		templateBytes, err := ioutil.ReadFile(imp.URL)
		if err != nil {
			return p, err
		}

		partials[imp.Name] = string(templateBytes)

	}

	p.partials = mustache.StaticProvider{Partials: partials}

	// parse partials and replace {{>partial}} with template
	// is this even needed here?
	for name, part := range partials {
		p.templates[name], err = mustache.ParseStringPartials(part, &p.partials)
		if err != nil {
			return p, err
		}
	}

	for name, temp := range p.partials.Partials {
		buf := new(bytes.Buffer)
		buf.WriteString(temp)
		doc, err := goquery.NewDocumentFromReader(buf)
		if err != nil {
			return p, err
		}
		p.documents[name] = doc
	}

	return p, nil
}

func (p *Pages) iter(h map[string][]*Route, route *Route, basePath string, parents []*Route) map[string][]*Route {
	p.routeCount += 1

	route.parents = parents
	route.id = p.routeCount

	newPath := path.Join(basePath, route.Path)

	h[newPath] = append(h[newPath], parents...)
	h[newPath] = append(h[newPath], route)

	if len(route.Children) > 0 {
		ps := append(parents, route)
		for _, childRoute := range route.Children {
			h = p.iter(h, childRoute, newPath, ps)
		}
	}
	return h
}

func (p *Pages) BuildRouter() (err error) {
	p.Router = mux.NewRouter()
	p.routeCount = -1

	// attaches routes to paths - this way we don't have two Handlers for the same path

	var handle = map[string][]*Route{}
	for _, route := range p.Routes {
		handle = p.iter(handle, route, "/", nil)
	}

	for routePath, routes := range handle {
		err = p.handleRoute(p.Router, routePath, routes)
		if err != nil {
			return err
		}
	}

	return err
}

// one path can have multiple routes defined -> when having multiple routers on one page
func (p *Pages) handleRoute(r *mux.Router, path string, routes []*Route) (err error) {
	//mux.NewRouter().PathPrefix(opt.HandlerPathPrefix).Subrouter(),
	var document = p.documents["index"].Clone()

	var done = map[int]bool{}
	//var routesToHandle []*Route
	for _, route := range routes {
		// one path match with multiple routes
		// how to handle multiple routes?
		// compare if it's been handled already

		if _, ok := done[route.id]; ok {
			continue
		}
		done[route.id] = true

		// set outlet
		outlet := route.Outlet
		if len(outlet) == 0 {
			outlet = DefaultOutlet
		}

		outletSelection := document.Find(outlet)
		if outletSelection.Length() == 0 {
			return errors.New("can't find router outlet " + outlet)
		}

		component := p.documents[route.Component].Clone()
		if component.Children().Length() == 0 {
			return errors.New("component empty " + route.Component)
		}

		componentHtml, err := component.Html()
		if err != nil {
			return err
		}

		outletSelection.SetHtml(componentHtml)
	}

	html, err := document.Html()
	if err != nil {
		return err
	}

	html = regexp.MustCompile(`{{\s*(&gt;)`).ReplaceAllString(html, "{{>")

	temp, err := mustache.ParseStringPartials(html, &p.partials)
	if err != nil {
		return err
	}

	r.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		//vars := mux.Vars(r)

		temp.FRender(w, map[string]interface{}{

		})
		//w.Write([]byte(html))
	})

	return err
}

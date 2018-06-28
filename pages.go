package pages

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strings"
	"path/filepath"
	"path"
	"io/ioutil"
	"google.golang.org/appengine"
	"net/url"
	"google.golang.org/appengine/urlfetch"
	"bytes"
	"encoding/json"
	"regexp"
	"github.com/aymerick/raymond"
	"errors"
	"github.com/gorilla/sessions"
)

type Pages struct {
	router  *httprouter.Router
	session *sessions.CookieStore
	locale  string // current locale
	*Options
	*Manifest
	Components map[string]*Component
	routeCount int
}

type Options struct {
	base               string
	IsRendering        bool
	JsonFilePath       string
	ForceSSL           bool
	EnableSessionStore bool
	SessionKey         []byte // key must be 16, 24 or 32 bytes long (AES-128, AES-192 or AES-256)
	ForceSubDomain     string
	forceSubDomain     bool
}

var (
	DefaultOutlet = "router-outlet"
	DefaultLayout = "index"
)

func (p *Pages) withMiddleware(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		proto := r.Header.Get("x-forwarded-proto")
		if p.ForceSSL {
			if proto == "http" {
				http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
				return
			}
		}
		if p.forceSubDomain {
			x := strings.Split(r.Host, ".")
			if x[0] != p.ForceSubDomain {
				http.Redirect(w, r, "https://"+p.ForceSubDomain+"."+r.Host+r.RequestURI, http.StatusMovedPermanently)
				return
			}
		}
		next(w, r, ps)
	}
}

func New(opt *Options) (*Pages, error) {
	p := &Pages{
		Options:    opt,
		Manifest:   new(Manifest),
		Components: map[string]*Component{},
	}

	if opt.EnableSessionStore {
		if len(opt.SessionKey) == 0 || len(opt.SessionKey)%8 != 0 {
			return p, errors.New("invalid session store key length")
		}
		p.session = sessions.NewCookieStore(opt.SessionKey)
	}

	// read manifest
	err := readAndUnmarshal(p.JsonFilePath, p.Manifest)
	if err != nil {
		return p, err
	}

	// add necessary fields
	if p.Manifest.Resources == nil {
		p.Manifest.Resources = &Resources{
			Translations: map[string]map[string]string{},
		}
	} else if p.Manifest.Resources.Translations == nil {
		p.Manifest.Resources.Translations = map[string]map[string]string{}
	}

	// set base path from calling script absolute path and settings.json dir
	p.base = filepath.Dir(p.JsonFilePath)

	// read partials
	for _, imp := range p.Imports {
		if len(imp.Path) > 0 {
			// single file definition
			if !filepath.IsAbs(imp.Path) {
				imp.Path = filepath.Join(p.base, imp.Path)
			}
			newC, err := NewComponent(imp.Name, imp.Path, imp.IsLayout)
			if err != nil {
				return p, err
			}
			p.Components[imp.Name] = newC
			if err != nil {
				return p, err
			}
		}
	}

	return p, nil
}

func (p *Pages) iter(h map[string][]*Route, route *Route, basePath string, parents []*Route) map[string][]*Route {
	p.routeCount += 1

	route.parents = parents
	route.id = p.routeCount

	newPath := path.Join(basePath, route.Path)
	if route.Path == "/" {
		newPath += "/"
	}

	h[newPath] = append(h[newPath], parents...)

	// this IF is because we don't want to render a path that has children by it's own - should always be rendered only when rendering with child path
	if len(route.Children) == 0 {
		h[newPath] = append(h[newPath], route)
	}

	if len(route.Children) > 0 {
		ps := append(parents, route)
		for _, childRoute := range route.Children {
			h = p.iter(h, childRoute, newPath, ps)
		}
	}
	return h
}

func (p *Pages) BuildRouter() (*httprouter.Router, error) {
	p.router = httprouter.New()
	p.routeCount = -1

	// add i18n helper
	raymond.RegisterHelper("i18n", func(key string) string {
		return p.Manifest.Resources.Translations[p.locale][key]
	})

	// attaches routes to paths - this way we don't have two Handlers for the same path

	var handle = map[string][]*Route{}
	for _, route := range p.Routes {
		handle = p.iter(handle, route, "/", nil)
	}

	for routePath, routes := range handle {
		err := p.handleRoute(p.router, routePath, routes)
		if err != nil {
			return p.router, err
		}
	}

	// serve static files
	public := path.Join(p.base, "public")
	files, err2 := ioutil.ReadDir(public)
	if err2 == nil {
		for _, file := range files {
			if file.IsDir() {
				p.router.ServeFiles("/"+file.Name()+"/*filepath", http.Dir(path.Join(public, file.Name())))
			} else {
				p.router.Handler(http.MethodGet, "/"+file.Name(), http.FileServer(http.Dir(public)))
			}
		}
	}

	return p.router, nil
}

// one path can have multiple routes defined -> when having multiple routers on one page
func (p *Pages) handleRoute(r *httprouter.Router, path string, routes []*Route) (err error) {
	var layout = DefaultLayout

	if len(routes) > 0 {
		if r := routes[0]; r != nil && len(r.Layout) > 0 {
			layout = r.Layout
		}
	}

	context, templ, apiUri, err := p.RenderRoute(p.Components[layout], routes)
	if err != nil {
		return err
	}

	var hasApi = len(apiUri) > 0

	var handleFunc httprouter.Handle = func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		ctx := appengine.NewContext(req)

		context["query"] = map[string]string{}

		for _, v := range ps {
			context["query"].(map[string]string)[v.Key] = v.Value
		}

		// read lang
		p.locale = p.DefaultLocale
		if lang, err := req.Cookie("lang"); err == nil {
			p.locale = lang.Value
		} else {
			req.AddCookie(&http.Cookie{Name: "lang", Value: p.DefaultLocale, Path: "/", MaxAge: 60 * 60 * 24 * 30 * 12})
		}
		context["locale"] = p.locale
		context["translations"] = p.Resources.Translations[p.locale]

		// add query parameters to the api request
		if hasApi {
			resolvedApiUri := regex.ReplaceAllStringFunc(apiUri, func(s string) string {
				context["query"].(map[string]string)[s[1:]] = ps.ByName(s[1:])
				return ps.ByName(s[1:])
			})

			apiUrl, err := url.Parse(resolvedApiUri)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			apiUrlQuery := apiUrl.Query()
			reqQuery := req.URL.Query()
			for paramName, val := range reqQuery {
				for _, v := range val {
					apiUrlQuery.Add(paramName, v)
				}
			}
			apiUrl.RawQuery = apiUrlQuery.Encode()

			client := urlfetch.Client(ctx)
			resp, err := client.Get(apiUrl.String())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			var data map[string]interface{}
			err = json.Unmarshal(buf.Bytes(), &data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			context["data"] = data
		}

		jsonContext, _ := json.Marshal(context)
		context["json"] = string(jsonContext)

		html, err := templ.Exec(context)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(html))
	}

	p.forceSubDomain = len(p.ForceSubDomain) > 0

	if !appengine.IsDevAppServer() && (p.ForceSSL || p.forceSubDomain) {
		r.GET(path, p.withMiddleware(handleFunc))
	} else {
		r.GET(path, handleFunc)
	}

	return err
}

var (
	regex = regexp.MustCompile(`\$(\w+)`)
)

func (p *Pages) RenderRoute(layout *Component, routes []*Route) (map[string]interface{}, *raymond.Template, string, error) {
	var ctx = map[string]interface{}{}
	var body = layout.Template.Clone()
	var apiUri string
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

		if len(route.Api) > 0 {
			apiUri = route.Api
		}

		if route.Page != nil {
			for k, v := range route.Page {
				ctx[k] = v
			}
		}

		if component, ok := p.Components[route.Component]; ok {
			body.RegisterPartial(outlet, component.Raw)
		} else {
			return ctx, body, apiUri, errors.New("component " + route.Component + " doesn't exist")
		}

	}

	return ctx, body, apiUri, nil
}

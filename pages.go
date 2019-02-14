package pages

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/aymerick/raymond"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
)

type Pages struct {
	router  *mux.Router
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
	ForceHostname      string
	forceHostname      bool
}

var (
	DefaultOutlet = "router-outlet"
	DefaultLayout = "index"
)

func (p *Pages) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proto := r.Header.Get("x-forwarded-proto")
		if p.ForceSSL {
			if proto == "http" {
				http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
				return
			}
		}
		if p.forceHostname && getHost(r) != p.ForceHostname {
			http.Redirect(w, r, "https://"+p.ForceHostname+r.RequestURI, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
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
		if len(imp.ComponentPath) > 0 {
			if !filepath.IsAbs(imp.ComponentPath) {
				imp.ComponentPath = filepath.Join(p.base, imp.ComponentPath)
			}
		}
		if len(imp.TemplatePath) > 0 {

			// single file definition
			if !filepath.IsAbs(imp.TemplatePath) {
				imp.TemplatePath = filepath.Join(p.base, imp.TemplatePath)
			}

			newC, err := NewComponent(imp)
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

func (p *Pages) BuildRouter() (*mux.Router, error) {
	p.router = mux.NewRouter()
	p.routeCount = -1

	// add i18n helper
	raymond.RegisterHelper("i18n", func(key string) string {
		return p.Manifest.Resources.Translations[p.locale][key]
	})

	// add json helper
	raymond.RegisterHelper("json", func(k interface{}) string {
		d, _ := json.Marshal(k)
		return string(d)
	})

	// serve components
	p.router.HandleFunc("/"+p.Manifest.ComponentsVersion+".js", func(w http.ResponseWriter, r *http.Request) {
		var lang = r.URL.Query().Get("lang")
		if len(lang) == 0 {
			lang = p.DefaultLocale
		}
		resources := map[string]interface{}{
			"translations": p.Resources.Translations[lang],
			"storage":      p.Resources.Storage,
		}
		res, _ := json.Marshal(resources)
		w.Write([]byte(p.Manifest.Components[0] + string(res) + p.Manifest.Components[1]))
	})

	// serve self-contained components
	for _, c := range p.Components {
		p.router.HandleFunc("/"+c.Name+".component.js", func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/javascript")
			writer.Write(c.RawSelfContained)
		})
	}

	// serve static files
	public := path.Join(p.base, "public")
	files, err2 := ioutil.ReadDir(public)
	if err2 == nil {
		for _, file := range files {
			if file.IsDir() {
				p.router.PathPrefix("/" + file.Name()).Handler(http.StripPrefix("/"+file.Name(), http.FileServer(http.Dir(path.Join(public, file.Name())))))
			} else {
				p.router.Handle("/"+file.Name(), http.FileServer(http.Dir(public)))
			}
		}
	}

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

	return p.router, nil
}

var cachedPages = map[string][]byte{}

// one path can have multiple routes defined -> when having multiple routers on one page
func (p *Pages) handleRoute(r *mux.Router, path string, routes []*Route) (err error) {
	var layout = DefaultLayout

	if len(routes) > 0 {
		if r := routes[0]; r != nil && len(r.Layout) > 0 {
			layout = r.Layout
		}
	}

	context, templ, requests, redirect, cache, err := p.RenderRoute(p.Components[layout], routes)
	if err != nil {
		return err
	}

	var hasApi = len(requests) > 0

	var handleFunc http.HandlerFunc
	if len(redirect) > 0 {
		handleFunc = func(w http.ResponseWriter, req *http.Request) {
			vars := mux.Vars(req)

			resolvedRedirectUri := regex.ReplaceAllStringFunc(redirect, func(s string) string {
				return vars[s[1:]]
			})

			http.Redirect(w, req, resolvedRedirectUri, http.StatusPermanentRedirect)
		}
	} else if cache {
		handleFunc = func(w http.ResponseWriter, req *http.Request) {
			ctx := appengine.NewContext(req)

			var cacheKey = req.URL.Host + "/" + req.URL.Path

			if it, ok := cachedPages[cacheKey]; ok {
				w.Write(it)
				return
			}

			context["query"] = map[string]string{}

			vars := mux.Vars(req)
			for key, val := range vars {
				context["query"].(map[string]string)[key] = val
			}

			// read lang
			p.locale = p.DefaultLocale
			if lang, err := req.Cookie("lang"); err == nil {
				p.locale = lang.Value
			} else {
				req.AddCookie(&http.Cookie{Name: "lang", Value: p.DefaultLocale, Path: "/", MaxAge: 60 * 60 * 24 * 30 * 12})
			}
			context["locale"] = p.locale

			// add query parameters to the api request
			if hasApi {
				var dataArray = make([]interface{}, len(requests))

				for index, r := range requests {
					resolvedApiUri := regex.ReplaceAllStringFunc(r.URL, func(s string) string {
						context["query"].(map[string]string)[s[1:]] = vars[s[1:]]
						return vars[s[1:]]
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

					req, err := http.NewRequest(r.Method, apiUrl.String(), nil)
					for key, value := range r.Headers {
						req.Header.Add(key, value)
					}
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					resp, err := client.Do(req)
					if resp.StatusCode != http.StatusOK {
						http.Error(w, http.StatusText(resp.StatusCode), resp.StatusCode)
						return
					}
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					buf := new(bytes.Buffer)
					buf.ReadFrom(resp.Body)
					var data interface{}
					err = json.Unmarshal(buf.Bytes(), &data)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					dataArray[index] = data
				}

				context["data"] = dataArray
			}

			jsonContext, _ := json.Marshal(context)
			context["json"] = string(jsonContext)

			html, err := templ.Exec(context)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write([]byte(html))

			cachedPages[cacheKey] = []byte(html)

			/*memcache.Set(ctx, &memcache.Item{
				Key:        req.URL.Host + "/" + req.URL.Path,
				Value:      []byte(html),
				Expiration: time.Hour * 24,
			})*/
		}
	} else {
		handleFunc = func(w http.ResponseWriter, req *http.Request) {
			ctx := appengine.NewContext(req)

			context["query"] = map[string]string{}

			vars := mux.Vars(req)
			for key, val := range vars {
				context["query"].(map[string]string)[key] = val
			}

			// read lang
			p.locale = p.DefaultLocale
			if lang, err := req.Cookie("lang"); err == nil {
				p.locale = lang.Value
			} else {
				req.AddCookie(&http.Cookie{Name: "lang", Value: p.DefaultLocale, Path: "/", MaxAge: 60 * 60 * 24 * 30 * 12})
			}
			context["locale"] = p.locale

			// add query parameters to the api request
			if hasApi {
				var dataArray = make([]interface{}, len(requests))

				for index, r := range requests {
					resolvedApiUri := regex.ReplaceAllStringFunc(r.URL, func(s string) string {
						context["query"].(map[string]string)[s[1:]] = vars[s[1:]]
						return vars[s[1:]]
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

					req, err := http.NewRequest(r.Method, apiUrl.String(), nil)
					for key, value := range r.Headers {
						req.Header.Add(key, value)
					}
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					resp, err := client.Do(req)
					if resp.StatusCode != http.StatusOK {
						http.Error(w, http.StatusText(resp.StatusCode), resp.StatusCode)
						return
					}
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					buf := new(bytes.Buffer)
					buf.ReadFrom(resp.Body)
					var data interface{}
					err = json.Unmarshal(buf.Bytes(), &data)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					dataArray[index] = data
				}

				context["data"] = dataArray
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
	}

	p.forceHostname = len(p.ForceHostname) > 0

	r.Handle(path, p.withMiddleware(handleFunc))

	return err
}

var (
	regex = regexp.MustCompile(`\$(\w+)`)
)

type Context struct {
	Vars  map[string]string
	Query map[string]interface{}
	Page  string

	html template.HTML
	/*data map[string]interface{}*/
}

func (p *Pages) RenderRoute(layout *Component, routes []*Route) (map[string]interface{}, *raymond.Template, []Request, string, bool, error) {
	var ctx = map[string]interface{}{}
	var body = layout.Template.Clone()
	var requests []Request
	var redirect string
	var done = map[int]bool{}
	var cache bool

	//var routesToHandle []*Route
	for _, route := range routes {
		// one path match with multiple routes
		// how to handle multiple routes?
		// compare if it's been handled already

		if _, ok := done[route.id]; ok {
			continue
		}
		done[route.id] = true

		if !cache && route.Cache {
			cache = route.Cache
		}

		// redirect?
		if len(route.Redirect) > 0 {
			redirect = route.Redirect
			break
		}

		// set outlet
		outlet := route.Outlet
		if len(outlet) == 0 {
			outlet = DefaultOutlet
		}

		requests = route.Requests

		if route.Page != nil {
			for k, v := range route.Page {
				ctx[k] = v
			}
		}

		if len(route.Component) > 0 {
			if component, ok := p.Components[route.Component]; ok {
				if component.Render {
					body.RegisterPartial(outlet, "<"+component.Name+">"+component.Raw+"</"+component.Name+">")
				} else {
					body.RegisterPartial(outlet, "<"+component.Name+"></"+component.Name+">")
				}
			} else {
				return ctx, body, requests, redirect, cache, errors.New("component " + route.Component + " doesn't exist")
			}
		}

	}

	return ctx, body, requests, redirect, cache, nil
}

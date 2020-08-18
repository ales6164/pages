package pages

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/ales6164/raymond"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"log"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

const (
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
		if p.forceHostname && r.Host != p.ForceHostname {
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

	// parse resources
	err = json.Unmarshal(p.Manifest.Resources, &p.Manifest.parsedResources)
	if err != nil {
		return p, err
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
		}
	}

	return p, nil
}

func (p *Pages) iter(h map[string][]*Route, route *Route, basePath string, parents []*Route) map[string][]*Route {
	p.routeCount += 1

	route.parents = parents
	route.id = p.routeCount

	newPath := path.Join(basePath, route.Path)
	if len(route.Path) > 1 && route.Path[len(route.Path)-1:] == "/" {
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

	// add json helper
	raymond.RegisterHelper("stringify", func(k interface{}) string {
		d, _ := json.Marshal(k)
		return string(d)
	})

	// add translation helper
	raymond.RegisterHelper("trans", func(locale string, k string) string {
		v, err := p.Manifest.GetResource("translations", locale, k)
		if err != nil {
			return k
		}
		return v
	})

	// append string helper
	raymond.RegisterHelper("append", func(k1, k2 string) string {
		return k1 + k2
	})

	raymond.RegisterHelper("i18n", func(k string) string {
		v, err := p.Manifest.GetResource("translations", p.Manifest.DefaultLocale, k)
		if err != nil {
			return k
		}
		return v
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

	return p.router, nil
}

//var cachedPages = map[string][]byte{}

// one path can have multiple routes defined -> when having multiple routers on one page
func (p *Pages) handleRoute(r *mux.Router, path string, routes []*Route) (err error) {
	var layout = DefaultLayout

	if len(routes) > 0 {
		if r := routes[0]; r != nil && len(r.Layout) > 0 {
			layout = r.Layout
		}
	}

	routerPageVars, templ, requests, redirect, _, err := p.RenderRoute(p.Components[layout], routes)
	if err != nil {
		return err
	}

	var hasApi = len(requests) > 0

	var handleFunc http.HandlerFunc
	//var resolvedRedirectUri string
	if len(redirect) > 0 {
		handleFunc = func(w http.ResponseWriter, req *http.Request) {
			// todo: doesnt work
			/*vars := mux.Vars(req)

			resolvedRedirectUri = regex.ReplaceAllStringFunc(redirect, func(s string) string {
				return vars[s[1:]]
			})*/

			http.Redirect(w, req, redirect, http.StatusPermanentRedirect)
		}
	} else {
		handleFunc = func(w http.ResponseWriter, req *http.Request) {
			_ = req.Body.Close()

			pageContext := map[string]interface{}{
				"storage": p.parsedResources,
			}
			if routerPageVars != nil {
				for k, v := range routerPageVars {
					pageContext[k] = v
				}
			}

			vars := mux.Vars(req)

			pageContext["query"] = vars

			/*vars := mux.Vars(req)
			for key, val := range vars {
				pageContext["query"].(map[string]string)[key] = val
			}*/

			// read lang
			/*locale := p.DefaultLocale
			if lang, err := req.Cookie("lang"); err == nil {
				locale = lang.Value
			} else {
				req.AddCookie(&http.Cookie{Name: "lang", Value: p.DefaultLocale, Path: "/", MaxAge: 60 * 60 * 24 * 30 * 12})
			}
			pageContext["locale"] = locale*/

			splitPath := strings.Split(req.URL.EscapedPath(), "/")

			if len(splitPath) > 1 && len(splitPath[1]) == 2 {
				// alternate url
				pageContext["alternate"] = "/" + strings.Join(splitPath[2:], "/")
			} else {
				pageContext["alternate"] = strings.Join(splitPath, "/")
			}

			// add query parameters to the api request
			if hasApi {
				var dataArray = make([]interface{}, len(requests))

				for index, r := range requests {
					resolvedApiUri := regex.ReplaceAllStringFunc(r.URL, func(s string) string {
						pageContext["query"].(map[string]string)[s[1:]] = vars[s[1:]]
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

					var client = &http.Client{
						Timeout: time.Second * 10,
					}

					/*var reader io.Reader
					if r.BodyReader != nil {
						reader = r.BodyReader
					}*/

					var req *http.Request
					var resp *http.Response
					buf := new(bytes.Buffer)

					if r.Body != nil {
						newBody := regex.ReplaceAllStringFunc(string(r.Body), func(s string) string {
							pageContext["query"].(map[string]string)[s[1:]] = vars[s[1:]]
							return vars[s[1:]]
						})

						buf.WriteString(newBody)
						req, err = http.NewRequest(r.Method, apiUrl.String(), buf)
					} else {
						req, err = http.NewRequest(r.Method, apiUrl.String(), nil)
					}

					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					for key, value := range r.Headers {
						req.Header.Add(key, value)
					}

					resp, err = client.Do(req)
					buf.Reset()

					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					if resp.StatusCode != http.StatusOK {
						http.Error(w, http.StatusText(resp.StatusCode), resp.StatusCode)
						return
					}

					_, _ = buf.ReadFrom(resp.Body)
					_ = resp.Body.Close()

					var data interface{}
					err = json.Unmarshal(buf.Bytes(), &data)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					dataArray[index] = data
					buf.Reset()
				}

				pageContext["data"] = dataArray
			}

			jsonContext, err := json.Marshal(pageContext)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			pageContext["contextObject"] = string(jsonContext)

			html, err := templ.Exec(pageContext)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(html))
		}
	}

	p.forceHostname = len(p.ForceHostname) > 0

	if path[len(path)-1:] == "*" {
		// catch all handler
		log.Printf("Catch all handler on path %s", path[:len(path)-1])
		r.PathPrefix(path[:len(path)-1]).Handler(p.withMiddleware(handleFunc))
	} else {
		if len(redirect) > 0 {
			log.Printf("Redirect handler on path %s leading to %s", path, redirect)
		} else {
			log.Printf("Handler on path %s", path)
		}

		r.Handle(path, p.withMiddleware(handleFunc))
	}

	return err
}

var (
	regex = regexp.MustCompile(`\$(\w+)`)
)

func (p *Pages) RenderRoute(layout *Component, routes []*Route) (map[string]interface{}, *raymond.Template, []Request, string, bool, error) {
	var body = layout.Template.Clone()
	var requests []Request
	var redirect string
	var done = map[int]bool{}
	var cache bool
	var routePage map[string]interface{}

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

		routePage = route.Page

		// set outlet
		outlet := route.Outlet
		if len(outlet) == 0 {
			outlet = DefaultOutlet
		}

		requests = route.Requests

		if len(route.Component) > 0 {
			if component, ok := p.Components[route.Component]; ok {
				body.RegisterPartial(outlet, "{{> "+component.Name+"}}")
			} else {
				return route.Page, body, requests, redirect, cache, errors.New("component " + route.Component + " doesn't exist")
			}
		}

	}

	return routePage, body, requests, redirect, cache, nil
}

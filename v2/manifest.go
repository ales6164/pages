package pages

import (
	"encoding/json"
	"github.com/buger/jsonparser"
)

type Manifest struct {
	DefaultLocale     string          `json:"defaultLocale"`
	Imports           []*Import       `json:"imports"`
	Routes            []*Route        `json:"routes"`
	Resources         json.RawMessage `json:"resources"`
	parsedResources   interface{}
	ComponentsVersion string `json:"componentsVersion"`
}

type Import struct {
	TemplatePath  string `json:"templatePath"`
	ComponentPath string `json:"componentPath"`
	Name          string `json:"name"`
	IsLayout      bool   `json:"layout"`
	Render        bool   `json:"render"`
	OmitTags      bool   `json:"omitTags"`
}

func (m *Manifest) GetResource(keys ...string) (string, error) {
	return jsonparser.GetString(m.Resources, keys...)
}

/*
path is a string that uses the route matcher DSL.
pathMatch is a string that specifies the matching strategy.
matcher defines a custom strategy for path matching and supersedes path and pathMatch.
component is a component type.
redirectTo is the url fragment which will replace the current matched segment.
outlet is the name of the outlet the component should be placed into.
canActivate is an array of DI tokens used to look up CanActivate handlers. See CanActivate for more info.
canActivateChild is an array of DI tokens used to look up CanActivateChild handlers. See CanActivateChild for more info.
canDeactivate is an array of DI tokens used to look up CanDeactivate handlers. See CanDeactivate for more info.
canLoad is an array of DI tokens used to look up CanLoad handlers. See CanLoad for more info.
data is additional data provided to the component via ActivatedRoute.
resolve is a map of DI tokens used to look up data resolvers. See Resolve for more info.
runGuardsAndResolvers defines when guards and resolvers will be run. By default they run only when the matrix parameters of the route change. When set to paramsOrQueryParamsChange they will also run when query params change. And when set to always, they will run every time.
children is an array of child route definitions.
loadChildren is a reference to lazy loaded child routes. See LoadChildren for more info.
*/
type Route struct {
	id        int                    // used for route handling
	Path      string                 `json:"path"`
	Component string                 `json:"component"`
	Layout    string                 `json:"layout"`
	Requests  []Request              `json:"requests"`
	Outlet    string                 `json:"outlet"`
	Children  []*Route               `json:"children"`
	Page      map[string]interface{} `json:"page"`
	Redirect  string                 `json:"redirect"`
	Cache     bool                   `json:"cache"`

	CanActivate      interface{} `json:"canActivate"`      // not implemented
	CanActivateChild interface{} `json:"canActivateChild"` // not implemented
	PathMatch        interface{} `json:"pathMatch"`        // not implemented

	parents []*Route
}

type Request struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

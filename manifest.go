package pages

type Manifest struct {
	API string `json:"api"`
	/*Resources     []*Resource `json:"resources"`
	Name          string      `json:"name"`*/
	Dist      string   `json:"dist"`
	Templates []string `json:"templates"`
	/*Files         []string    `json:"files"`*/
	Routers []*Router `json:"routers"`
	Layout  string    `json:"layout"`
	/*DefaultLocale string      `json:"default_locale"`*/
}

/*type Resource struct {
	Src  string `json:"src"`
	Type string `json:"type"`
}*/

type Router struct {
	Layout string            `json:"layout"`
	Handle map[string]*Route `json:"handle"`
	/*Auth   *Auth             `json:"auth"`*/
}

type Route struct {
	locale      string
	pattern     string
	layout      string
	Layout      string      `json:"layout"`
	Page        string      `json:"page"`
	Alternative Alternative `json:"alternative"`
}

type Alternative map[string]string

type Auth struct {
	Login     string `json:"login"`
	SignInURL string `json:"sign_in_url"`
}

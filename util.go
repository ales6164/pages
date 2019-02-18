package pages

import (
	"encoding/json"
	"google.golang.org/appengine"
	"io/ioutil"
	"net/http"
	"strings"
)

func readAndUnmarshal(filePath string, v interface{}) error {
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, v)
	file = []byte("")
	return err
}

// getHost tries its best to return the request host.
func getHost(r *http.Request) string {
	var host = r.URL.Host
	if r.URL.IsAbs() {
		host = r.Host
		// Slice off any port information.
		if i := strings.Index(host, ":"); i != -1 {
			host = host[:i]
		}
	}
	if len(host) == 0 {
		if appengine.IsDevAppServer() {
			host = appengine.DefaultVersionHostname(appengine.NewContext(r))
		}
	}
	return host
}

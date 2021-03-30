package core

import (
	"net/http"
)

type node struct {
	// mapping route path with route
	handlers map[methodTyp][]*patternHandler
}

// InsertRoute used to register new route with pattern and method type
func (n *node) InsertRoute(method methodTyp, pattern string, handler http.Handler, redirect bool) {
	handlers := n.handlers[method]
	for _, p := range handlers {
		if p.pat == pattern {
			return
		}
	}

	nodeHandler := &patternHandler{
		pat:      pattern,
		Handler:  handler,
		redirect: redirect,
	}
	n.handlers[method] = append(handlers, nodeHandler)

	patLen := len(pattern)
	if patLen > 0 && pattern[patLen-1] == '/' {
		n.InsertRoute(method, pattern[:patLen-1], http.HandlerFunc(addSlashRedirect), true)
	}
}

// FindRoute used to find route handler with method and path
func (n *node) FindRoute(rctx *Context, method methodTyp, path string) http.Handler {
	if _, ok := n.handlers[method]; !ok {
		rctx.methodNotAllowed = true
		return nil
	}

	for _, ph := range n.handlers[method] {
		if params, ok := ph.try(path); ok {
			rctx.RouteParams = params
			return ph
		}
	}

	return nil
}

func addSlashRedirect(w http.ResponseWriter, r *http.Request) {
	u := *r.URL
	u.Path += "/"
	http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
}

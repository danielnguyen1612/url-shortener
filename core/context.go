package core

import (
	"context"
	"net/url"
)

var (
	// RouteCtxKey is the context.Context key to store the request context.
	RouteCtxKey = &contextKey{"RouteContext"}
)

// Context is the default routing context set on the root of a
// request context to track route patterns
type Context struct {
	// Routing path/method override used during the route search.
	// See Mux#routeHTTP method.
	RoutePath   string
	RouteMethod string
	RouteParams url.Values

	// methodNotAllowed hint
	methodNotAllowed bool
}

// NewRouteContext returns a new routing Context object.
func NewRouteContext() *Context {
	return &Context{}
}

// RouteContext returns routing Context object from a
// http.Request Context.
func RouteContext(ctx context.Context) *Context {
	return ctx.Value(RouteCtxKey).(*Context)
}

// Reset a routing context to its initial state.
func (x *Context) Reset() {
	x.RoutePath = ""
	x.RouteMethod = ""

	x.methodNotAllowed = false
}

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "context value " + k.name
}

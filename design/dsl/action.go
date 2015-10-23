package dsl

import (
	"fmt"

	"bitbucket.org/pkg/inflect"
	. "github.com/raphael/goa/design"
)

// Action implements the action definition DSL. Action definitions describe specific API endpoints
// including the URL, HTTP method and request parameters (via path wildcards or query strings) and
// payload (data structure describing the request HTTP body). An action belongs to a resource and
// "inherits" default values from the resource definition including the URL path prefix, default
// response media type and default payload attribute properties (inherited from the attribute with
// identical name in the resource default media type). Action definitions also describe all the
// possible responses including the HTTP status, headers and body. Here is an example showing all
// the possible sub-definitions:
//
//	 Action("Update", func() {
//	     Description("Update account")
//	     Routing(
//	         PUT("/:id"),                     // Full action path is built by appending "/:id" to parent resource base path
//	         PUT("//orgs/:org/accounts/:id"), // The // prefix indicates an absolute path
//	     )
//	     Params(func() {                      // Params describe the action parameters
//	         Param("org", String)             // Parameters may correspond to path wildcards
//	         Param("id", Integer)
//	         Param("sort", func() {           // or URL query string values.
//			Enum("asc", desc")
//		 })
//	     })
//	     Headers(func() {                     // Headers describe relevant action headers
//	         Header("Authorization", String)
//	         Header("X-Account", Integer)
//	         Required("Authorization", "X-Account")
//	     })
//	     Payload(UpdatePayload)               // Payload describes the HTTP request body (here using a type)
//	     Response(NoContent)                  // Each possible HTTP response is described via Response
//	     Response(NotFound)
//	 })
func Action(name string, dsl func()) {
	if r, ok := resourceDefinition(true); ok {
		if r.Actions == nil {
			r.Actions = make(map[string]*ActionDefinition)
		}
		action, ok := r.Actions[name]
		if !ok {
			action = &ActionDefinition{
				Parent: r,
				Name:   name,
			}
		}
		if !executeDSL(dsl, action) {
			return
		}
		r.Actions[name] = action
	}
}

// Routing lists the action route. Each route is defined with a function named after the HTTP method.
// The route function takes the path as argument. Route paths may use wildcards as described in the
// [httprouter](https://godoc.org/github.com/julienschmidt/httprouter) package documentation. These
// wildcards define parameters using the `:name` or `*name` syntax where `:name` matches a path
// segment and `*name` is a catch-all that matches the path until the end.
func Routing(routes ...*RouteDefinition) {
	if a, ok := actionDefinition(true); ok {
		for _, r := range routes {
			rwcs := ExtractWildcards(a.Parent.FullPath())
			wcs := ExtractWildcards(r.Path)
			for _, rwc := range rwcs {
				for _, wc := range wcs {
					if rwc == wc {
						ReportError(`duplicate wildcard "%s" in resource base path "%s" and action route "%s"`,
							wc, a.Parent.FullPath(), r.Path)
					}
				}
			}
			r.Parent = a
			a.Routes = append(a.Routes, r)
		}
	}
}

// GET creates a route using the GET HTTP method.
func GET(path string) *RouteDefinition {
	return &RouteDefinition{Verb: "GET", Path: path}
}

// HEAD creates a route using the HEAD HTTP method.
func HEAD(path string) *RouteDefinition {
	return &RouteDefinition{Verb: "HEAD", Path: path}
}

// POST creates a route using the POST HTTP method.
func POST(path string) *RouteDefinition {
	return &RouteDefinition{Verb: "POST", Path: path}
}

// PUT creates a route using the PUT HTTP method.
func PUT(path string) *RouteDefinition {
	return &RouteDefinition{Verb: "PUT", Path: path}
}

// DELETE creates a route using the DELETE HTTP method.
func DELETE(path string) *RouteDefinition {
	return &RouteDefinition{Verb: "DELETE", Path: path}
}

// TRACE creates a route using the TRACE HTTP method.
func TRACE(path string) *RouteDefinition {
	return &RouteDefinition{Verb: "TRACE", Path: path}
}

// CONNECT creates a route using the GET HTTP method.
func CONNECT(path string) *RouteDefinition {
	return &RouteDefinition{Verb: "CONNECT", Path: path}
}

// PATCH creates a route using the PATCH HTTP method.
func PATCH(path string) *RouteDefinition {
	return &RouteDefinition{Verb: "PATCH", Path: path}
}

// Headers implements the DSL for describing HTTP headers. The DSL syntax is identical to the one
// of Attribute. Here is an example defining a couple of headers with validations:
//
//	Headers(func() {
//		Header("Authorization")
//		Header("X-Account", Integer, func() {
//			Minimum(1)
//		})
//		Required("Authorization")
//	})
//
// Headers can be used inside Action to define the action request headers, Response to define the
// response headers or Resource to define common request headers to all the resource actions.
func Headers(dsl func()) {
	if a, ok := actionDefinition(false); ok {
		headers := newAttribute(a.Parent.MediaType)
		if executeDSL(dsl, headers) {
			a.Headers = headers
		}
	} else if r, ok := resourceDefinition(false); ok {
		headers := newAttribute(r.MediaType)
		if executeDSL(dsl, headers) {
			r.Headers = headers
		}
	} else if r, ok := responseDefinition(true); ok {
		if r.Headers != nil {
			ReportError("headers already defined")
			return
		}
		var mtid string
		if pa, ok := r.Parent.(*ResourceDefinition); ok {
			mtid = pa.MediaType
		} else if pa, ok := r.Parent.(*ActionDefinition); ok {
			mtid = pa.Parent.MediaType
		}
		h := newAttribute(mtid)
		if executeDSL(dsl, h) {
			r.Headers = h
		}
	}
}

// Params describe the action parameters, either path parameters identified via wildcards or query
// string parameters. Each parameter is described via the `Param` function which uses the same DSL
// as the Attribute DSL. Here is an example:
//
//	Params(func() {
//		Param("id", Integer)            // A path parameter defined using e.g. GET("/:id")
//		Param("sort", String, func() {  // A query string parameter
//			Enum("asc", "desc")
//		})
//	})
//
// Params can be used inside Action to define the action parameters or Resource to define common
// parameters to all the resource actions.
func Params(dsl func()) {
	if a, ok := actionDefinition(false); ok {
		params := newAttribute(a.Parent.MediaType)
		if executeDSL(dsl, params) {
			a.Params = params
		}
	} else if r, ok := resourceDefinition(true); ok {
		params := newAttribute(r.MediaType)
		if executeDSL(dsl, params) {
			r.Params = params
		}
	}
}

// Payload implements the action payload DSL. An action payload describes the HTTP request body
// data structure. The function accepts either a type or a DSL that describes the payload members
// using the Member DSL which accepts the same syntax as the Attribute DSL. This function can be
// called passing in a type, a DSL or both. Examples:
//
//	 Payload(BottlePayload)	   // Request payload is described by the BottlePayload type
//
//	 Payload(func() {          // Request payload is described inline
//	 	Member("Name")
//	 })
//
//	 Payload(BottlePayload, func() { // Request payload is described by merging the inline
//	 	Required("Name")         // definition into the BottlePayload type.
//	 })
//
func Payload(p interface{}, dsls ...func()) {
	if len(dsls) > 1 {
		ReportError("too many arguments given to Payload")
		return
	}
	if a, ok := actionDefinition(true); ok {
		var att *AttributeDefinition
		var dsl func()
		switch actual := p.(type) {
		case func():
			dsl = actual
			att = newAttribute(a.Parent.MediaType)
			att.Type = Object{}
		case *AttributeDefinition:
			att = actual
		case DataStructure:
			att = actual.Definition()
		case string:
			ut, ok := Design.Types[actual]
			if !ok {
				ReportError("unknown payload type %s", actual)
			}
			att = ut.AttributeDefinition
		}
		if len(dsls) == 1 {
			if dsl != nil {
				ReportError("invalid arguments in Payload call, must be (type), (dsl) or (type, dsl)")
			}
			dsl = dsls[0]
		}
		if dsl != nil {
			executeDSL(dsl, att)
		}
		rn := inflect.Camelize(a.Parent.Name)
		an := inflect.Camelize(a.Name)
		a.Payload = &UserTypeDefinition{
			AttributeDefinition: att,
			TypeName:            fmt.Sprintf("%s%sPayload", an, rn),
		}
	}
}

// newAttribute creates a new attribute definition using the media type with the given identifier
// as base type.
func newAttribute(baseMT string) *AttributeDefinition {
	var base DataType
	if mt, ok := Design.MediaTypes[baseMT]; ok {
		base = mt.Type
	}
	return &AttributeDefinition{Reference: base}
}

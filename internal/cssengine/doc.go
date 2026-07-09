// Package cssengine owns htmlterm's CSS parsing, selector matching, and
// cascade resolution.
//
// It is internal deliberately: the boundary keeps CSS/selector machinery out
// of the renderer and DOM packages without committing htmlterm to a public CSS
// engine API.
package cssengine

package worker

// Context object aims to store runtime configurations

import "errors"

// A Context object is a layered key-value storage
// when enters a context, the changes to the storage would be stored
// in a new layer and when exits, the top layer poped and the storage
// returned to the state before entering this context
type Context struct {
	parent *Context
	store  map[string]interface{}
}

// NewContext returns a new context object
func NewContext() *Context {
	return &Context{
		parent: nil,
		store:  make(map[string]interface{}),
	}
}

// Enter generates a new layer of context
func (ctx *Context) Enter() *Context {

	return &Context{
		parent: ctx,
		store:  make(map[string]interface{}),
	}

}

// Exit return the upper  layer of context
func (ctx *Context) Exit() (*Context, error) {
	if ctx.parent == nil {
		return nil, errors.New("Cannot exit the bottom layer context")
	}
	return ctx.parent, nil
}

// Get returns the value corresponding to key, if it's
// not found in the current layer, return the lower layer
// context's value
func (ctx *Context) Get(key string) (interface{}, bool) {
	if ctx.parent == nil {
		if value, ok := ctx.store[key]; ok {
			return value, true
		}
		return nil, false
	}
	if value, ok := ctx.store[key]; ok {
		return value, true
	}
	return ctx.parent.Get(key)
}

// Set sets the value to the key at current layer
func (ctx *Context) Set(key string, value interface{}) {
	ctx.store[key] = value
}

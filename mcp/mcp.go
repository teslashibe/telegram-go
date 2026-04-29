// Package mcp exposes the telegram-go [telegram.Client] surface as a
// set of MCP (Model Context Protocol) tools that any host application
// can mount on its own MCP server.
//
// Tools are defined via [mcptool.Define] so JSON input schemas are
// reflected from typed input structs — no hand-maintained schemas, no
// drift. The coverage test in mcp_test.go fails if a new exported
// method is added to *telegram.Client without either being wrapped by
// a tool or appearing in [Excluded] (with a reason).
package mcp

import "github.com/teslashibe/mcptool"

// Provider implements [mcptool.Provider] for telegram-go.
type Provider struct{}

// Platform returns "telegram".
func (Provider) Platform() string { return "telegram" }

// Tools returns every telegram-go MCP tool, in registration order.
func (Provider) Tools() []mcptool.Tool {
	out := make([]mcptool.Tool, 0,
		len(statusTools)+len(chatTools)+len(messageTools)+
			len(sendTools)+len(reactTools)+len(contactTools)+
			len(joinTools)+len(watchTools))
	out = append(out, statusTools...)
	out = append(out, chatTools...)
	out = append(out, messageTools...)
	out = append(out, sendTools...)
	out = append(out, reactTools...)
	out = append(out, contactTools...)
	out = append(out, joinTools...)
	out = append(out, watchTools...)
	return out
}

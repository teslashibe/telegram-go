package mcp_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
	tmcp "github.com/teslashibe/telegram-go/mcp"
)

func TestEveryClientMethodIsWrappedOrExcluded(t *testing.T) {
	rep := mcptool.Coverage(
		reflect.TypeOf(&telegram.Client{}),
		tmcp.Provider{}.Tools(),
		tmcp.Excluded,
	)
	if len(rep.Missing) > 0 {
		t.Fatalf("methods missing MCP exposure (add a tool or list in excluded.go): %v", rep.Missing)
	}
	if len(rep.UnknownExclusions) > 0 {
		t.Fatalf("excluded.go references methods that don't exist on *Client (rename?): %v", rep.UnknownExclusions)
	}
	if len(rep.Wrapped)+len(rep.Excluded) == 0 {
		t.Fatal("no wrapped or excluded methods detected — coverage helper is mis-configured")
	}
}

func TestToolsValidate(t *testing.T) {
	if err := mcptool.ValidateTools(tmcp.Provider{}.Tools()); err != nil {
		t.Fatal(err)
	}
}

func TestPlatformName(t *testing.T) {
	if got := (tmcp.Provider{}).Platform(); got != "telegram" {
		t.Errorf("Platform() = %q, want telegram", got)
	}
}

func TestToolsHaveTelegramPrefix(t *testing.T) {
	const prefix = "telegram_"
	for _, tool := range (tmcp.Provider{}).Tools() {
		if !strings.HasPrefix(tool.Name, prefix) {
			t.Errorf("tool %q lacks %s prefix", tool.Name, prefix)
		}
	}
}

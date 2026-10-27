package executor

import (
	"context"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

// TestCodexAutoExecutor_UseUpstreamWebsocket covers the routing decision that upgrades an
// HTTP/SSE downstream to the websocket upstream transport.
func TestCodexAutoExecutor_UseUpstreamWebsocket(t *testing.T) {
	newAuth := func(websockets string) *cliproxyauth.Auth {
		attrs := map[string]string{}
		if websockets != "" {
			attrs["websockets"] = websockets
		}
		return &cliproxyauth.Auth{ID: "codex-auth", Provider: "codex", Attributes: attrs}
	}

	tests := []struct {
		name           string
		force          bool
		websocketsAttr string
		downstreamWS   bool
		want           bool
	}{
		{
			name:           "default config keeps HTTP downstream on HTTP upstream",
			force:          false,
			websocketsAttr: "true",
			downstreamWS:   false,
			want:           false,
		},
		{
			name:           "websocket downstream still upgrades without force",
			force:          false,
			websocketsAttr: "true",
			downstreamWS:   true,
			want:           true,
		},
		{
			name:           "force upgrades HTTP downstream when credential enables websockets",
			force:          true,
			websocketsAttr: "true",
			downstreamWS:   false,
			want:           true,
		},
		{
			name:           "force does not upgrade when credential disables websockets",
			force:          true,
			websocketsAttr: "false",
			downstreamWS:   false,
			want:           false,
		},
		{
			name:           "force does not upgrade when credential has no websockets attribute",
			force:          true,
			websocketsAttr: "",
			downstreamWS:   false,
			want:           false,
		},
		{
			name:           "force with websocket downstream and enabled credential",
			force:          true,
			websocketsAttr: "true",
			downstreamWS:   true,
			want:           true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Codex: config.CodexConfig{ForceUpstreamWebsocket: tc.force}}
			exec := NewCodexAutoExecutor(cfg)

			ctx := context.Background()
			if tc.downstreamWS {
				ctx = cliproxyexecutor.WithDownstreamWebsocket(ctx)
			}

			if got := exec.useUpstreamWebsocket(ctx, newAuth(tc.websocketsAttr)); got != tc.want {
				t.Fatalf("useUpstreamWebsocket() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCodexAutoExecutor_ForceUpstreamWebsocket_RoutesToWebsocketSource asserts the setting
// actually reaches the websocket executor, not just the boolean helper.
func TestCodexAutoExecutor_ForceUpstreamWebsocket_RoutesToWebsocketSource(t *testing.T) {
	cfg := &config.Config{Codex: config.CodexConfig{ForceUpstreamWebsocket: true}}
	exec := NewCodexAutoExecutor(cfg)

	if got := exec.forceUpstreamWebsocket(); !got {
		t.Fatal("forceUpstreamWebsocket() = false, want true")
	}

	auth := &cliproxyauth.Auth{
		ID:         "codex-auth",
		Provider:   "codex",
		Attributes: map[string]string{"websockets": "true"},
	}
	// A plain HTTP downstream context must now be routed to the websocket executor.
	if got := exec.useUpstreamWebsocket(context.Background(), auth); !got {
		t.Fatal("HTTP downstream was not upgraded to the websocket upstream")
	}
}

// TestCodexAutoExecutor_NilSafety keeps the helper safe on a partially built executor.
func TestCodexAutoExecutor_NilSafety(t *testing.T) {
	var exec *CodexAutoExecutor
	if got := exec.forceUpstreamWebsocket(); got {
		t.Fatalf("nil executor forceUpstreamWebsocket() = %v, want false", got)
	}
	if exec.useUpstreamWebsocket(context.Background(), &cliproxyauth.Auth{Attributes: map[string]string{"websockets": "true"}}) {
		t.Fatal("nil executor useUpstreamWebsocket() = true, want false")
	}
}

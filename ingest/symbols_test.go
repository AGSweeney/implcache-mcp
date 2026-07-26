// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"strconv"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func TestExtractSymbolsTableDriven(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
		want map[string]string // name -> kind
	}{
		{
			name: "go func and method",
			path: "client.go",
			body: "package p\n\nfunc OpenStore(path string) error { return nil }\n\nfunc (c *Client) Connect() error { return nil }\n",
			want: map[string]string{"OpenStore": KindFunction, "Connect": KindMethod},
		},
		{
			name: "c definition and call",
			path: "main.c",
			body: "int InitDevice(void) {\n  ConfigurePin(1);\n  return 0;\n}\n",
			want: map[string]string{"InitDevice": KindFunction, "ConfigurePin": KindCall},
		},
		{
			name: "header prototype",
			path: "api.h",
			body: "void ConfigurePin(int mode);\nint SpiTransfer(const uint8_t *buf, size_t n);\n",
			want: map[string]string{"ConfigurePin": KindDeclaration, "SpiTransfer": KindDeclaration},
		},
		{
			name: "cpp scoped method",
			path: "cmd.cpp",
			body: "void demo::RegisterCommand(const char *name) {\n  return;\n}\n",
			want: map[string]string{"demo::RegisterCommand": KindMethod},
		},
		{
			name: "types macros constants",
			path: "types.hpp",
			body: "class RetryPolicy {};\nenum class Status { Ok };\n#define CONFIG_MAX_RETRIES 3\nconst int CONFIG_MAX_RETRIES = 3;\n",
			want: map[string]string{"RetryPolicy": KindType, "Status": KindType, "CONFIG_MAX_RETRIES": KindMacro},
		},
		{
			name: "member and qualified calls",
			path: "use.cpp",
			body: "void f() {\n  Client.Connect();\n  ns::Helper();\n}\n",
			want: map[string]string{"Client.Connect": KindCall, "ns::Helper": KindCall},
		},
		{
			name: "definition beats call",
			path: "both.c",
			body: "void RegisterHandler(void);\nint RegisterHandler(void) {\n  RegisterHandler();\n  return 0;\n}\n",
			want: map[string]string{"RegisterHandler": KindFunction},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			syms := ExtractSymbols(tc.path, tc.body)
			got := map[string]string{}
			for _, s := range syms {
				got[s.Name] = s.Kind
			}
			for name, kind := range tc.want {
				if got[name] != kind {
					// Allow unqualified form for scoped methods.
					if kind == KindMethod {
						ok := false
						for n, k := range got {
							if k == KindMethod && (n == name || strings.HasSuffix(n, "::"+name) || strings.HasSuffix(name, n)) {
								ok = true
								break
							}
						}
						if ok {
							continue
						}
					}
					t.Fatalf("want %s kind=%s; got map=%v syms=%+v", name, kind, got, syms)
				}
			}
		})
	}
}

func TestNoHardCodedDemoRegex(t *testing.T) {
	// Production extraction must not rely on a fixed demo API allow-list.
	body := "void BrandNewAPI(void) {\n  OtherNewHelper(1);\n}\n"
	syms := ExtractSymbols("x.c", body)
	names := map[string]bool{}
	for _, s := range syms {
		names[s.Name] = true
	}
	if !names["BrandNewAPI"] {
		t.Fatalf("expected general extraction of BrandNewAPI, got %+v", syms)
	}
}

func TestUnknownLanguageNoFalseSymbols(t *testing.T) {
	md := "# Setup\n\nCall `RegisterHandler` then `Client.Connect()`.\n\n```\nint foo(void) { return 0; }\n```\n"
	if syms := ExtractSymbols("README.md", md); len(syms) != 0 {
		t.Fatalf("markdown should not extract C-family symbols: %+v", syms)
	}
	yaml := "name: demo\ncommand: RegisterHandler --init\n"
	if syms := ExtractSymbols("config.yaml", yaml); len(syms) != 0 {
		t.Fatalf("yaml should not extract symbols: %+v", syms)
	}
	// Shell/config-like extensions stay empty (no C fallthrough).
	if syms := ExtractSymbols("run.sh", "RegisterHandler()\nClient.Connect()\n"); len(syms) != 0 {
		t.Fatalf("shell should not extract symbols: %+v", syms)
	}
	if syms := ExtractSymbols("cfg.json", `{"cmd":"RegisterHandler()","x":"Client.Connect()"}`); len(syms) != 0 {
		t.Fatalf("json should not extract symbols: %+v", syms)
	}
}

func TestExtractPythonJSJava(t *testing.T) {
	py := ExtractSymbols("client.py", `
class RetryPolicy:
    pass

def connect(host):
    return True

async def disconnect():
    pass

    def _inner(self):
        pass
`)
	got := map[string]string{}
	for _, s := range py {
		got[s.Name] = s.Kind
	}
	for _, name := range []string{"RetryPolicy", "connect", "disconnect"} {
		if got[name] == "" {
			t.Fatalf("python missing %s: %+v", name, py)
		}
	}
	if got["RetryPolicy"] != KindType {
		t.Fatalf("RetryPolicy kind=%s", got["RetryPolicy"])
	}

	js := ExtractSymbols("sdk.ts", `
export class NetworkClient {}
export function connect(host: string) {}
export const writeJSON = (x) => x;
const openSession = async function() {};
`)
	got = map[string]string{}
	for _, s := range js {
		got[s.Name] = s.Kind
	}
	for _, name := range []string{"NetworkClient", "connect", "writeJSON", "openSession"} {
		if got[name] == "" {
			t.Fatalf("js/ts missing %s: %+v", name, js)
		}
	}

	java := ExtractSymbols("Client.java", `
public class NetworkClient {
  public void connect(String host) {
  }
  public Status disconnect();
}
`)
	got = map[string]string{}
	for _, s := range java {
		got[s.Name] = s.Kind
	}
	if got["NetworkClient"] != KindType {
		t.Fatalf("java type=%v", got)
	}
	if got["connect"] != KindMethod {
		t.Fatalf("java connect=%v", got)
	}
}

func TestExtractTemplatesMacrosAliases(t *testing.T) {
	body := `
template<typename T, typename U>
class ResultHolder {};

template<typename T>
T MaxValue(T a, T b) {
  return a;
}

#define CONFIG_ENABLE_TRACE 1
#define LOG_MSG(msg) do { puts(msg); } while (0)

using BufferPtr = std::unique_ptr<uint8_t[]>;
typedef int StatusCode;
`
	syms := ExtractSymbols("util.hpp", body)
	got := map[string]string{}
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	for _, name := range []string{"ResultHolder", "MaxValue", "CONFIG_ENABLE_TRACE", "LOG_MSG", "BufferPtr", "StatusCode"} {
		if got[name] == "" {
			t.Fatalf("missing %s in %+v", name, got)
		}
	}
	if got["ResultHolder"] != KindType || got["MaxValue"] != KindFunction {
		t.Fatalf("kinds=%v", got)
	}
	if got["LOG_MSG"] != KindMacro {
		t.Fatalf("LOG_MSG kind=%s", got["LOG_MSG"])
	}
}

func TestInferAuthorityTightHeuristics(t *testing.T) {
	if got := InferAuthority("my-app", "src/main.go"); got != store.AuthorityUnknown {
		t.Fatalf("bare .go/src path should be unknown, got %q", got)
	}
	if got := InferAuthority("sdk", "examples/gpio/main.c"); got != store.AuthorityOfficialExample {
		t.Fatalf("examples/ segment: got %q", got)
	}
	if got := InferAuthority("help-docs", "api/dita/foo.md"); got != store.AuthorityOfficialDocs {
		t.Fatalf("docs cues: got %q", got)
	}
	if got := InferAuthority("app", "notes-about-example-usage.md"); got != store.AuthorityUnknown {
		t.Fatalf("substring example in filename must not upgrade: got %q", got)
	}
}

func TestExtractionEdgeCases(t *testing.T) {
	t.Run("trailing return type definition", func(t *testing.T) {
		syms := ExtractSymbols("ret.cpp", "auto ComputeValue(int x) -> int {\n  return x;\n}\n")
		got := map[string]bool{}
		for _, s := range syms {
			got[s.Name] = true
		}
		if !got["ComputeValue"] {
			t.Fatalf("missing trailing-return function: %+v", syms)
		}
	})
	t.Run("unclosed comparison preserves later definition", func(t *testing.T) {
		syms := ExtractSymbols("compare.cpp", "bool Less(int a, int b) { return a<b;\n}\nvoid LaterDefinition() {}\n")
		got := map[string]bool{}
		for _, s := range syms {
			got[s.Name] = true
		}
		if !got["LaterDefinition"] {
			t.Fatalf("comparison normalization discarded later symbol: %+v", syms)
		}
	})
	t.Run("csharp access modifiers", func(t *testing.T) {
		syms := ExtractSymbols("NetworkClient.cs", "public class NetworkClient {\n  public async Task ConnectAsync(string host) { }\n}\n")
		got := map[string]string{}
		for _, s := range syms {
			got[s.Name] = s.Kind
		}
		if got["NetworkClient"] != KindType || got["ConnectAsync"] != KindMethod {
			t.Fatalf("csharp symbols=%+v", syms)
		}
	})
	t.Run("javascript control flow is not a method", func(t *testing.T) {
		syms := ExtractSymbols("client.ts", "export class Client {\n  connect() {}\n}\nif (ready) {\n  connect();\n}\n")
		for _, s := range syms {
			if s.Name == "if" {
				t.Fatalf("control flow extracted as symbol: %+v", syms)
			}
		}
	})
}

func TestExtractionPrefersDefinitionsWhenCapped(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 90; i++ {
		b.WriteString("void Defined")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() {}\n")
		b.WriteString("void Use")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() { RandomCall(); }\n")
	}
	syms := ExtractSymbols("many.cpp", b.String())
	if len(syms) != 80 {
		t.Fatalf("symbols=%d want 80", len(syms))
	}
	for _, s := range syms {
		if s.Kind == KindCall {
			t.Fatalf("lower priority call retained under cap: %+v", s)
		}
	}
}

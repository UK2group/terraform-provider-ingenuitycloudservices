package provider

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNewICSClientRejectsInvalidBaseURL(t *testing.T) {
	for _, baseURL := range []string{
		"",
		"api.ingenuitycloudservices.com",
		"ftp://api.ingenuitycloudservices.com",
		"https://user:pass@api.ingenuitycloudservices.com",
		"https://api.ingenuitycloudservices.com?x=1",
		"https://api.ingenuitycloudservices.com#frag",
		"https://api.ingenuitycloudservices.com/?",
		"https://api.ingenuitycloudservices.com/#",
		"http://:8080",
	} {
		if _, err := NewICSClient("token", baseURL); err == nil {
			t.Errorf("NewICSClient(%q) expected error, got nil", baseURL)
		}
	}
}

func TestClientEscapesUserSuppliedValues(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := NewICSClient("token", srv.URL+"/")
	if err != nil {
		t.Fatalf("NewICSClient: %v", err)
	}

	if err := client.CancelServer("../../other?x=1"); err != nil {
		t.Fatalf("CancelServer: %v", err)
	}
	if want := "/rest-api/servers/..%2F..%2Fother%3Fx=1/cancel"; gotPath != want || gotQuery != "" {
		t.Errorf("CancelServer requested path %q query %q, want %q with no query", gotPath, gotQuery, want)
	}

	for _, id := range []string{"", ".", ".."} {
		if err := client.CancelServer(id); err == nil {
			t.Errorf("CancelServer(%q) expected error, got nil", id)
		}
	}

	_, _ = client.GetAddons("sku&location_code=evil", "LON")
	if want := "location_code=LON&sku_product_name=sku%26location_code%3Devil"; gotQuery != want {
		t.Errorf("GetAddons requested query %q, want %q", gotQuery, want)
	}
}

func TestClientRefusesCrossOriginRedirect(t *testing.T) {
	var leakedToken string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leakedToken = r.Header.Get("X-Api-Token")
	}))
	defer other.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest-api/servers/1/cancel" {
			http.Redirect(w, r, "/rest-api/servers/2/cancel", http.StatusTemporaryRedirect)
			return
		}
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()

	client, err := NewICSClient("secret", srv.URL)
	if err != nil {
		t.Fatalf("NewICSClient: %v", err)
	}

	if err := client.CancelServer("1"); err == nil {
		t.Error("CancelServer followed a cross-origin redirect, want error")
	}
	if leakedToken != "" {
		t.Errorf("API token was sent to the redirect target: %q", leakedToken)
	}
}

func TestOriginNormalizesHostAndDefaultPort(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		same bool
	}{
		{"https://Example.com:443/x", "https://example.com/y", true},
		{"http://example.com", "http://example.com:80", true},
		{"https://example.com", "http://example.com", false},
		{"https://example.com", "https://example.com:8443", false},
		{"https://example.com", "https://evil.example", false},
	} {
		a, _ := url.Parse(tc.a)
		b, _ := url.Parse(tc.b)
		if got := origin(a) == origin(b); got != tc.same {
			t.Errorf("origin(%q) == origin(%q) is %v, want %v", tc.a, tc.b, got, tc.same)
		}
	}
}

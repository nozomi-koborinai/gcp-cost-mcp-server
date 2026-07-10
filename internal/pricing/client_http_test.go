package pricing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient returns a Client wired to a test server.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
}

func TestClient_ListServices(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2beta/services" {
			t.Errorf("path = %q, want /v2beta/services", r.URL.Path)
		}
		if got := r.URL.Query().Get("pageSize"); got != "100" {
			t.Errorf("pageSize = %q, want 100", got)
		}
		if got := r.URL.Query().Get("pageToken"); got != "tok1" {
			t.Errorf("pageToken = %q, want tok1", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"services":[{"name":"services/0000-AAAA","serviceId":"0000-AAAA","displayName":"Cloud Run"}],"nextPageToken":"tok2"}`))
	}))

	resp, err := client.ListServices(context.Background(), 100, "tok1")
	if err != nil {
		t.Fatalf("ListServices returned error: %v", err)
	}
	if len(resp.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(resp.Services))
	}
	if resp.Services[0].DisplayName != "Cloud Run" {
		t.Errorf("DisplayName = %q, want Cloud Run", resp.Services[0].DisplayName)
	}
	if resp.NextPageToken != "tok2" {
		t.Errorf("NextPageToken = %q, want tok2", resp.NextPageToken)
	}
}

func TestClient_ListServices_DefaultPageSize(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("pageSize"); got != "5000" {
			t.Errorf("pageSize = %q, want 5000 (DefaultPageSize)", got)
		}
		w.Write([]byte(`{"services":[]}`))
	}))

	if _, err := client.ListServices(context.Background(), 0, ""); err != nil {
		t.Fatalf("ListServices returned error: %v", err)
	}
}

func TestClient_ListServices_HTTPError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))

	if _, err := client.ListServices(context.Background(), 0, ""); err == nil {
		t.Fatal("ListServices should return error on HTTP 500")
	}
}

func TestClient_ListServices_InvalidJSON(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not json`))
	}))

	if _, err := client.ListServices(context.Background(), 0, ""); err == nil {
		t.Fatal("ListServices should return error on invalid JSON")
	}
}

func TestClient_ListSKUs(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2beta/skus" {
			t.Errorf("path = %q, want /v2beta/skus", r.URL.Path)
		}
		if got, want := r.URL.Query().Get("filter"), `service="services/6F81-5844-456A"`; got != want {
			t.Errorf("filter = %q, want %q", got, want)
		}
		w.Write([]byte(`{"skus":[{"skuId":"0008-F633-76AA","displayName":"N1 Predefined Instance Core"}]}`))
	}))

	resp, err := client.ListSKUs(context.Background(), "6F81-5844-456A", 0, "")
	if err != nil {
		t.Fatalf("ListSKUs returned error: %v", err)
	}
	if len(resp.SKUs) != 1 || resp.SKUs[0].SKUID != "0008-F633-76AA" {
		t.Errorf("unexpected SKUs: %+v", resp.SKUs)
	}
}

func TestClient_ListSKUs_EmptyServiceID(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when serviceID is empty")
	}))

	if _, err := client.ListSKUs(context.Background(), "", 0, ""); err == nil {
		t.Fatal("ListSKUs should return error when serviceID is empty")
	}
}

func TestClient_GetSKUPrice(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2beta/skus/0008-F633-76AA/price" {
			t.Errorf("path = %q, want /v2beta/skus/0008-F633-76AA/price", r.URL.Path)
		}
		if got := r.URL.Query().Get("currencyCode"); got != "JPY" {
			t.Errorf("currencyCode = %q, want JPY", got)
		}
		w.Write([]byte(`{"name":"skus/0008-F633-76AA/price","currencyCode":"JPY","skuPrices":[{"valueType":"rate","rate":{"tiers":[{"startAmount":{"value":"0"},"listPrice":{"currencyCode":"JPY","units":"5","nanos":0}}],"unitInfo":{"unit":"h"}}}]}`))
	}))

	resp, err := client.GetSKUPrice(context.Background(), "0008-F633-76AA", "JPY")
	if err != nil {
		t.Fatalf("GetSKUPrice returned error: %v", err)
	}
	if resp.CurrencyCode != "JPY" {
		t.Errorf("CurrencyCode = %q, want JPY", resp.CurrencyCode)
	}
	if len(resp.SKUPrices) != 1 || resp.SKUPrices[0].Rate == nil {
		t.Fatalf("unexpected SKUPrices: %+v", resp.SKUPrices)
	}
}

func TestClient_GetSKUPrice_EmptySKUID(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when skuID is empty")
	}))

	if _, err := client.GetSKUPrice(context.Background(), "", "USD"); err == nil {
		t.Fatal("GetSKUPrice should return error when skuID is empty")
	}
}

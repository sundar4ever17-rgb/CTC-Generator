package app

import (
	"ctc/internal/engine"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONBodyRejectsTrailingMalformedContent(t *testing.T) {
	for _, body := range []string{"{\"name\":\"ok\"} garbage", "{\"name\":\"ok\"} {}"} {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		var value map[string]any
		if readJSON(rr, req, &value) || rr.Code != 400 {
			t.Fatal("invalid trailing JSON accepted")
		}
	}
}
func TestJSONBodyRejectsCrossSiteFormContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	var value map[string]any
	if readJSON(rr, req, &value) || rr.Code != 415 {
		t.Fatal("non-JSON body accepted")
	}
}
func TestDocumentEscapesUntrustedCandidateAndTemplateText(t *testing.T) {
	c := engine.DemoConfiguration()
	v := Revision{Input: engine.Input{CandidateName: "<script>alert(1)</script>"}, Result: engine.Result{}}
	tpl := c.Data.OfferTemplate
	tpl.Body = "<img src=x onerror=alert(1)>"
	html, err := renderDocument(v, c, tpl, "offer", "draft")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script>") || strings.Contains(html, "<img src=x") {
		t.Fatal("unescaped document content")
	}
	if !strings.Contains(html, "NOT AN EMPLOYMENT OFFER") {
		t.Fatal("draft warning missing")
	}
}

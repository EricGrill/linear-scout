package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const chatResp = `{"choices":[{"message":{"role":"assistant","content":
"{\"recommendations\":[{\"summary\":\"Fix login crash\",\"why_it_matters\":\"blocks users\",\"confidence\":0.8,\"evidence\":[{\"ref\":\"ENG-1\",\"url\":\"https://l/ENG-1\"}]}]}"}}]}`

func TestOpenAIProviderParsesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(chatResp))
	}))
	defer srv.Close()

	p := NewOpenAI("sk_test", "gpt-4o-mini", WithBaseURL(srv.URL))
	recs, err := p.Recommend(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Summary != "Fix login crash" {
		t.Fatalf("bad parse: %+v", recs)
	}
	if len(recs[0].Evidence) != 1 || recs[0].Evidence[0].Ref != "ENG-1" {
		t.Fatalf("evidence not parsed: %+v", recs[0])
	}
}

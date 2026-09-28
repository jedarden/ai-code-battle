package conformance

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestReferenceBotRejectsIncorrectBotIdentifier covers the identity check
// that sits outside the HMAC. X-ACB-Bot-Id is deliberately not signed by the
// protocol, so changing it leaves the otherwise-valid signature unchanged;
// an implementation that has the registered ID available must still reject
// the request before executing the turn.
func TestReferenceBotRejectsIncorrectBotIdentifier(t *testing.T) {
	const expectedBotID = "b_conformance"
	secret := DefaultConformanceSecret
	body := buildBody(func(map[string]interface{}) {})
	fresh := canonicalTimestamp(time.Now())

	tests := []struct {
		name       string
		botID      string
		wantStatus int
		signed     bool
	}{
		{name: "registered identifier", botID: expectedBotID, wantStatus: http.StatusOK, signed: true},
		{name: "different registered identifier", botID: "b_other", wantStatus: http.StatusUnauthorized},
		{name: "malformed identifier", botID: "not-a-bot", wantStatus: http.StatusUnauthorized},
	}

	server := httptest.NewServer(ReferenceBotHandlerForBotID(secret, expectedBotID))
	t.Cleanup(server.Close)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := authHeaders(secret, ConformanceMatchID, "7", fresh, body, nil)
			// The signature remains valid after this change: bot ID is not in
			// the signed payload. The handler must enforce the provisioned ID
			// as a separate identity check.
			headers["X-ACB-Bot-Id"] = tt.botID
			result := RunCase(t.Context(), SuiteClient(5*time.Second), server.URL, secret, Case{
				Name:        tt.name,
				ContentType: "application/json",
				Headers:     headers,
				Body:        body,
				WantStatus:  tt.wantStatus,
				Signed:      tt.signed,
			})
			if !result.Pass {
				t.Fatalf("%s: %s", result.Case, result.Detail)
			}
		})
	}
}

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apache261/Byakugan-core/engine"
)

func TestJSONTransferCheckAndAuthentication(t *testing.T) {
	handler := New(engine.New(nil), Options{APIKeys: []string{"secret"}})
	body := `{"message_type":"pacs.008","payload":{"message_id":"m","creation_date_time":"2026-09-21T08:00:00Z","payment_identification":{"end_to_end_id":"e"},"instructed_amount":{"amount":1,"currency":"PHP"},"debtor_account":{"id":"d"},"debtor_agent":{"bicfi":"DEUTPHMM"},"creditor_account":{"id":"c"},"creditor_agent":{"bicfi":"BOPIPHMM"}}}`

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/v1/transfer-checks", strings.NewReader(body)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", unauthorized.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/transfer-checks", strings.NewReader(body))
	request.Header.Set("X-API-Key", "secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"decision":"allow"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestJSONTransferCheckRejectsNonJSONNumber(t *testing.T) {
	handler := New(engine.New(nil), Options{})
	body := `{"message_type":"pacs.008","payload":{"instructed_amount":{"amount":NaN,"currency":"PHP"}}}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/transfer-checks", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestJSONTransferCheckRejectsTrailingValue(t *testing.T) {
	handler := New(engine.New(nil), Options{})
	body := `{"message_type":"pacs.008","payload":{}} {}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/transfer-checks", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

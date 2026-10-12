package iso20022

import (
	"strings"
	"testing"
)

func TestParsePacs008(t *testing.T) {
	xml := `<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"><FIToFICstmrCdtTrf><GrpHdr><MsgId>msg-1</MsgId><CreDtTm>2026-09-21T08:00:00Z</CreDtTm></GrpHdr><CdtTrfTxInf><PmtId><EndToEndId>e2e-1</EndToEndId></PmtId><IntrBkSttlmAmt Ccy="PHP">1250.50</IntrBkSttlmAmt><DbtrAcct><Id><Othr><Id>debtor</Id></Othr></Id></DbtrAcct><DbtrAgt><FinInstnId><BICFI>DEUTPHMM</BICFI></FinInstnId></DbtrAgt><CdtrAcct><Id><Othr><Id>creditor</Id></Othr></Id></CdtrAcct><CdtrAgt><FinInstnId><BICFI>BOPIPHMM</BICFI></FinInstnId></CdtrAgt></CdtTrfTxInf></FIToFICstmrCdtTrf></Document>`
	request, err := ParsePacs008(strings.NewReader(xml), "idem-xml")
	if err != nil {
		t.Fatal(err)
	}
	if request.Payload.InstructedAmount.Amount != "1250.5" || request.Payload.CreditorAccount.ID != "creditor" || request.Payload.MessageDefinitionID != "pacs.008.001.08" {
		t.Fatalf("unexpected request: %+v", request)
	}
}

func TestParsePacs008RejectsNonFiniteAmounts(t *testing.T) {
	for _, amount := range []string{"NaN", "Inf", "-Inf"} {
		xml := `<Document><FIToFICstmrCdtTrf><GrpHdr><MsgId>msg</MsgId><CreDtTm>2026-09-21T08:00:00Z</CreDtTm></GrpHdr><CdtTrfTxInf><PmtId><EndToEndId>e2e</EndToEndId></PmtId><IntrBkSttlmAmt Ccy="PHP">` + amount + `</IntrBkSttlmAmt></CdtTrfTxInf></FIToFICstmrCdtTrf></Document>`
		if _, err := ParsePacs008(strings.NewReader(xml), ""); err == nil {
			t.Fatalf("amount %q accepted", amount)
		}
	}
}

func TestParsePacs008PreservesExactLargeAmountAndCurrencyScale(t *testing.T) {
	for _, tc := range []struct {
		amount, currency, want string
		invalid                bool
	}{
		{"999999999999999999.99", "USD", "999999999999999999.99", false},
		{"0.001", "KWD", "0.001", false}, {"1.1", "JPY", "", true}, {"0", "PHP", "", true},
	} {
		xml := `<Document><FIToFICstmrCdtTrf><GrpHdr><MsgId>msg</MsgId><CreDtTm>2026-09-21T08:00:00Z</CreDtTm></GrpHdr><CdtTrfTxInf><PmtId><EndToEndId>e2e</EndToEndId></PmtId><IntrBkSttlmAmt Ccy="` + tc.currency + `">` + tc.amount + `</IntrBkSttlmAmt></CdtTrfTxInf></FIToFICstmrCdtTrf></Document>`
		req, err := ParsePacs008(strings.NewReader(xml), "")
		if tc.invalid {
			if err == nil {
				t.Fatalf("invalid amount accepted: %+v", tc)
			}
			continue
		}
		if err != nil || req.Payload.InstructedAmount.Amount != tc.want {
			t.Fatalf("precision changed: %+v %v", req, err)
		}
	}
}

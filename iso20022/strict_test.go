package iso20022

import (
	"strings"
	"testing"
)

const singleTransfer = `<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"><FIToFICstmrCdtTrf><GrpHdr><MsgId>synthetic</MsgId><NbOfTxs>1</NbOfTxs><CreDtTm>2026-10-14T00:00:00Z</CreDtTm></GrpHdr><CdtTrfTxInf><PmtId><EndToEndId>synthetic-e2e</EndToEndId></PmtId><IntrBkSttlmAmt Ccy="USD">0.10</IntrBkSttlmAmt><DbtrAcct><Id><Othr><Id>debtor</Id></Othr></Id></DbtrAcct><CdtrAcct><Id><Othr><Id>creditor</Id></Othr></Id></CdtrAcct><DbtrAgt><FinInstnId><BICFI>DEUTUSMM</BICFI></FinInstnId></DbtrAgt><CdtrAgt><FinInstnId><BICFI>CITIUSMM</BICFI></FinInstnId></CdtrAgt></CdtTrfTxInf></FIToFICstmrCdtTrf></Document>`

func TestStrictPacs008(t *testing.T) {
	if _, err := ParsePacs008(strings.NewReader(singleTransfer), ""); err != nil {
		t.Fatal(err)
	}
	postal := strings.Replace(singleTransfer, "<DbtrAcct>", "<Dbtr><PstlAdr><AdrLine>line one</AdrLine><AdrLine>line two</AdrLine></PstlAdr></Dbtr><DbtrAcct>", 1)
	if _, err := ParsePacs008(strings.NewReader(postal), ""); err != nil {
		t.Fatalf("valid ignored postal lines rejected: %v", err)
	}
	a := strings.Index(singleTransfer, "<CdtTrfTxInf>")
	b := strings.Index(singleTransfer, "</CdtTrfTxInf>") + len("</CdtTrfTxInf>")
	tx := singleTransfer[a:b]
	cases := map[string]string{
		"multiple transfers":    strings.Replace(singleTransfer, tx, tx+tx, 1),
		"count mismatch":        strings.Replace(singleTransfer, "<NbOfTxs>1", "<NbOfTxs>2", 1),
		"unsupported version":   strings.ReplaceAll(singleTransfer, "pacs.008.001.08", "pacs.008.001.09"),
		"header mismatch":       strings.Replace(singleTransfer, "<FIToFICstmrCdtTrf>", "<AppHdr><MsgDefIdr>pacs.008.001.09</MsgDefIdr></AppHdr><FIToFICstmrCdtTrf>", 1),
		"wrong child namespace": strings.Replace(singleTransfer, "<CdtTrfTxInf>", `<CdtTrfTxInf xmlns="urn:other">`, 1),
		"duplicate amount":      strings.Replace(singleTransfer, "</IntrBkSttlmAmt>", `</IntrBkSttlmAmt><IntrBkSttlmAmt Ccy="USD">2</IntrBkSttlmAmt>`, 1),
		"duplicate message":     strings.Replace(singleTransfer, "</Document>", "<FIToFICstmrCdtTrf/></Document>", 1),
		"trailing document":     singleTransfer + "<Document/>", "truncated": strings.TrimSuffix(singleTransfer, "</Document>"),
		"oversized trailing whitespace": singleTransfer + strings.Repeat(" ", MaxPacs008Bytes), "excessive nesting": strings.Replace(singleTransfer, "<GrpHdr>", "<GrpHdr>"+strings.Repeat("<x>", 65)+strings.Repeat("</x>", 65), 1),
		"directive": "<!DOCTYPE Document>" + singleTransfer,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePacs008(strings.NewReader(input), ""); err == nil {
				t.Fatal("ambiguous XML accepted")
			}
		})
	}
}

func FuzzParsePacs008(f *testing.F) {
	f.Add([]byte(singleTransfer))
	f.Add([]byte("<Document>"))
	f.Add([]byte(singleTransfer + "<Document/>"))
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > MaxPacs008Bytes+1 {
			return
		}
		_, _ = ParsePacs008(strings.NewReader(string(input)), "synthetic-fuzz")
	})
}

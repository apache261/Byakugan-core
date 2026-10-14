package iso20022

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/apache261/Byakugan-core/domain"
)

type applicationHeader struct {
	From          headerParty `xml:"Fr"`
	To            headerParty `xml:"To"`
	BusinessMsgID string      `xml:"BizMsgIdr"`
	MessageDefID  string      `xml:"MsgDefIdr"`
	CreationDate  string      `xml:"CreDt"`
}
type headerParty struct {
	FinancialInstitution headerFIID `xml:"FIId"`
}
type headerFIID struct {
	FinancialInstitution financialInstitution `xml:"FinInstnId"`
}
type transferMessage struct {
	GroupHeader groupHeader `xml:"GrpHdr"`
	Transfers   []transfer  `xml:"CdtTrfTxInf"`
}
type groupHeader struct {
	NumberOfTransactions string `xml:"NbOfTxs"`
	MessageID            string `xml:"MsgId"`
	CreationDateTime     string `xml:"CreDtTm"`
	SettlementDate       string `xml:"IntrBkSttlmDt"`
}
type transfer struct {
	PaymentID        paymentID  `xml:"PmtId"`
	InterbankAmount  amount     `xml:"IntrBkSttlmAmt"`
	InstructedAmount amount     `xml:"InstdAmt"`
	InstructingAgent agent      `xml:"InstgAgt"`
	InstructedAgent  agent      `xml:"InstdAgt"`
	Debtor           party      `xml:"Dbtr"`
	DebtorAccount    account    `xml:"DbtrAcct"`
	DebtorAgent      agent      `xml:"DbtrAgt"`
	Creditor         party      `xml:"Cdtr"`
	CreditorAccount  account    `xml:"CdtrAcct"`
	CreditorAgent    agent      `xml:"CdtrAgt"`
	Remittance       remittance `xml:"RmtInf"`
}
type paymentID struct {
	InstructionID     string `xml:"InstrId"`
	EndToEndID        string `xml:"EndToEndId"`
	TransactionID     string `xml:"TxId"`
	ClearingReference string `xml:"ClrSysRef"`
}
type amount struct {
	Currency string `xml:"Ccy,attr"`
	Value    string `xml:",chardata"`
}
type party struct {
	Name string `xml:"Nm"`
}
type agent struct {
	FinancialInstitution financialInstitution `xml:"FinInstnId"`
}
type financialInstitution struct {
	BICFI string `xml:"BICFI"`
	Name  string `xml:"Nm"`
}
type account struct {
	ID accountID `xml:"Id"`
}
type accountID struct {
	IBAN  string  `xml:"IBAN"`
	Other otherID `xml:"Othr"`
}
type otherID struct {
	ID string `xml:"Id"`
}
type remittance struct {
	Unstructured []string `xml:"Ustrd"`
}

// ParsePacs008 accepts one complete pacs.008.001.08 Document with one transfer.
// It rejects oversized, ambiguous, unsupported, and trailing XML input.
func ParsePacs008(r io.Reader, idempotencyKey string) (domain.TransferCheckRequest, error) {
	message, header, namespace, err := decode(r)
	if err != nil {
		return domain.TransferCheckRequest{}, err
	}
	if len(message.Transfers) != 1 {
		return domain.TransferCheckRequest{}, fmt.Errorf("pacs.008 requires exactly one CdtTrfTxInf")
	}
	if message.GroupHeader.NumberOfTransactions != "" && message.GroupHeader.NumberOfTransactions != "1" {
		return domain.TransferCheckRequest{}, fmt.Errorf("pacs.008 group transaction count must be one")
	}
	if header.MessageDefID != "" && header.MessageDefID != "pacs.008.001.08" {
		return domain.TransferCheckRequest{}, fmt.Errorf("unsupported pacs.008 message definition")
	}
	tx := message.Transfers[0]
	transferAmount := tx.InterbankAmount
	if strings.TrimSpace(transferAmount.Value) == "" {
		transferAmount = tx.InstructedAmount
	}
	value, err := domain.CanonicalAmount(strings.TrimSpace(transferAmount.Value), strings.ToUpper(strings.TrimSpace(transferAmount.Currency)))
	if err != nil {
		return domain.TransferCheckRequest{}, fmt.Errorf("invalid pacs.008 amount")
	}
	createdAt, err := parseTime(first(message.GroupHeader.CreationDateTime, header.CreationDate))
	if err != nil {
		return domain.TransferCheckRequest{}, fmt.Errorf("invalid pacs.008 CreDtTm")
	}
	request := domain.TransferCheckRequest{IdempotencyKey: strings.TrimSpace(idempotencyKey), MessageType: "pacs.008", Payload: domain.Pacs008Transfer{
		MessageDefinitionID: messageDefinition(header.MessageDefID, namespace), SchemaNamespace: namespace,
		MessageID: first(message.GroupHeader.MessageID, header.BusinessMsgID), CreationDateTime: createdAt,
		PaymentIdentification: domain.PaymentIdentification{InstructionID: strings.TrimSpace(tx.PaymentID.InstructionID), EndToEndID: strings.TrimSpace(tx.PaymentID.EndToEndID), TransactionID: strings.TrimSpace(tx.PaymentID.TransactionID), ClearingSystemReference: strings.TrimSpace(tx.PaymentID.ClearingReference)},
		InstructedAmount:      domain.Money{Amount: value, Currency: strings.ToUpper(strings.TrimSpace(transferAmount.Currency))},
		Debtor:                domain.Party{Name: strings.TrimSpace(tx.Debtor.Name)}, DebtorAccount: accountRef(tx.DebtorAccount), DebtorAgent: agentRef(firstAgent(tx.DebtorAgent, tx.InstructingAgent, agentFrom(header.From.FinancialInstitution.FinancialInstitution))),
		Creditor: domain.Party{Name: strings.TrimSpace(tx.Creditor.Name)}, CreditorAccount: accountRef(tx.CreditorAccount), CreditorAgent: agentRef(firstAgent(tx.CreditorAgent, tx.InstructedAgent, agentFrom(header.To.FinancialInstitution.FinancialInstitution))),
		RemittanceInformation: strings.TrimSpace(strings.Join(tx.Remittance.Unstructured, " ")), InterbankSettlementDate: strings.TrimSpace(message.GroupHeader.SettlementDate),
	}}
	if errs := domain.ValidateTransfer(request); len(errs) > 0 {
		return domain.TransferCheckRequest{}, fmt.Errorf("%s", domain.ValidationReason(errs))
	}
	return request, nil
}

const MaxPacs008Bytes = 2 << 20
const pacs008Namespace = "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"

func decode(r io.Reader) (transferMessage, applicationHeader, string, error) {
	var message transferMessage
	var header applicationHeader
	raw, err := io.ReadAll(io.LimitReader(r, MaxPacs008Bytes+1))
	if err != nil || len(raw) > MaxPacs008Bytes {
		return message, header, "", fmt.Errorf("pacs.008 input exceeds limit or cannot be read")
	}
	// Validate the complete token stream before mapping; encoding/xml otherwise
	// ignores duplicate scalar fields and fields in unexpected namespaces.
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	paths := []string{}
	seen := map[string]bool{}
	mapped := singleValuePaths()
	roots := 0
	elements := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return message, header, "", fmt.Errorf("invalid pacs.008 xml: %w", err)
		}
		switch token := token.(type) {
		case xml.Directive:
			return message, header, "", fmt.Errorf("XML directives are unsupported")
		case xml.CharData:
			if len(paths) == 0 && strings.TrimSpace(string(token)) != "" {
				return message, header, "", fmt.Errorf("unexpected XML content")
			}
		case xml.StartElement:
			elements++
			if elements > 10000 || len(paths) >= 64 || len(token.Name.Local) > 128 {
				return message, header, "", fmt.Errorf("pacs.008 structure exceeds parser limits")
			}
			if len(paths) == 0 {
				roots++
				if roots != 1 || token.Name.Local != "Document" || token.Name.Space != pacs008Namespace {
					return message, header, "", fmt.Errorf("unsupported pacs.008 document or namespace")
				}
			}
			paths = append(paths, token.Name.Local)
			path := strings.Join(paths, "/")
			if strings.HasPrefix(path, "Document/FIToFICstmrCdtTrf") && token.Name.Space != pacs008Namespace {
				return message, header, "", fmt.Errorf("inconsistent pacs.008 namespace")
			}
			if mapped[path] && seen[path] {
				return message, header, "", fmt.Errorf("duplicate pacs.008 element %s", path)
			}
			seen[path] = true
		case xml.EndElement:
			paths = paths[:len(paths)-1]
		}
	}
	if !seen["Document/FIToFICstmrCdtTrf/GrpHdr"] {
		return message, header, "", fmt.Errorf("pacs.008 group header is required")
	}
	var doc struct {
		Header  applicationHeader `xml:"AppHdr"`
		Message transferMessage   `xml:"FIToFICstmrCdtTrf"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return message, header, "", err
	}
	return doc.Message, doc.Header, pacs008Namespace, nil
}

func parseTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02T15:04:05", strings.TrimSpace(value))
}
func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
func messageDefinition(header, namespace string) string {
	if strings.TrimSpace(header) != "" {
		return strings.TrimSpace(header)
	}
	return strings.TrimPrefix(namespace, "urn:iso:std:iso:20022:tech:xsd:")
}
func accountRef(value account) domain.AccountRef {
	if iban := strings.TrimSpace(value.ID.IBAN); iban != "" {
		return domain.AccountRef{ID: iban, IBAN: iban}
	}
	other := strings.TrimSpace(value.ID.Other.ID)
	return domain.AccountRef{ID: other, BBAN: other}
}
func agentFrom(value financialInstitution) agent { return agent{FinancialInstitution: value} }
func firstAgent(values ...agent) agent {
	for _, value := range values {
		if strings.TrimSpace(value.FinancialInstitution.BICFI) != "" || strings.TrimSpace(value.FinancialInstitution.Name) != "" {
			return value
		}
	}
	return agent{}
}
func agentRef(value agent) domain.Agent {
	return domain.Agent{BICFI: strings.TrimSpace(value.FinancialInstitution.BICFI), Name: strings.TrimSpace(value.FinancialInstitution.Name)}
}

// Only mapped values have singleton semantics here. Ignored ISO structures
// may legitimately repeat (for example postal address lines).
func singleValuePaths() map[string]bool {
	paths := map[string]bool{"Document": true, "Document/AppHdr": true}
	base := "Document/FIToFICstmrCdtTrf"
	for _, suffix := range []string{"", "/GrpHdr", "/GrpHdr/MsgId", "/GrpHdr/CreDtTm", "/GrpHdr/NbOfTxs", "/GrpHdr/IntrBkSttlmDt", "/CdtTrfTxInf", "/CdtTrfTxInf/PmtId", "/CdtTrfTxInf/PmtId/InstrId", "/CdtTrfTxInf/PmtId/EndToEndId", "/CdtTrfTxInf/PmtId/TxId", "/CdtTrfTxInf/PmtId/ClrSysRef", "/CdtTrfTxInf/IntrBkSttlmAmt", "/CdtTrfTxInf/InstdAmt", "/CdtTrfTxInf/RmtInf"} {
		paths[base+suffix] = true
	}
	for _, party := range []string{"Dbtr", "Cdtr"} {
		paths[base+"/CdtTrfTxInf/"+party] = true
		paths[base+"/CdtTrfTxInf/"+party+"/Nm"] = true
	}
	for _, account := range []string{"DbtrAcct", "CdtrAcct"} {
		for _, suffix := range []string{"", "/Id", "/Id/IBAN", "/Id/Othr", "/Id/Othr/Id"} {
			paths[base+"/CdtTrfTxInf/"+account+suffix] = true
		}
	}
	for _, agent := range []string{"DbtrAgt", "CdtrAgt", "InstgAgt", "InstdAgt"} {
		for _, suffix := range []string{"", "/FinInstnId", "/FinInstnId/BICFI", "/FinInstnId/Nm"} {
			paths[base+"/CdtTrfTxInf/"+agent+suffix] = true
		}
	}
	for _, suffix := range []string{"/BizMsgIdr", "/MsgDefIdr", "/CreDt"} {
		paths["Document/AppHdr"+suffix] = true
	}
	for _, party := range []string{"Fr", "To"} {
		for _, suffix := range []string{"", "/FIId", "/FIId/FinInstnId", "/FIId/FinInstnId/BICFI", "/FIId/FinInstnId/Nm"} {
			paths["Document/AppHdr/"+party+suffix] = true
		}
	}
	return paths
}

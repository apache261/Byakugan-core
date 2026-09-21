package iso20022

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
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
	MessageID        string `xml:"MsgId"`
	CreationDateTime string `xml:"CreDtTm"`
	SettlementDate   string `xml:"IntrBkSttlmDt"`
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

// ParsePacs008 maps the first credit-transfer record to the public canonical request.
func ParsePacs008(r io.Reader, idempotencyKey string) (domain.TransferCheckRequest, error) {
	message, header, namespace, err := decode(r)
	if err != nil {
		return domain.TransferCheckRequest{}, err
	}
	if len(message.Transfers) == 0 {
		return domain.TransferCheckRequest{}, fmt.Errorf("pacs.008 xml missing CdtTrfTxInf")
	}
	tx := message.Transfers[0]
	transferAmount := tx.InterbankAmount
	if strings.TrimSpace(transferAmount.Value) == "" {
		transferAmount = tx.InstructedAmount
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(transferAmount.Value), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return domain.TransferCheckRequest{}, fmt.Errorf("invalid pacs.008 amount")
	}
	createdAt, err := parseTime(first(message.GroupHeader.CreationDateTime, header.CreationDate))
	if err != nil {
		return domain.TransferCheckRequest{}, fmt.Errorf("invalid pacs.008 CreDtTm")
	}
	return domain.TransferCheckRequest{IdempotencyKey: strings.TrimSpace(idempotencyKey), MessageType: "pacs.008", Payload: domain.Pacs008Transfer{
		MessageDefinitionID: messageDefinition(header.MessageDefID, namespace), SchemaNamespace: namespace,
		MessageID: first(message.GroupHeader.MessageID, header.BusinessMsgID), CreationDateTime: createdAt,
		PaymentIdentification: domain.PaymentIdentification{InstructionID: strings.TrimSpace(tx.PaymentID.InstructionID), EndToEndID: strings.TrimSpace(tx.PaymentID.EndToEndID), TransactionID: strings.TrimSpace(tx.PaymentID.TransactionID), ClearingSystemReference: strings.TrimSpace(tx.PaymentID.ClearingReference)},
		InstructedAmount:      domain.Money{Amount: value, Currency: strings.ToUpper(strings.TrimSpace(transferAmount.Currency))},
		Debtor:                domain.Party{Name: strings.TrimSpace(tx.Debtor.Name)}, DebtorAccount: accountRef(tx.DebtorAccount), DebtorAgent: agentRef(firstAgent(tx.DebtorAgent, tx.InstructingAgent, agentFrom(header.From.FinancialInstitution.FinancialInstitution))),
		Creditor: domain.Party{Name: strings.TrimSpace(tx.Creditor.Name)}, CreditorAccount: accountRef(tx.CreditorAccount), CreditorAgent: agentRef(firstAgent(tx.CreditorAgent, tx.InstructedAgent, agentFrom(header.To.FinancialInstitution.FinancialInstitution))),
		RemittanceInformation: strings.TrimSpace(strings.Join(tx.Remittance.Unstructured, " ")), InterbankSettlementDate: strings.TrimSpace(message.GroupHeader.SettlementDate),
	}}, nil
}

func decode(r io.Reader) (transferMessage, applicationHeader, string, error) {
	var message transferMessage
	var header applicationHeader
	decoder := xml.NewDecoder(io.LimitReader(r, 2<<20))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return message, header, "", fmt.Errorf("invalid pacs.008 xml: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "AppHdr":
			if err := decoder.DecodeElement(&header, &start); err != nil {
				return message, header, "", fmt.Errorf("invalid pacs.008 AppHdr: %w", err)
			}
		case "FIToFICstmrCdtTrf":
			if err := decoder.DecodeElement(&message, &start); err != nil {
				return message, header, "", fmt.Errorf("invalid pacs.008 transfer: %w", err)
			}
			return message, header, start.Name.Space, nil
		}
	}
	return message, header, "", fmt.Errorf("pacs.008 xml missing FIToFICstmrCdtTrf")
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

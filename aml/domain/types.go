// Package domain defines portable AML evaluation contracts, without ledger or workflow services.
package domain

import "time"

type SubjectType string

const (
	SubjectCustomer        SubjectType = "customer"
	SubjectAccount         SubjectType = "account"
	SubjectBeneficialOwner SubjectType = "beneficial_owner"
	SubjectRelatedParty    SubjectType = "related_party"
	SubjectCounterparty    SubjectType = "counterparty"
	SubjectInstitution     SubjectType = "institution"
)

type AssetType string

const (
	AssetFiat      AssetType = "fiat"
	AssetVirtual   AssetType = "virtual_asset"
	AssetSecurity  AssetType = "security"
	AssetCommodity AssetType = "commodity"
	AssetOther     AssetType = "other"
)

type TransactionKind string

const (
	TransactionPayment      TransactionKind = "payment"
	TransactionCash         TransactionKind = "cash"
	TransactionCard         TransactionKind = "card"
	TransactionTrade        TransactionKind = "trade"
	TransactionSecurity     TransactionKind = "security"
	TransactionVirtualAsset TransactionKind = "virtual_asset"
	TransactionAdjustment   TransactionKind = "adjustment"
	TransactionReversal     TransactionKind = "reversal"
	TransactionOther        TransactionKind = "other"
)

type TransactionDirection string

const (
	DirectionInbound  TransactionDirection = "inbound"
	DirectionOutbound TransactionDirection = "outbound"
	DirectionInternal TransactionDirection = "internal"
	DirectionUnknown  TransactionDirection = "unknown"
)

type TransactionStatus string

const (
	TransactionPending   TransactionStatus = "pending"
	TransactionPosted    TransactionStatus = "posted"
	TransactionSettled   TransactionStatus = "settled"
	TransactionRejected  TransactionStatus = "rejected"
	TransactionCancelled TransactionStatus = "cancelled"
	TransactionReversed  TransactionStatus = "reversed"
)

type TransactionLinkType string

const (
	LinkCorrection TransactionLinkType = "correction_of"
	LinkReversal   TransactionLinkType = "reversal_of"
	LinkAdjustment TransactionLinkType = "adjustment_of"
)

type SourceIdentity struct {
	SourceSystem  string    `json:"source_system"`
	ExternalID    string    `json:"external_id"`
	SourceEventAt time.Time `json:"source_event_at"`
}

type SubjectReference struct {
	Type SubjectType `json:"type"`
	ID   string      `json:"id"`
}

type Money struct {
	Amount    Decimal   `json:"amount"`
	AssetCode string    `json:"asset_code"`
	AssetType AssetType `json:"asset_type,omitempty"`
}

type TransactionLink struct {
	Type          TransactionLinkType `json:"type"`
	TransactionID string              `json:"transaction_id"`
}

type Transaction struct {
	ID                 string               `json:"id"`
	Source             SourceIdentity       `json:"source"`
	EventAt            time.Time            `json:"event_at"`
	BookedAt           time.Time            `json:"booked_at,omitempty"`
	ValueAt            time.Time            `json:"value_at,omitempty"`
	ReceivedAt         time.Time            `json:"received_at,omitempty"`
	Kind               TransactionKind      `json:"kind"`
	Subtype            string               `json:"subtype,omitempty"`
	Direction          TransactionDirection `json:"direction"`
	Status             TransactionStatus    `json:"status"`
	Money              Money                `json:"money"`
	Subjects           []SubjectReference   `json:"subjects"`
	RelatedTransaction *TransactionLink     `json:"related_transaction,omitempty"`
	Channel            string               `json:"channel,omitempty"`
	Purpose            string               `json:"purpose,omitempty"`
	OriginCountry      string               `json:"origin_country,omitempty"`
	DestinationCountry string               `json:"destination_country,omitempty"`
	Metadata           map[string]any       `json:"metadata,omitempty"`
}

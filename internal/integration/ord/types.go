package ord

import (
	"encoding/json"
	"strings"
)

// Organization — тело POST /organization.
type Organization struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	IsOrs bool   `json:"isOrs"`
	IsRr  bool   `json:"isRr"`
	INN   string `json:"inn,omitempty"`
	Name  string `json:"name,omitempty"`
	KPP   string `json:"kpp,omitempty"`
}

// Contract — тело POST /contract.
type Contract struct {
	ID                      string `json:"id"`
	Type                    string `json:"type"`
	ContractorID            string `json:"contractorId"`
	ClientID                string `json:"clientId"`
	IsRegReport             bool   `json:"isRegReport"`
	Date                    string `json:"date"`
	Number                  string `json:"number,omitempty"`
	SubjectType             string `json:"subjectType,omitempty"`
	ActionType              string `json:"actionType,omitempty"`
	AgentActingForPublisher *bool  `json:"agentActingForPublisher,omitempty"`
}

// Platforms — тело POST /platforms.
type Platforms struct {
	OrganizationID string     `json:"organizationId"`
	Platforms      []Platform `json:"platforms"`
}

// Platform — площадка.
type Platform struct {
	PlatformID string `json:"platformId"`
	IsOwned    bool   `json:"isOwned"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	URL        string `json:"url,omitempty"`
}

// Creative — тело POST /creative.
type Creative struct {
	ID            string      `json:"id"`
	CreativeType  string      `json:"creativeType"`
	IsSocial      bool        `json:"isSocial"`
	IsSocialQuota bool        `json:"isSocialQuota"`
	CoBranding    bool        `json:"coBranding"`
	ContractIDs   []string    `json:"contractIds"`
	KKTUCodes     []string    `json:"kktuCodes"`
	Form          string      `json:"form"`
	TextData      []string    `json:"textData,omitempty"`
	MediaData     []MediaData `json:"mediaData,omitempty"`
	URLs          []string    `json:"urls,omitempty"`
}

// MediaData — файл креатива по публичной ссылке.
type MediaData struct {
	MediaURL         string `json:"mediaUrl"`
	MediaURLFileType string `json:"mediaUrlFileType"`
}

// Receipt — ответ ResponseStatus на POST и GET /status.
type Receipt struct {
	RequestID      string            `json:"request_id"`
	Status         string            `json:"status"`
	Token          string            `json:"token"`
	ObjectStatuses []ObjectStatus    `json:"object_statuses"`
	ErrorMessage   []json.RawMessage `json:"error_message"`
}

// ObjectStatus — статус объекта запроса.
type ObjectStatus struct {
	ObjectID     string            `json:"object_id"`
	Status       string            `json:"status"`
	Token        string            `json:"token"`
	ErrorMessage []json.RawMessage `json:"error_message"`
}

// Errors — тексты error_message[]: строки как есть, прочие значения — JSON.
func (r Receipt) Errors() []string { return messages(r.ErrorMessage) }

// Errors — тексты error_message[] объекта.
func (o ObjectStatus) Errors() []string { return messages(o.ErrorMessage) }

func messages(raw []json.RawMessage) []string {
	out := make([]string, 0, len(raw))
	for _, m := range raw {
		var s string
		if json.Unmarshal(m, &s) == nil {
			out = append(out, s)
			continue
		}
		out = append(out, strings.TrimSpace(string(m)))
	}
	return out
}

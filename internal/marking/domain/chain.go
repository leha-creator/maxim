package domain

import "github.com/google/uuid"

// Requirement — объект, без которого нельзя отправить зависимый (05 §3.2, R-4).
type Requirement string

const (
	NeedPlatformOrganization   Requirement = "platform_organization"
	NeedAdvertiserOrganization Requirement = "advertiser_organization"
	NeedBloggerOrganization    Requirement = "blogger_organization"
	NeedInitialContract        Requirement = "initial_contract"
	// Отчётность (R-20, R-22): креатив с erid, площадка, договор акта.
	NeedCreative Requirement = "creative"
	NeedPlatform Requirement = "platform"
	NeedContract Requirement = "contract"
)

// CreativeDependencies — цепочка креатива: контрагент платформы → контрагент рекламодателя →
// изначальный договор (R-4).
func CreativeDependencies() []Requirement {
	return []Requirement{NeedPlatformOrganization, NeedAdvertiserOrganization, NeedInitialContract}
}

// ContractDependencies — стороны договора: изначальный — платформа и рекламодатель, с блогером —
// платформа и блогер (R-4).
func ContractDependencies(kind ContractKind) []Requirement {
	if kind == ContractPublisher {
		return []Requirement{NeedPlatformOrganization, NeedBloggerOrganization}
	}
	return []Requirement{NeedPlatformOrganization, NeedAdvertiserOrganization}
}

// PlatformDependencies — площадка зависит только от контрагента блогера (R-4).
func PlatformDependencies() []Requirement { return []Requirement{NeedBloggerOrganization} }

// Dependency — состояние зависимости на момент проверки.
type Dependency struct {
	Need Requirement
	// Exists — запись есть; иначе её нужно создать.
	Exists bool
	Ref    ObjectRef
	State  SyncState
}

// Verdict — решение по цепочке зависимостей (R-4).
type Verdict struct {
	// Missing — записей нет: создать pending и поставить их задания.
	Missing []Requirement
	// Waiting — первая неготовая зависимость (в работе ОРД или ждёт отправки).
	Waiting *Dependency
	// Failed — первая зависимость в error: зависимый объект не отправляется.
	Failed *Dependency
}

// Ready — все зависимости готовы: объект можно отправлять.
func (v Verdict) Ready() bool { return len(v.Missing) == 0 && v.Waiting == nil && v.Failed == nil }

// DependencyVerdict проверяет цепочку по порядку.
func DependencyVerdict(deps []Dependency) Verdict {
	var v Verdict
	for i := range deps {
		d := deps[i]
		switch {
		case !d.Exists:
			v.Missing = append(v.Missing, d.Need)
		case d.State.Status == SyncError:
			if v.Failed == nil {
				v.Failed = &d
			}
		case !d.State.Ready():
			if v.Waiting == nil {
				v.Waiting = &d
			}
		}
	}
	return v
}

// ContractParties — стороны договора вида kind: клиент и исполнитель (A-31).
func ContractParties(kind ContractKind, platformOrg, otherOrg uuid.UUID) (client, contractor uuid.UUID) {
	if kind == ContractPublisher {
		return platformOrg, otherOrg
	}
	return otherOrg, platformOrg
}

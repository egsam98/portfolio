package portfolio

import (
	"github.com/pkg/errors"
)

type TriggerType uint8

const (
	CRL TriggerType = iota + 1
	CCBP
)

var (
	triggerTypeKeyValues = map[TriggerType]string{
		CRL:  "COST_REACHED_LIMIT",
		CCBP: "COST_CHANGED_BY_PERCENT",
	}
	triggerTypeValueKeys = map[string]TriggerType{
		"COST_REACHED_LIMIT":      CRL,
		"COST_CHANGED_BY_PERCENT": CCBP,
	}
)

func (t TriggerType) String() string {
	return triggerTypeKeyValues[t]
}

func (t TriggerType) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *TriggerType) UnmarshalText(text []byte) error {
	txt := string(text)
	if triggerType, ok := triggerTypeValueKeys[txt]; ok {
		*t = triggerType
		return nil
	}
	return errors.Errorf("invalid trigger type: %s", txt)
}

package notify

import "time"

// Action is the decision output of Decide.
type Action int

const (
	ActionNone Action = iota
	ActionSendAlert
	ActionSendRecovery
)

// Decide implements the notification decision matrix. It is a pure function:
// given the current time, previous state, current warn count and signature,
// and the cooldown window, return what to do.
func Decide(now time.Time, prev *State, warnCount int, sigNow string, cooldown time.Duration) Action {
	if prev == nil {
		prev = &State{}
	}
	hasAlerts := warnCount > 0
	wasAlert := prev.LastStatus == StatusAlert

	switch {
	case !hasAlerts && !wasAlert:
		return ActionNone

	case !hasAlerts && wasAlert:
		return ActionSendRecovery

	case hasAlerts && !wasAlert:
		return ActionSendAlert

	case hasAlerts && wasAlert:
		if sigNow != prev.LastSignature {
			return ActionSendAlert
		}
		if now.Sub(prev.LastSentAt) >= cooldown {
			return ActionSendAlert
		}
		return ActionNone
	}
	return ActionNone
}

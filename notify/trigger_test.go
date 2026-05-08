package notify

import (
	"testing"
	"time"
)

func TestDecide_ColdStartOK(t *testing.T) {
	if got := Decide(time.Now(), nil, 0, "", 2*time.Hour); got != ActionNone {
		t.Fatalf("cold + ok → none, got %v", got)
	}
}

func TestDecide_ColdStartAlert(t *testing.T) {
	if got := Decide(time.Now(), nil, 3, "sig", 2*time.Hour); got != ActionSendAlert {
		t.Fatalf("cold + warn → alert, got %v", got)
	}
}

func TestDecide_OKAfterAlertSendsRecovery(t *testing.T) {
	prev := &State{LastStatus: StatusAlert, LastSignature: "sig", LastSentAt: time.Now()}
	if got := Decide(time.Now(), prev, 0, "", 2*time.Hour); got != ActionSendRecovery {
		t.Fatalf("alert→ok must send recovery, got %v", got)
	}
}

func TestDecide_PersistentOK(t *testing.T) {
	prev := &State{LastStatus: StatusOK}
	if got := Decide(time.Now(), prev, 0, "", 2*time.Hour); got != ActionNone {
		t.Fatalf("ok→ok → none, got %v", got)
	}
}

func TestDecide_SuppressWithinCooldown(t *testing.T) {
	now := time.Now()
	prev := &State{LastStatus: StatusAlert, LastSignature: "sig", LastSentAt: now.Add(-30 * time.Minute)}
	if got := Decide(now, prev, 5, "sig", 2*time.Hour); got != ActionNone {
		t.Fatalf("same sig within cooldown → none, got %v", got)
	}
}

func TestDecide_SignatureChangeOverridesCooldown(t *testing.T) {
	now := time.Now()
	prev := &State{LastStatus: StatusAlert, LastSignature: "old", LastSentAt: now.Add(-30 * time.Minute)}
	if got := Decide(now, prev, 5, "new", 2*time.Hour); got != ActionSendAlert {
		t.Fatalf("sig change must send even within cooldown, got %v", got)
	}
}

func TestDecide_ResendAfterCooldown(t *testing.T) {
	now := time.Now()
	prev := &State{LastStatus: StatusAlert, LastSignature: "sig", LastSentAt: now.Add(-3 * time.Hour)}
	if got := Decide(now, prev, 5, "sig", 2*time.Hour); got != ActionSendAlert {
		t.Fatalf("after cooldown must resend, got %v", got)
	}
}

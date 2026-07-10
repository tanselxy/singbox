package panel

import (
	"strings"
	"testing"
	"time"

	"github.com/tanselxy/singbox/internal/model"
)

func newNotificationTestServer() *Server {
	return &Server{
		notifyLast:    map[string]time.Time{},
		notifyOffline: map[int64]offlineNotificationState{},
	}
}

func TestOfflineNotificationSendsOnceUntilRecovery(t *testing.T) {
	s := newNotificationTestServer()
	now := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)

	reason, due := s.markOfflineNotification(1, 0, now)
	if !due || reason != "节点离线" {
		t.Fatalf("first offline = (%q, %v), want immediate offline notification", reason, due)
	}
	if reason, due = s.markOfflineNotification(1, 0, now.Add(4*time.Hour)); due {
		t.Fatalf("continuous offline = (%q, %v), want no repeat without repeat setting", reason, due)
	}
	if !s.markRecoveredNotification(1) {
		t.Fatal("recovery after offline should notify")
	}
	if s.markRecoveredNotification(1) {
		t.Fatal("second recovery should not notify")
	}
}

func TestOfflineNotificationOptionalRepeat(t *testing.T) {
	s := newNotificationTestServer()
	now := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	repeat := 2 * time.Hour

	if _, due := s.markOfflineNotification(2, repeat, now); !due {
		t.Fatal("first offline should notify")
	}
	if reason, due := s.markOfflineNotification(2, repeat, now.Add(90*time.Minute)); due {
		t.Fatalf("offline before repeat interval = (%q, %v), want no repeat", reason, due)
	}
	reason, due := s.markOfflineNotification(2, repeat, now.Add(2*time.Hour))
	if !due {
		t.Fatal("offline at repeat interval should notify")
	}
	if !strings.Contains(reason, "节点持续离线") || !strings.Contains(reason, "2 小时") {
		t.Fatalf("repeat reason = %q, want sustained offline duration", reason)
	}
	if reason, due = s.markOfflineNotification(2, repeat, now.Add(3*time.Hour)); due {
		t.Fatalf("offline before next repeat interval = (%q, %v), want no repeat", reason, due)
	}
}

func TestSanitizeNotificationConfigOfflineRepeatHours(t *testing.T) {
	cfg := sanitizeNotificationConfig(notificationConfig{OfflineRepeatHours: -1})
	if cfg.OfflineRepeatHours != 0 {
		t.Fatalf("negative repeat hours = %d, want 0", cfg.OfflineRepeatHours)
	}
	cfg = sanitizeNotificationConfig(notificationConfig{OfflineRepeatHours: 1000})
	if cfg.OfflineRepeatHours != 720 {
		t.Fatalf("large repeat hours = %d, want 720", cfg.OfflineRepeatHours)
	}
}

func TestRenewNodeBillingAdvancesCycleAndLeavesReminderWindow(t *testing.T) {
	end := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC).Unix()
	node := model.Node{EndAt: end, BillingCycle: "monthly"}
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	if !nodeRenewalReminderDue(node, now) {
		t.Fatal("node should be inside 7-day renewal reminder window")
	}
	startAt, endAt, err := renewNodeBilling(node, now)
	if err != nil {
		t.Fatal(err)
	}
	renewed := node
	renewed.StartAt = startAt
	renewed.EndAt = endAt
	renewed.NextRemindAt = 0

	if startAt != end {
		t.Fatalf("renew start = %d, want previous end %d", startAt, end)
	}
	if got := nodeDateInput(endAt); got != "2026-08-15" {
		t.Fatalf("renew end date = %s, want 2026-08-15", got)
	}
	if nodeRenewalReminderDue(renewed, now) {
		t.Fatal("renewed node should leave current reminder window")
	}
}

func TestNodeRenewalReminderDailyGate(t *testing.T) {
	end := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC).Unix()
	now := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	node := model.Node{EndAt: end, BillingCycle: "yearly"}
	if !nodeRenewalReminderDue(node, now) {
		t.Fatal("first reminder should be due")
	}
	node.NextRemindAt = nextNodeRenewalReminderAt(now)
	if nodeRenewalReminderDue(node, now.Add(time.Hour)) {
		t.Fatal("reminder before next reminder time should be suppressed")
	}
	if !nodeRenewalReminderDue(node, time.Unix(node.NextRemindAt, 0).Add(time.Minute)) {
		t.Fatal("next-day reminder should be due")
	}
}

package telegram

import (
	"github.com/AppsGanin/rospanel/internal/i18n"
	"testing"
	"time"

	"github.com/AppsGanin/rospanel/internal/model"
)

func TestHumanLeft(t *testing.T) {
	cases := map[int64]string{
		30 * 86400: "осталось 30 дн.",
		2 * 3600:   "осталось 2 ч.",
		45 * 60:    "осталось 45 мин.",
	}
	for sec, want := range cases {
		if got := humanLeft(sec, i18n.RU); got != want {
			t.Errorf("humanLeft(%d, i18n.RU) = %q, want %q", sec, got, want)
		}
	}
}

func TestUserOnlineLine(t *testing.T) {
	now := time.Now().Unix()
	loc := time.UTC
	if got := userOnlineLine(model.User{LastSeen: 0}, now, loc, i18n.RU); got != "🕐 Ещё не подключались" {
		t.Errorf("never-seen: %q", got)
	}
	if got := userOnlineLine(model.User{LastSeen: now - 30}, now, loc, i18n.RU); got != "🟢 Сейчас в сети" {
		t.Errorf("online: %q", got)
	}
	if got := userOnlineLine(model.User{LastSeen: now - 20*60}, now, loc, i18n.RU); got != "🕐 Был в сети 20 мин назад" {
		t.Errorf("mins ago: %q", got)
	}
}

func TestUserStatusLine(t *testing.T) {
	if got := userStatusLine(model.StatusActive, i18n.RU); got != "🟢 <b>Активна</b>" {
		t.Errorf("active: %q", got)
	}
	if got := userStatusLine(model.StatusExpired, i18n.RU); got != "🔴 <b>Срок истёк</b>" {
		t.Errorf("expired: %q", got)
	}
}

// The download button sits directly above "Refresh", so a user looking for one finds
// the other: their position in the keyboard is the whole feature.
func TestUserMenuRowsAppButtonBeforeRefresh(t *testing.T) {
	rows := userMenuRows(&model.Settings{}, model.User{SubToken: "t"}, i18n.RU)
	var prev InlineButton
	for _, row := range rows {
		for _, b := range row {
			if b.CallbackData == "vu:menu" {
				if prev.CallbackData != "vu:apps" {
					t.Fatalf("button before refresh = %q, want the app download one", prev.CallbackData)
				}
				return
			}
			prev = b
		}
	}
	t.Fatal("no refresh button in the user menu")
}

func TestAppRowsPlatforms(t *testing.T) {
	rows := appRows(i18n.RU)
	if len(rows) != 2 || len(rows[0]) != 2 {
		t.Fatalf("picker shape = %d rows (first %d buttons), want 2 rows of 2/1", len(rows), len(rows[0]))
	}
	if got := rows[0][0].CallbackData; got != "vu:app:android" {
		t.Errorf("android callback = %q, want vu:app:android", got)
	}
	if got := rows[0][1].URL; got != iosAppURL {
		t.Errorf("ios url = %q, want %q", got, iosAppURL)
	}
	if got := rows[1][0].CallbackData; got != "vu:menu" {
		t.Errorf("back callback = %q, want vu:menu", got)
	}
}

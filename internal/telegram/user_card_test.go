package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AppsGanin/rospanel/internal/i18n"
	"github.com/AppsGanin/rospanel/internal/model"
	"github.com/AppsGanin/rospanel/internal/store"
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

type recordedAPICall struct {
	method  string
	payload map[string]any
}

type recordingAPI struct {
	mu           sync.Mutex
	calls        []recordedAPICall
	chatResponse string
	server       *httptest.Server
}

func newRecordingAPI(t *testing.T) *recordingAPI {
	t.Helper()
	api := &recordingAPI{}
	api.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		method := parts[len(parts)-1]
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		api.mu.Lock()
		api.calls = append(api.calls, recordedAPICall{method: method, payload: payload})
		api.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if method == "getChat" {
			result := `{"id":-1004211681825,"type":"channel"}`
			if api.chatResponse != "" {
				result = api.chatResponse
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":` + result + `}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(api.server.Close)
	return api
}

func (a *recordingAPI) client() *Client {
	return newTestClient(a.server.URL+"/bot", "test-token")
}

func (a *recordingAPI) snapshot() []recordedAPICall {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]recordedAPICall(nil), a.calls...)
}

func appDownloadService(t *testing.T) *UserService {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "user.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewUser(nil, st)
}

func TestPinnedAPKForwarding(t *testing.T) {
	s := &UserService{}
	api := newRecordingAPI(t)
	api.chatResponse = `{"id":-1004211681825,"type":"channel","pinned_message":{"message_id":42,"chat":{"id":-1004211681825,"type":"channel"},"document":{"file_id":"apk-42","file_name":"app.apk","mime_type":"application/vnd.android.package-archive"}}}`

	s.forwardPinnedAPK(context.Background(), api.client(), 123, i18n.RU)

	calls := api.snapshot()
	if len(calls) != 2 || calls[0].method != "getChat" || calls[1].method != "forwardMessage" {
		t.Fatalf("API calls = %v, want getChat then forwardMessage", calls)
	}
	if got := calls[0].payload["chat_id"]; got != float64(appDownloadChannelID) {
		t.Errorf("getChat chat_id = %v, want %d", got, appDownloadChannelID)
	}
	if got := calls[1].payload["chat_id"]; got != float64(123) {
		t.Errorf("chat_id = %v, want 123", got)
	}
	if got := calls[1].payload["from_chat_id"]; got != float64(appDownloadChannelID) {
		t.Errorf("from_chat_id = %v, want %d", got, appDownloadChannelID)
	}
	if got := calls[1].payload["message_id"]; got != float64(42) {
		t.Errorf("message_id = %v, want 42", got)
	}
}

func TestPinnedAPKRejectsNonAPK(t *testing.T) {
	s := &UserService{}
	api := newRecordingAPI(t)
	api.chatResponse = `{"id":-1004211681825,"type":"channel","pinned_message":{"message_id":42,"chat":{"id":-1004211681825,"type":"channel"},"document":{"file_id":"doc-42","file_name":"notes.txt","mime_type":"text/plain"}}}`

	s.forwardPinnedAPK(context.Background(), api.client(), 123, i18n.RU)

	calls := api.snapshot()
	if len(calls) != 2 || calls[0].method != "getChat" || calls[1].method != "sendMessage" {
		t.Fatalf("API calls = %v, want getChat then fallback sendMessage", calls)
	}
}

func TestAppDownloadMenuButton(t *testing.T) {
	rows := userMenuRows(&model.Settings{}, model.User{}, i18n.RU)
	found := false
	for _, row := range rows {
		for _, button := range row {
			if button.Text == i18n.T(i18n.RU, "user.btnDownloadApp") && button.CallbackData == userCallbackDownloadApp {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("user menu has no download-app callback button")
	}
}

func TestAppDownloadSubmenu(t *testing.T) {
	rows := downloadAppRows(i18n.RU)
	if len(rows) != 3 {
		t.Fatalf("downloadAppRows() has %d rows, want 3", len(rows))
	}
	if got := rows[0][0]; got.Text != i18n.T(i18n.RU, "user.btnDownloadAndroid") || got.CallbackData != userCallbackDownloadAndroid {
		t.Errorf("Android button = %#v", got)
	}
	if got := rows[1][0]; got.Text != i18n.T(i18n.RU, "user.btnDownloadIOS") || got.URL != proofKitAppStoreURL {
		t.Errorf("iOS button = %#v", got)
	}
	if got := rows[2][0]; got.CallbackData != "vu:menu" {
		t.Errorf("back button = %#v", got)
	}
}

func TestAppDownloadCallback(t *testing.T) {
	s := appDownloadService(t)
	api := newRecordingAPI(t)

	s.handleUserCallback(context.Background(), api.client(), &CallbackQuery{
		Message: &Message{Chat: Chat{ID: 123}, MessageID: 7},
		Data:    userCallbackDownloadApp,
	}, &model.Settings{}, model.User{})

	calls := api.snapshot()
	if len(calls) != 1 || calls[0].method != "editMessageText" {
		t.Fatalf("API calls = %v, want one editMessageText", calls)
	}
	if got := calls[0].payload["text"]; got != i18n.T(i18n.RU, "user.downloadAppTitle") {
		t.Errorf("text = %v, want download-app title", got)
	}
	markup, ok := calls[0].payload["reply_markup"].(map[string]any)
	if !ok {
		t.Fatalf("reply_markup = %T, want object", calls[0].payload["reply_markup"])
	}
	rows, ok := markup["inline_keyboard"].([]any)
	if !ok || len(rows) != 3 {
		t.Fatalf("inline_keyboard = %#v, want 3 rows", markup["inline_keyboard"])
	}
}

func TestAndroidDownloadCallback(t *testing.T) {
	s := appDownloadService(t)
	api := newRecordingAPI(t)
	api.chatResponse = `{"id":-1004211681825,"type":"channel","pinned_message":{"message_id":42,"chat":{"id":-1004211681825,"type":"channel"},"document":{"file_id":"apk-42","file_name":"app.apk","mime_type":"application/vnd.android.package-archive"}}}`

	s.handleUserCallback(context.Background(), api.client(), &CallbackQuery{
		Message: &Message{Chat: Chat{ID: 123}, MessageID: 7},
		Data:    userCallbackDownloadAndroid,
	}, &model.Settings{}, model.User{})

	calls := api.snapshot()
	if len(calls) != 2 || calls[0].method != "getChat" || calls[1].method != "forwardMessage" {
		t.Fatalf("API calls = %v, want getChat then forwardMessage", calls)
	}
}

func TestChatPinnedMessageUnmarshal(t *testing.T) {
	var chat Chat
	err := json.Unmarshal([]byte(`{
		"id": -1004211681825,
		"type": "channel",
		"pinned_message": {
			"message_id": 42,
			"chat": {"id": -1004211681825, "type": "channel"},
			"document": {"file_id": "doc-42", "file_name": "app.apk", "mime_type": "application/vnd.android.package-archive"}
		}
	}`), &chat)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if chat.PinnedMessage == nil || chat.PinnedMessage.MessageID != 42 || chat.PinnedMessage.MediaFileID() != "doc-42" {
		t.Fatalf("pinned_message was not decoded: %#v", chat.PinnedMessage)
	}
}

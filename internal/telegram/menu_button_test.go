package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/AppsGanin/rospanel/internal/store"
)

func TestMenuButtonRestoresCommandMenu(t *testing.T) {
	t.Parallel()
	st, err := store.Open(filepath.Join(t.TempDir(), "menu.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.ExecForTest(`UPDATE settings SET host = 'vpn.example.com', miniapp_path = 'mini', tg_lang = 'ru' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	url := "https://vpn.example.com/sub/old-mini"
	var mu sync.Mutex
	menuType := "web_app"
	sets := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/getChatMenuButton"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"type":"` + menuType + `","web_app":{"url":"` + url + `"}}}`))
		case strings.HasSuffix(r.URL.Path, "/setChatMenuButton"):
			var p struct {
				MenuButton struct {
					Type string `json:"type"`
				} `json:"menu_button"`
			}
			_ = json.Unmarshal(body, &p)
			menuType = p.MenuButton.Type
			sets++
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		}
	}))
	defer srv.Close()
	s := &UserService{store: st}
	c := newTestClient(srv.URL+"/bot", "111:AAA")
	if !s.publishMenuButton(context.Background(), c) {
		t.Fatal("publish failed")
	}
	if sets != 1 || menuType != "commands" {
		t.Fatalf("command menu not restored: %d sets, type %q", sets, menuType)
	}
	if !s.publishMenuButton(context.Background(), c) || sets != 1 {
		t.Errorf("command menu was set again (%d sets)", sets)
	}
}

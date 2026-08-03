package core

import "testing"

func TestGetTrayMenuLabels(t *testing.T) {
	tests := []struct {
		name    string
		lang    string
		running bool
		status  string
		toggle  string
		show    string
		quit    string
	}{
		{
			name:    "korean running",
			lang:    "ko",
			running: true,
			status:  "서버 상태: 작동 중",
			toggle:  "서버 중지",
			show:    "메인 창 열기",
			quit:    "종료",
		},
		{
			name:    "korean stopped",
			lang:    "ko",
			running: false,
			status:  "서버 상태: 중지됨",
			toggle:  "서버 시작",
			show:    "메인 창 열기",
			quit:    "종료",
		},
		{
			name:    "english running",
			lang:    "en",
			running: true,
			status:  "Server status: Running",
			toggle:  "Stop Server",
			show:    "Open Main Window",
			quit:    "Quit",
		},
		{
			name:    "english stopped",
			lang:    "en",
			running: false,
			status:  "Server status: Stopped",
			toggle:  "Start Server",
			show:    "Open Main Window",
			quit:    "Quit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := getTrayMenuLabels(tt.lang, tt.running)
			if labels.status != tt.status {
				t.Fatalf("status = %q, want %q", labels.status, tt.status)
			}
			if labels.toggle != tt.toggle {
				t.Fatalf("toggle = %q, want %q", labels.toggle, tt.toggle)
			}
			if labels.show != tt.show {
				t.Fatalf("show = %q, want %q", labels.show, tt.show)
			}
			if labels.quit != tt.quit {
				t.Fatalf("quit = %q, want %q", labels.quit, tt.quit)
			}
		})
	}
}

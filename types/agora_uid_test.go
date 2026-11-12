package avflow

import (
	"encoding/json"
	"testing"
)

func TestAgoraUID_JSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		json    string
		want    AgoraUID
		wantOut string
	}{
		{
			name:    "omit via null",
			json:    `null`,
			want:    AgoraUID{},
			wantOut: `null`,
		},
		{
			name:    "numeric",
			json:    `12345`,
			want:    NewAgoraUIDNum(12345),
			wantOut: `12345`,
		},
		{
			name:    "zero unset",
			json:    `0`,
			want:    AgoraUID{},
			wantOut: `null`,
		},
		{
			name:    "account digits",
			json:    `"67890"`,
			want:    NewAgoraUIDAccount("67890"),
			wantOut: `"67890"`,
		},
		{
			name:    "account",
			json:    `"alice"`,
			want:    NewAgoraUIDAccount("alice"),
			wantOut: `"alice"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got AgoraUID
			if err := json.Unmarshal([]byte(tc.json), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.kind != tc.want.kind || got.num != tc.want.num || got.str != tc.want.str {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			out, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(out) != tc.wantOut {
				t.Fatalf("marshal = %s, want %s", out, tc.wantOut)
			}
		})
	}
}

func TestAgoraUID_inConfig(t *testing.T) {
	t.Parallel()
	raw := `{"appId":"app","channel":"ch","uid":"bob"}`
	var cfg AgoraSourceConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.UID.IsString() || cfg.UID.Account() != "bob" {
		t.Fatalf("uid = %+v", cfg.UID)
	}
}

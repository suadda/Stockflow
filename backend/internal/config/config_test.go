package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name  string
		addr  string
		dbURL string
		want  Config
	}{
		{
			name: "defaults when unset",
			want: Config{Addr: defaultAddr, DatabaseURL: defaultDatabaseURL},
		},
		{
			name:  "env overrides",
			addr:  ":9090",
			dbURL: "postgres://u:p@db:5432/x?sslmode=disable",
			want:  Config{Addr: ":9090", DatabaseURL: "postgres://u:p@db:5432/x?sslmode=disable"},
		},
		{
			name: "partial override keeps other default",
			addr: "127.0.0.1:8081",
			want: Config{Addr: "127.0.0.1:8081", DatabaseURL: defaultDatabaseURL},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			t.Setenv("ADDR", tc.addr)
			t.Setenv("DATABASE_URL", tc.dbURL)
			if got := Load(); got != tc.want {
				t.Fatalf("Load() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

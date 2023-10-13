package bwhatsapp

import "testing"

func Test_parseUserAndConfFromConfPath(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name     string
		args     args
		wantUser string
		wantConf string
	}{
		{
			name: "valid path",
			args: args{
				path: "/home/ibraham/lab/api-ref/dendrite-endpoint/mtbridge-base-path/cod11/wh07/wh07.toml.qrcode.png",
			},
			wantUser: "cod11",
			wantConf: "wh07",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := parseUserAndConfFromConfPath(tt.args.path)
			if got != tt.wantUser {
				t.Errorf("parseUserAndConfFromConfPath() got = %v, want %v", got, tt.wantUser)
			}
			if got1 != tt.wantConf {
				t.Errorf("parseUserAndConfFromConfPath() got1 = %v, want %v", got1, tt.wantConf)
			}
		})
	}
}

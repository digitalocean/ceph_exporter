package version

import (
	"reflect"
	"testing"
)

func TestParseCephVersion(t *testing.T) {
	type args struct {
		cephVersion string
	}
	tests := []struct {
		name    string
		args    args
		want    *Version
		wantErr bool
	}{
		{
			name:    "invalid version 1",
			args:    args{cephVersion: "totally real version"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid version 2",
			args:    args{cephVersion: "ceph version 14.2.18-97"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nautilus",
			args:    args{cephVersion: "ceph version 14.2.18-97-gcc1e126 (cc1e1267bc7afc8288c718fc3e59c5a6735f6f4a) nautilus (stable)"},
			want:    &Version{Major: 14, Minor: 2, Patch: 18, Revision: 97, Commit: "gcc1e126"},
			wantErr: false,
		},
		{
			name:    "octopus",
			args:    args{cephVersion: "ceph version 15.2.0-1-gcc1e126"},
			want:    &Version{Major: 15, Minor: 2, Patch: 0, Revision: 1, Commit: "gcc1e126"},
			wantErr: false,
		},
		{
			name:    "pacific",
			args:    args{cephVersion: "ceph version 16.2.3-33-gcc1e126"},
			want:    &Version{Major: 16, Minor: 2, Patch: 3, Revision: 33, Commit: "gcc1e126"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCephVersion(tt.args.cephVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCephVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseCephVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersion_IsAtLeast(t *testing.T) {
	type fields struct {
		Major    int
		Minor    int
		Patch    int
		Revision int
		Commit   string
	}
	type args struct {
		constraint *Version
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "equal versions",
			fields: fields{Major: Nautilus.Major, Minor: Nautilus.Minor, Patch: Nautilus.Patch, Revision: Nautilus.Revision, Commit: Nautilus.Commit},
			args:   args{constraint: Nautilus},
			want:   true,
		},
		{
			name:   "slightly older",
			fields: fields{Major: Pacific.Major, Minor: Pacific.Minor - 1, Patch: Pacific.Patch, Revision: Pacific.Revision, Commit: Pacific.Commit},
			args:   args{constraint: Pacific},
			want:   false,
		},
		{
			name:   "significantly newer",
			fields: fields{Major: Pacific.Major, Minor: Pacific.Minor - 1, Patch: Pacific.Patch, Revision: Pacific.Revision, Commit: Pacific.Commit},
			args:   args{constraint: Octopus},
			want:   true,
		},
		{
			name:   "older revision",
			fields: fields{Major: 16, Minor: 2, Patch: 0, Revision: 1},
			args:   args{constraint: &Version{Major: 16, Minor: 2, Patch: 0, Revision: 2}},
			want:   false,
		},
		{
			name:   "newer revision",
			fields: fields{Major: 16, Minor: 2, Patch: 0, Revision: 1},
			args:   args{constraint: Pacific},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := &Version{
				Major:    tt.fields.Major,
				Minor:    tt.fields.Minor,
				Patch:    tt.fields.Patch,
				Revision: tt.fields.Revision,
				Commit:   tt.fields.Commit,
			}
			if got := version.IsAtLeast(tt.args.constraint); got != tt.want {
				t.Errorf("IsAtLeast() = %v, want %v", got, tt.want)
			}
		})
	}
}

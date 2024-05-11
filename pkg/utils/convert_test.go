package utils

import (
	"reflect"
	"testing"
)

func TestBytesToReadOnlyString(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "TestBytesToReadOnlyString",
			args: args{
				b: []byte("hello world"),
			},
			want: "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BytesToReadOnlyString(tt.args.b); got != tt.want {
				t.Errorf("BytesToReadOnlyString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringToReadOnlyBytes(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name   string
		args   args
		wantBs []byte
	}{
		{
			name: "TestStringToReadOnlyBytes",
			args: args{
				s: "hello world",
			},
			wantBs: []byte("hello world"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotBs := StringToReadOnlyBytes(tt.args.s); !reflect.DeepEqual(gotBs, tt.wantBs) {
				t.Errorf("StringToReadOnlyBytes() = %v, want %v", gotBs, tt.wantBs)
			}
		})
	}
}

package services_test

import (
	"reflect"
	"testing"

	"github.com/yccoskun/website/internal/services"
)

func TestExtractMediaIDs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []int64
	}{
		{
			name: "digit bounded longer id",
			in:   []string{"See ![x](/media/12) and /media/123 trailing"},
			want: []int64{12, 123},
		},
		{
			name: "prefix not matched as shorter id",
			in:   []string{"![x](/media/10)"},
			want: []int64{10},
		},
		{
			name: "end of string",
			in:   []string{"link /media/7"},
			want: []int64{7},
		},
		{
			name: "dedupe across md and html",
			in: []string{
				"![a](/media/5)",
				`<img src="/media/5"><img src="/media/6">`,
			},
			want: []int64{5, 6},
		},
		{
			name: "skip zero",
			in:   []string{"/media/0 /media/3"},
			want: []int64{3},
		},
		{
			name: "empty",
			in:   []string{"no media here", ""},
			want: []int64{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := services.ExtractMediaIDs(tc.in...)
			if got == nil {
				got = []int64{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ExtractMediaIDs(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

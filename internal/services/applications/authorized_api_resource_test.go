package applications

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDiffScopes(t *testing.T) {
	tests := []struct {
		name        string
		old         []string
		new         []string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name:        "add one",
			old:         []string{"a"},
			new:         []string{"a", "b"},
			wantAdded:   []string{"b"},
			wantRemoved: nil,
		},
		{
			name:        "remove one",
			old:         []string{"a", "b"},
			new:         []string{"a"},
			wantAdded:   nil,
			wantRemoved: []string{"b"},
		},
		{
			name:        "add and remove",
			old:         []string{"a", "b"},
			new:         []string{"b", "c"},
			wantAdded:   []string{"c"},
			wantRemoved: []string{"a"},
		},
		{
			name:        "no change",
			old:         []string{"a", "b"},
			new:         []string{"a", "b"},
			wantAdded:   nil,
			wantRemoved: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			added, removed := diffScopes(tc.old, tc.new)
			if !reflect.DeepEqual(added, tc.wantAdded) {
				t.Errorf("added: want %v, got %v", tc.wantAdded, added)
			}
			if !reflect.DeepEqual(removed, tc.wantRemoved) {
				t.Errorf("removed: want %v, got %v", tc.wantRemoved, removed)
			}
		})
	}
}

func TestScopesFromSet_Sorted(t *testing.T) {
	set, diags := types.SetValueFrom(context.Background(), types.StringType,
		[]string{"z_scope", "a_scope", "m_scope"})
	if diags.HasError() {
		t.Fatalf("building set: %v", diags)
	}

	got, d := scopesFromSet(context.Background(), set)
	if d.HasError() {
		t.Fatalf("scopesFromSet: %v", d)
	}
	want := []string{"a_scope", "m_scope", "z_scope"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want sorted %v, got %v", want, got)
	}
}

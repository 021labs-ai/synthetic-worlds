package services

import (
	"testing"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

func TestMatchFixtureExactInput(t *testing.T) {
	fixtures := []domain.Fixture{
		{ToolName: "crm_get_deal", Input: map[string]any{"id": 1}, Output: map[string]any{"amount": 100}},
		{ToolName: "crm_get_deal", Input: map[string]any{"id": 2}, Output: map[string]any{"amount": 200}},
	}
	out, ok := MatchFixture(fixtures, "crm_get_deal", map[string]any{"id": 2})
	if !ok || out["amount"] != 200 {
		t.Fatalf("expected exact match for id=2, got ok=%v out=%v", ok, out)
	}
	// key order and int/float representation must not matter
	out, ok = MatchFixture(fixtures, "crm_get_deal", map[string]any{"id": float64(1)})
	if !ok || out["amount"] != 100 {
		t.Fatalf("expected float64 id=1 to match, got ok=%v out=%v", ok, out)
	}
}

func TestMatchFixtureToolDefaultAndMiss(t *testing.T) {
	fixtures := []domain.Fixture{
		{ToolName: "crm_list_deals", Output: map[string]any{"deals": []any{}}},
	}
	if out, ok := MatchFixture(fixtures, "crm_list_deals", map[string]any{"whatever": true}); !ok || out == nil {
		t.Fatal("input-less fixture should act as tool-level default")
	}
	if _, ok := MatchFixture(fixtures, "crm_delete_all", nil); ok {
		t.Fatal("unseeded tool must not match")
	}
	// exact-input fixture must NOT match a different input
	fixtures = append(fixtures, domain.Fixture{
		ToolName: "get", Input: map[string]any{"id": 1}, Output: map[string]any{"v": 1}})
	if _, ok := MatchFixture(fixtures, "get", map[string]any{"id": 99}); ok {
		t.Fatal("mismatched input must not match an exact-input fixture")
	}
}

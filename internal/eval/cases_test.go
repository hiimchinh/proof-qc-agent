package eval

import (
	"strings"
	"testing"
)

func TestParseCasesKeepsFileNamesAsWritten(t *testing.T) {
	cs, err := ParseCases([]byte(`[{"id":"a","file":"a.png","order":{"customer":"Ana","product":"die_cut","width_in":3,"height_in":3},"want":"ready_for_proof"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].File != "a.png" || cs[0].Order.Customer != "Ana" {
		t.Fatalf("cases = %+v", cs)
	}
}

func TestParseCasesRejectsInvalidOrders(t *testing.T) {
	_, err := ParseCases([]byte(`[{"id":"bad","file":"a.png","order":{"product":"circle","width_in":3,"height_in":3}}]`))
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("err = %v, want one naming case bad", err)
	}
}

package usage

import (
	"strings"
	"testing"
)

func TestTypeSafePriceUsesDocumentedResponseVersion(t *testing.T) {
	t.Parallel()
	price, version, ok := LookupPriceVersion("jev-1.14.0")
	if !ok || !price.ExactModel || !strings.HasPrefix(version, "jev-1.14.0@") {
		t.Fatalf("documented response version unpriced: known=%v version=%q", ok, version)
	}
	approx(t, price.InputPerMTok, 0.042, "Jev input per million")
	approx(t, price.OutputPerMTok, 0, "Jev output per million")
	approx(t, price.Cost(1_000_000, 9_000_000), 0.042, "input-only charge")
	approx(t, price.Cost(0, 1_000_000), 0, "free output")
	for _, model := range []string{"jev-latest", "jev-preview", "jev-1.15.0", "jev-1.14.1", "jev-1.14.01", "jev-1.14.0-preview", ""} {
		if _, _, known := LookupPriceVersion(model); known {
			t.Errorf("undocumented identity %q treated as priced", model)
		}
	}
}

func TestExactPriceKeepsExistingPrefixAndVersionContracts(t *testing.T) {
	_, legacyVersion, known := LookupPriceVersion("gpt-6.1-sol-extra")
	if !known || legacyVersion != "gpt-6.1-sol@425613cd" {
		t.Fatalf("existing prefix/version changed: %v %q", known, legacyVersion)
	}
	price := Price{ExactModel: true, InputPerMTok: 1}
	RegisterPrice("exact-price-fixture", price)
	if _, _, ok := LookupPriceVersion("exact-price-fixture"); !ok {
		t.Fatal("exact registered price did not match its identity")
	}
	if _, _, ok := LookupPriceVersion("exact-price-fixture-suffix"); ok {
		t.Fatal("exact registered price matched another identity")
	}
	exactVersion := price.version("exact-price-fixture")
	price.ExactModel = false
	RegisterPrice("exact-price-fixture", price)
	_, prefixVersion, ok := LookupPriceVersion("exact-price-fixture-suffix")
	if !ok || prefixVersion == exactVersion {
		t.Fatal("operator prefix behavior or matching-mode version identity was lost")
	}
}

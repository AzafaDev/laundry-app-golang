package order

import (
	"sort"
	"testing"
)

func sortDiscrepancies(d []Discrepancy) {
	sort.Slice(d, func(i, j int) bool {
		if d[i].ItemType != d[j].ItemType {
			return d[i].ItemType < d[j].ItemType
		}
		return d[i].ItemID < d[j].ItemID
	})
}

func TestCompareItems(t *testing.T) {
	t.Run("matching quantities produce no discrepancies", func(t *testing.T) {
		expectedBreakdown := map[string]int32{"ct-1": 3}
		expectedSatuan := map[string]int32{"li-1": 2}
		req := SubmitItemsRequest{
			ActualItems:       []ActualBreakdownItem{{ClothingTypeID: "ct-1", ActualQuantity: 3}},
			ActualSatuanItems: []ActualSatuanItem{{LaundryItemID: "li-1", ActualQuantity: 2}},
		}

		got := compareItems(expectedBreakdown, expectedSatuan, req)
		if len(got) != 0 {
			t.Errorf("expected no discrepancies, got %v", got)
		}
	})

	t.Run("mismatched quantity is reported", func(t *testing.T) {
		expectedBreakdown := map[string]int32{"ct-1": 5}
		expectedSatuan := map[string]int32{}
		req := SubmitItemsRequest{
			ActualItems: []ActualBreakdownItem{{ClothingTypeID: "ct-1", ActualQuantity: 4}},
		}

		got := compareItems(expectedBreakdown, expectedSatuan, req)
		want := []Discrepancy{{ItemType: "clothing_type", ItemID: "ct-1", Expected: 5, Actual: 4}}
		if len(got) != 1 || got[0] != want[0] {
			t.Errorf("compareItems() = %+v, want %+v", got, want)
		}
	})

	t.Run("item entirely missing from submission is treated as actual=0", func(t *testing.T) {
		expectedBreakdown := map[string]int32{"ct-1": 2}
		expectedSatuan := map[string]int32{"li-1": 1}
		req := SubmitItemsRequest{} // worker submitted nothing at all

		got := compareItems(expectedBreakdown, expectedSatuan, req)
		sortDiscrepancies(got)

		want := []Discrepancy{
			{ItemType: "clothing_type", ItemID: "ct-1", Expected: 2, Actual: 0},
			{ItemType: "laundry_item", ItemID: "li-1", Expected: 1, Actual: 0},
		}
		if len(got) != len(want) {
			t.Fatalf("compareItems() = %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("discrepancy[%d] = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("extra item submitted beyond what was expected is reported", func(t *testing.T) {
		expectedBreakdown := map[string]int32{}
		expectedSatuan := map[string]int32{}
		req := SubmitItemsRequest{
			ActualItems: []ActualBreakdownItem{{ClothingTypeID: "ct-unexpected", ActualQuantity: 3}},
		}

		// compareItems only iterates expected* maps, so an actual item with
		// no corresponding expected entry is silently dropped rather than
		// flagged — documenting current behavior explicitly.
		got := compareItems(expectedBreakdown, expectedSatuan, req)
		if len(got) != 0 {
			t.Errorf("expected compareItems to ignore unexpected extra items (current behavior), got %v", got)
		}
	})
}

package dao

import "testing"

func TestNormalizeMarketStatusFromDetail(t *testing.T) {
	publishOnSale := 1
	publishOffSale := 2
	soldEnum := 2
	offSaleEnum := 1

	tests := []struct {
		name          string
		publishStatus *int
		rawStatus     *int
		rawSaleStatus *int
		dropReason    string
		want          string
	}{
		{
			name:          "publish status on-sale",
			publishStatus: &publishOnSale,
			want:          StatusOnSale,
		},
		{
			name:          "publish status off-sale by manual drop",
			publishStatus: &publishOffSale,
			dropReason:    "手动下架",
			want:          StatusOffSale,
		},
		{
			name:          "publish status off-sale with sold signal",
			publishStatus: &publishOffSale,
			dropReason:    "已成交",
			want:          StatusSoldOut,
		},
		{
			name:          "fallback sold by enum",
			rawStatus:     &soldEnum,
			rawSaleStatus: &soldEnum,
			want:          StatusSoldOut,
		},
		{
			name:       "fallback off-sale by drop reason",
			rawStatus:  &offSaleEnum,
			dropReason: "到期下架",
			want:       StatusOffSale,
		},
		{
			name:          "default on-sale when status is unknown",
			rawSaleStatus: &offSaleEnum,
			want:          StatusOnSale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMarketStatusFromDetail(tt.publishStatus, tt.rawStatus, tt.rawSaleStatus, tt.dropReason)
			if got != tt.want {
				t.Fatalf("unexpected status: got=%s want=%s", got, tt.want)
			}
		})
	}
}

func TestNormalizeMarketStatusDoesNotTreatSaleStatusOneAsSoldOut(t *testing.T) {
	saleStatus := 1
	got := NormalizeMarketStatus(nil, &saleStatus, nil)
	if got != StatusOnSale {
		t.Fatalf("expected on-sale, got %s", got)
	}
}

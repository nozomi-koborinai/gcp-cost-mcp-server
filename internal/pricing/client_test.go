package pricing

import (
	"testing"
)

func TestClient_CalculateCost(t *testing.T) {
	tests := []struct {
		name        string
		rate        *Rate
		usageAmount float64
		wantCost    float64
		wantErr     bool
	}{
		{
			name: "Single tier - simple calculation",
			rate: &Rate{
				Tiers: []Tier{
					{
						StartAmount: Amount{Value: "0"},
						ListPrice:   Money{Units: "0", Nanos: 100000000}, // $0.10 per unit
					},
				},
			},
			usageAmount: 100,
			wantCost:    10.0, // 100 * $0.10
			wantErr:     false,
		},
		{
			name: "Single tier - with units",
			rate: &Rate{
				Tiers: []Tier{
					{
						StartAmount: Amount{Value: "0"},
						ListPrice:   Money{Units: "1", Nanos: 500000000}, // $1.50 per unit
					},
				},
			},
			usageAmount: 10,
			wantCost:    15.0, // 10 * $1.50
			wantErr:     false,
		},
		{
			name: "Multiple tiers - basic tiered pricing",
			rate: &Rate{
				Tiers: []Tier{
					{
						StartAmount: Amount{Value: "0"},
						ListPrice:   Money{Units: "0", Nanos: 100000000}, // $0.10 for first 100
					},
					{
						StartAmount: Amount{Value: "100"},
						ListPrice:   Money{Units: "0", Nanos: 50000000}, // $0.05 after 100
					},
				},
			},
			usageAmount: 150,
			wantCost:    12.5, // 100 * $0.10 + 50 * $0.05
			wantErr:     false,
		},
		{
			name: "Zero usage",
			rate: &Rate{
				Tiers: []Tier{
					{
						StartAmount: Amount{Value: "0"},
						ListPrice:   Money{Units: "0", Nanos: 100000000},
					},
				},
			},
			usageAmount: 0,
			wantCost:    0,
			wantErr:     false,
		},
		{
			name:        "Nil rate",
			rate:        nil,
			usageAmount: 100,
			wantCost:    0,
			wantErr:     true,
		},
		{
			name: "Empty tiers",
			rate: &Rate{
				Tiers: []Tier{},
			},
			usageAmount: 100,
			wantCost:    0,
			wantErr:     true,
		},
		{
			name: "Very small price (Cloud Run vCPU-seconds)",
			rate: &Rate{
				Tiers: []Tier{
					{
						StartAmount: Amount{Value: "0"},
						ListPrice:   Money{Units: "0", Nanos: 24000}, // $0.000024 per vCPU-second
					},
				},
			},
			usageAmount: 2628000, // Full month of vCPU-seconds
			wantCost:    63.072,  // 2628000 * $0.000024
			wantErr:     false,
		},
		{
			name: "Free tier (zero price) first tier",
			rate: &Rate{
				Tiers: []Tier{
					{
						StartAmount: Amount{Value: "0"},
						ListPrice:   Money{Units: "0", Nanos: 0}, // Free for first 240000
					},
					{
						StartAmount: Amount{Value: "240000"},
						ListPrice:   Money{Units: "0", Nanos: 24000}, // $0.000024 after free tier
					},
				},
			},
			usageAmount: 480000,
			wantCost:    5.76, // 240000 * $0 + 240000 * $0.000024
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCost, err := CalculateCost(tt.rate, tt.usageAmount)

			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateCost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Use a tolerance for floating point comparison
				tolerance := 0.0001
				if diff := gotCost - tt.wantCost; diff < -tolerance || diff > tolerance {
					t.Errorf("CalculateCost() = %v, want %v (diff: %v)", gotCost, tt.wantCost, diff)
				}
			}
		})
	}
}

func TestAmount_Float64(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    float64
		wantErr bool
	}{
		{name: "empty string is zero", value: "", want: 0},
		{name: "integer", value: "100", want: 100},
		{name: "decimal", value: "0.5", want: 0.5},
		{name: "invalid", value: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Amount{Value: tt.value}.Float64()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Float64() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Float64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMoney_UnitPrice(t *testing.T) {
	tests := []struct {
		name    string
		money   Money
		want    float64
		wantErr bool
	}{
		{name: "units and nanos", money: Money{Units: "1", Nanos: 500000000}, want: 1.5},
		{name: "nanos only with empty units", money: Money{Nanos: 100000000}, want: 0.1},
		{name: "zero", money: Money{}, want: 0},
		{name: "invalid units", money: Money{Units: "x"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.money.UnitPrice()
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnitPrice() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("UnitPrice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateCost_InvalidTierData(t *testing.T) {
	rate := &Rate{
		Tiers: []Tier{{
			StartAmount: Amount{Value: "not-a-number"},
			ListPrice:   Money{Units: "1"},
		}},
	}
	if _, err := CalculateCost(rate, 100); err == nil {
		t.Fatal("CalculateCost should return error for invalid tier start amount")
	}

	rate = &Rate{
		Tiers: []Tier{{
			StartAmount: Amount{Value: "0"},
			ListPrice:   Money{Units: "not-a-number"},
		}},
	}
	if _, err := CalculateCost(rate, 100); err == nil {
		t.Fatal("CalculateCost should return error for invalid list price units")
	}
}

func TestDefaultPageSize(t *testing.T) {
	if DefaultPageSize != 5000 {
		t.Errorf("DefaultPageSize = %d, want 5000", DefaultPageSize)
	}
}

func TestBaseURL(t *testing.T) {
	expected := "https://cloudbilling.googleapis.com"
	if BaseURL != expected {
		t.Errorf("BaseURL = %s, want %s", BaseURL, expected)
	}
}

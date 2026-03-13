package service

import (
	"testing"
	"time"
)

func TestCalculateBilling(t *testing.T) {
	config := BillingConfig{
		HourlyRate: 30,
		MaxHours:   48,
		ExtraRate:  50,
	}

	tests := []struct {
		name           string
		totalDuration  time.Duration
		periodDays     int
		expectTotal    float64
		expectStandard float64
		expectExtra    float64
		expectProRated bool
	}{
		{
			name:           "Under threshold monthly",
			totalDuration:  40 * time.Hour,
			periodDays:     28,
			expectTotal:    1200,
			expectStandard: 1200,
			expectExtra:    0,
			expectProRated: true,
		},
		{
			name:           "Over threshold monthly",
			totalDuration:  50 * time.Hour,
			periodDays:     28,
			expectTotal:    1440 + 100, // 48*30 + 2*50
			expectStandard: 1440,
			expectExtra:    100,
			expectProRated: true,
		},
		{
			name:           "Weekly pro-rated (under)",
			totalDuration:  10 * time.Hour,
			periodDays:     7,
			expectTotal:    300,
			expectStandard: 300,
			expectExtra:    0,
			expectProRated: true,
		},
		{
			name:           "Weekly pro-rated (over)",
			totalDuration:  14 * time.Hour,
			periodDays:     7,
			expectTotal:    360 + 100, // 12*30 + 2*50
			expectStandard: 360,
			expectExtra:    100,
			expectProRated: true,
		},
		{
			name:           "Daily pro-rated (over)",
			totalDuration:  3 * time.Hour,
			periodDays:     1,
			expectTotal:    1.7142857142857142*30 + (3-1.7142857142857142)*50,
			expectStandard: 1.7142857142857142 * 30,
			expectExtra:    (3 - 1.7142857142857142) * 50,
			expectProRated: true,
		},
		{
			name:           "No threshold monthly (MaxHours = 0)",
			totalDuration:  100 * time.Hour,
			periodDays:     28,
			expectTotal:    3000,
			expectStandard: 3000,
			expectExtra:    0,
			expectProRated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentConfig := config
			if tt.name == "No threshold monthly (MaxHours = 0)" {
				currentConfig.MaxHours = 0
			}
			res := CalculateBilling(tt.totalDuration, currentConfig, tt.periodDays)
			if (res.TotalCost - tt.expectTotal) > 0.0001 || (res.TotalCost - tt.expectTotal) < -0.0001 {
				t.Errorf("TotalCost: expected %v, got %v", tt.expectTotal, res.TotalCost)
			}
			if (res.StandardCost - tt.expectStandard) > 0.0001 || (res.StandardCost - tt.expectStandard) < -0.0001 {
				t.Errorf("StandardCost: expected %v, got %v", tt.expectStandard, res.StandardCost)
			}
			if (res.ExtraCost - tt.expectExtra) > 0.0001 || (res.ExtraCost - tt.expectExtra) < -0.0001 {
				t.Errorf("ExtraCost: expected %v, got %v", tt.expectExtra, res.ExtraCost)
			}
			if res.IsProRated != tt.expectProRated {
				t.Errorf("IsProRated: expected %v, got %v", tt.expectProRated, res.IsProRated)
			}
		})
	}
}

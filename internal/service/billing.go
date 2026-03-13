package service

import (
	"time"
)

// BillingConfig holds the configuration for billing calculations.
type BillingConfig struct {
	HourlyRate float64
	MaxHours   float64
	ExtraRate  float64
}

// BillingResult holds the results of billing calculations.
type BillingResult struct {
	TotalCost    float64
	StandardCost float64
	ExtraCost    float64
	IsProRated   bool
	Threshold    float64
}

// CalculateBilling calculates the cost based on duration and report period.
// periodDays is the number of days in the report period.
// It assumes a standard billing month of 28 days (4 weeks) for pro-rating as per user request.
func CalculateBilling(totalDuration time.Duration, config BillingConfig, periodDays int) BillingResult {
	if config.HourlyRate <= 0 {
		return BillingResult{}
	}

	totalHours := totalDuration.Hours()
	
	// Default threshold is the configured monthly max
	threshold := config.MaxHours
	isProRated := false

	// If a period is specified, pro-rate the max hours
	if periodDays > 0 && config.MaxHours > 0 {
		// Use 28 days as a "month" to match user's 4-week logic
		threshold = (config.MaxHours / 28.0) * float64(periodDays)
		isProRated = true
	}

	var standardHours, extraHours float64
	if threshold > 0 && totalHours > threshold {
		standardHours = threshold
		extraHours = totalHours - threshold
	} else {
		standardHours = totalHours
		extraHours = 0
	}

	standardCost := standardHours * config.HourlyRate
	extraRate := config.ExtraRate
	if extraRate <= 0 {
		extraRate = config.HourlyRate // Default to standard rate if not set
	}
	extraCost := extraHours * extraRate

	return BillingResult{
		TotalCost:    standardCost + extraCost,
		StandardCost: standardCost,
		ExtraCost:    extraCost,
		IsProRated:   isProRated,
		Threshold:    threshold,
	}
}

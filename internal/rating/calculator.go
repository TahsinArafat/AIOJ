package rating

import (
	"math"
)

const (
	// DefaultRating is the starting rating for new users
	DefaultRating = 1500

	// K-factor determines how much ratings change
	// Higher K = more volatile ratings
	KFactor = 32

	// RatingPerfFactor scales the performance rating calculation
	RatingPerfFactor = 400
)

// CalculateRating computes new rating using Elo system adapted for competitive programming
// Based on Codeforces rating system
func CalculateRating(oldRating, rank, participants int) int {
	// For first contest, use default rating
	if oldRating == 0 {
		oldRating = DefaultRating
	}

	// Calculate performance rating
	// Better than expected = positive change
	performanceRating := calculatePerformanceRating(rank, participants, oldRating)

	// Calculate new rating with damping factor
	// New players have higher volatility
	dampingFactor := calculateDampingFactor(oldRating)

	ratingChange := int(float64(performanceRating-oldRating) * dampingFactor)
	newRating := oldRating + ratingChange

	// Ensure rating doesn't go below 0
	if newRating < 0 {
		newRating = 0
	}

	return newRating
}

// CalculateRatingChange returns the difference between old and new rating
func CalculateRatingChange(oldRating, newRating int) int {
	return newRating - oldRating
}

// calculateExpectedRank estimates where a player should rank based on rating
func calculateExpectedRank(rating, participants int) float64 {
	// Use logistic distribution to estimate expected rank
	// Higher rating = lower expected rank (better position)
	midRating := float64(DefaultRating)
	scale := float64(RatingPerfFactor)

	// Probability of beating an average player
	prob := 1.0 / (1.0 + math.Pow(10, (midRating-float64(rating))/scale))

	// Expected rank is inverse of probability
	return float64(participants) * (1.0 - prob)
}

// calculatePerformanceRating estimates rating based on actual performance
func calculatePerformanceRating(rank, participants, currentRating int) int {
	// Performance rating = rating that would make actual rank = expected rank
	// Use binary search to find this rating

	low, high := 0, 5000

	for low < high {
		mid := (low + high) / 2
		expectedRank := calculateExpectedRank(mid, participants)

		if expectedRank < float64(rank) {
			high = mid
		} else {
			low = mid + 1
		}
	}

	return low
}

// calculateDampingFactor returns how much ratings should change
// New players (low rating) have higher volatility
func calculateDampingFactor(rating int) float64 {
	if rating < 1000 {
		return 1.0 // Full change for new players
	} else if rating < 2000 {
		return 0.75 // Moderate change
	} else {
		return 0.5 // Experienced players change slowly
	}
}

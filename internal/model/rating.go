package model

import "time"

// Rating colors matching Codeforces
const (
	ColorNewbie                = "newbie"                 // < 1200
	ColorPupil                 = "pupil"                  // 1200-1399
	ColorSpecialist            = "specialist"             // 1400-1599
	ColorExpert                = "expert"                 // 1600-1899
	ColorCandidateMaster       = "candidate-master"       // 1900-2099
	ColorMaster                = "master"                 // 2100-2299
	ColorInternationalMaster   = "international-master"   // 2300-2399
	ColorGrandmaster           = "grandmaster"            // 2400-2599
	ColorInternationalGrandmaster = "international-grandmaster" // 2600-2899
	ColorLegendaryGrandmaster  = "legendary-grandmaster"  // 2900+
)

type RatingHistory struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ContestID    string    `json:"contest_id"`
	OldRating    int       `json:"old_rating"`
	NewRating    int       `json:"new_rating"`
	Rank         int       `json:"rank"`
	RatingChange int       `json:"rating_change"`
	CreatedAt    time.Time `json:"created_at"`
}

type RatingChange struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	OldRating    int    `json:"old_rating"`
	NewRating    int    `json:"new_rating"`
	RatingChange int    `json:"rating_change"`
	Rank         int    `json:"rank"`
	Color        string `json:"color"`
}

type ContestRatingRequest struct {
	ContestID string `json:"contest_id"`
}

// GetColor returns the color name for a given rating
func GetColor(rating int) string {
	switch {
	case rating >= 2900:
		return ColorLegendaryGrandmaster
	case rating >= 2600:
		return ColorInternationalGrandmaster
	case rating >= 2400:
		return ColorGrandmaster
	case rating >= 2300:
		return ColorInternationalMaster
	case rating >= 2100:
		return ColorMaster
	case rating >= 1900:
		return ColorCandidateMaster
	case rating >= 1600:
		return ColorExpert
	case rating >= 1400:
		return ColorSpecialist
	case rating >= 1200:
		return ColorPupil
	default:
		return ColorNewbie
	}
}

// GetColorHex returns the hex color for a given rating
func GetColorHex(rating int) string {
	switch {
	case rating >= 2900:
		return "#FF0000"
	case rating >= 2600:
		return "#FF0000"
	case rating >= 2400:
		return "#FF8C00"
	case rating >= 2300:
		return "#FF8C00"
	case rating >= 2100:
		return "#FFD700"
	case rating >= 1900:
		return "#AA00AA"
	case rating >= 1600:
		return "#0000FF"
	case rating >= 1400:
		return "#03A89E"
	case rating >= 1200:
		return "#008000"
	default:
		return "#808080"
	}
}

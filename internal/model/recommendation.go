package model

type RecommendationsResponse struct {
	Progression []ProblemListItem        `json:"progression"`
	WeakTags    WeakTagsRecommendations  `json:"weak_tags"`
	Hybrid      []ProblemListItem        `json:"hybrid"`
}

type WeakTagsRecommendations struct {
	Tags     []string         `json:"tags"`
	Problems []ProblemListItem `json:"problems"`
}

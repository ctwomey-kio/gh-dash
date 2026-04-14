package data

// teamResponse is the shape of a single entry from GET /user/teams.
type teamResponse struct {
	Slug string `json:"slug"`
}

// FetchMyTeamSlugs returns the slugs of all GitHub teams the authenticated user belongs to.
// It fetches up to 100 teams in a single request, which covers the vast majority of users.
func FetchMyTeamSlugs() ([]string, error) {
	client, err := getRESTClient()
	if err != nil {
		return nil, err
	}

	var teams []teamResponse
	if err := client.Get("user/teams?per_page=100", &teams); err != nil {
		return nil, err
	}

	slugs := make([]string, 0, len(teams))
	for _, t := range teams {
		slugs = append(slugs, t.Slug)
	}
	return slugs, nil
}

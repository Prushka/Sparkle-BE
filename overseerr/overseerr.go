package overseerr

import (
	"Sparkle/config"
	"Sparkle/discord"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Media represents the media information in each request
type Media struct {
	ID                  int    `json:"id"`
	MediaType           string `json:"mediaType"`
	TMDBID              int    `json:"tmdbId"`
	TVDBID              int    `json:"tvdbId"`
	IMDBID              *int   `json:"imdbId"`
	Status              int    `json:"status"`
	ServiceID           int    `json:"serviceId"`
	ExternalServiceSlug string `json:"externalServiceSlug"`
	PlexURL             string `json:"plexUrl"`
	ServiceURL          string `json:"serviceUrl"`
}

// User represents user information
type User struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

// Request represents a single request
type Request struct {
	ID            int    `json:"id"`
	Status        int    `json:"status"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	Type          string `json:"type"`
	IsAutoRequest bool   `json:"isAutoRequest"`
	Media         Media  `json:"media"`
	RequestedBy   User   `json:"requestedBy"`
	ModifiedBy    User   `json:"modifiedBy"`
	SeasonCount   int    `json:"seasonCount"`
}

// Response represents the overall structure of the API response
type Response struct {
	PageInfo struct {
		Pages    int `json:"pages"`
		PageSize int `json:"pageSize"`
		Results  int `json:"results"`
		Page     int `json:"page"`
	} `json:"pageInfo"`
	Results []Request `json:"results"`
}

func getOverseerr(path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // Ensure the context is canceled to free resources

	url := fmt.Sprintf("%s%s", config.TheConfig.OverSeerrURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Add("X-Api-Key", config.TheConfig.OverSeerrAPI)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("request timed out")
		}
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			discord.Errorf("Error closing: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user requests, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// GetUserRequests retrieves the list of requests for a specific user.
func GetUserRequests(userId int) (*Response, error) {
	body, err := getOverseerr(fmt.Sprintf("/api/v1/request?requestedBy=%d&take=5000", userId))
	if err != nil {
		return nil, err
	}
	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// MediaDetails represents the structure of a movie.
type MediaDetails struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"originalTitle"`
	Name          string `json:"name"`
}

// GetTitleById retrieves a movie/tv's title by its ID from the Overseerr API.
func GetTitleById(t string, id int) (string, error) {
	body, err := getOverseerr(fmt.Sprintf("/api/v1/%s/%d", t, id))
	if err != nil {
		return "", err
	}
	var media MediaDetails
	err = json.Unmarshal(body, &media)
	if err != nil {
		return "", err
	}

	if media.Name != "" {
		return media.Name, nil
	}
	if media.Title != "" {
		return media.Title, nil
	}
	return media.OriginalTitle, nil
}

type WatchlistResponse struct {
	Page         int               `json:"page"`
	TotalPages   int               `json:"totalPages"`
	TotalResults int               `json:"totalResults"`
	Results      []WatchlistResult `json:"results"`
}

type WatchlistResult struct {
	RatingKey string `json:"ratingKey"`
	Title     string `json:"title"`
	MediaType string `json:"mediaType"`
	TmdbId    int    `json:"tmdbId"`
}

func GetWatchlist(userId int) ([]WatchlistResult, error) {
	var allResults []WatchlistResult

	page := 1
	for {
		body, err := getOverseerr(fmt.Sprintf("/api/v1/user/%d/watchlist?page=%d", userId, page))
		if err != nil {
			return nil, err
		}

		var response WatchlistResponse
		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, err
		}

		if len(response.Results) == 0 {
			break
		}

		allResults = append(allResults, response.Results...)

		if page >= response.TotalPages {
			break
		}

		page++
	}

	return allResults, nil
}

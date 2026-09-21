package tofa

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"
)

const Endpoint = "https://api.tokenfactory.nebius.com/v1"

type Model struct {
	ID string `json:"id"`
}

func (a *App) models(project, key string) ([]Model, error) {
	endpoint := a.Endpoint
	if endpoint == "" {
		endpoint = Endpoint
	}
	u, err := url.Parse(endpoint + "/models")
	if err != nil {
		return nil, errors.New("invalid endpoint")
	}
	q := u.Query()
	q.Set("ai_project_id", project)
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client := a.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	// Never carry a saved key across a redirect, even on the same host.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := copyClient.Do(req)
	if err != nil {
		return nil, errors.New("model catalog connection failed; check network and endpoint availability")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model catalog returned HTTP %d; check API key, project ID and service availability", res.StatusCode)
	}
	const max = 4 << 20
	body, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil || len(body) > max {
		return nil, errors.New("model catalog unreadable or too large")
	}
	var result struct {
		Data []Model `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || result.Data == nil {
		return nil, errors.New("invalid model catalog response")
	}
	unique := map[string]bool{}
	models := []Model{}
	for _, m := range result.Data {
		if !validText(m.ID, 512) {
			return nil, errors.New("catalog contains an invalid model ID")
		}
		if !unique[m.ID] {
			models = append(models, m)
			unique[m.ID] = true
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

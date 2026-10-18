package assistant

import (
	"context"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/assistant_setting"
)

const maxModelListBytes = 2 << 20

type modelListEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListModels fetches the upstream model list for one profile. The model name is
// not required, because the admin uses this before selecting a model.
func ListModels(ctx context.Context, profile assistant_setting.Profile) ([]string, error) {
	baseURL, apiKey, err := ResolveModelListConnection(profile)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/models"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Accept", "application/json")
	client := service.GetHttpClient()
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxModelListBytes))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(body))
		if len(message) > 500 {
			message = message[:500]
		}
		message = strings.ReplaceAll(message, apiKey, "[REDACTED]")
		return nil, &UpstreamError{StatusCode: response.StatusCode, RequestID: response.Header.Get("x-request-id"), Message: message}
	}
	return parseModelList(body), nil
}

func parseModelList(body []byte) []string {
	models := make([]string, 0)
	seen := make(map[string]struct{})

	var payload struct {
		Data   []modelListEntry `json:"data"`
		Models []modelListEntry `json:"models"`
	}
	if err := common.Unmarshal(body, &payload); err == nil {
		for _, entry := range payload.Data {
			appendModelID(&models, seen, entry.ID)
		}
		for _, entry := range payload.Models {
			appendModelID(&models, seen, entry.ID)
		}
	}

	if len(models) == 0 {
		var stringModels struct {
			Models []string `json:"models"`
		}
		if err := common.Unmarshal(body, &stringModels); err == nil {
			for _, id := range stringModels.Models {
				appendModelID(&models, seen, id)
			}
		}
	}
	if len(models) == 0 {
		var entries []modelListEntry
		if err := common.Unmarshal(body, &entries); err == nil {
			for _, entry := range entries {
				appendModelID(&models, seen, entry.ID)
			}
		}
	}
	if len(models) == 0 {
		var ids []string
		if err := common.Unmarshal(body, &ids); err == nil {
			for _, id := range ids {
				appendModelID(&models, seen, id)
			}
		}
	}
	sort.Strings(models)
	return models
}

func appendModelID(models *[]string, seen map[string]struct{}, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	if _, ok := seen[id]; ok {
		return
	}
	seen[id] = struct{}{}
	*models = append(*models, id)
}

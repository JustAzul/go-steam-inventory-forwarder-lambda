package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type SteamAPIResponse struct {
	Assets              []interface{} `json:"assets"`
	Descriptions        []interface{} `json:"descriptions"`
	MoreItems           int           `json:"more_items"`
	LastAssetID         string        `json:"last_assetid"`
	TotalInventoryCount int           `json:"total_inventory_count"`
	Success             int           `json:"success"`
	Rwgrsn              int           `json:"rwgrsn"`
	FakeRedirect        int           `json:"fake_redirect"`
}

var httpClient = &http.Client{
	Timeout: time.Second * 10,
}

func fetchExternalAPI(url string) (SteamAPIResponse, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return SteamAPIResponse{}, fmt.Errorf("failed to make GET request to external API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SteamAPIResponse{}, fmt.Errorf("external API returned a non-200 status code: %d", resp.StatusCode)
	}

	var apiResponse SteamAPIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return SteamAPIResponse{}, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return apiResponse, nil
}

func buildExternalAPIURL(request events.APIGatewayProxyRequest, apiKey string) string {
	startAssetID := request.QueryStringParameters["start_assetid"]
	steamID64 := request.PathParameters["steam_id_64"]
	appID := request.PathParameters["appid"]
	contextID := request.PathParameters["context_id"]

	externalAPIURL := fmt.Sprintf("https://steam.supply/API/%s/loadinventory?steamid=%s&appid=%s&contextid=%s", apiKey, steamID64, appID, contextID)
	if startAssetID != "" {
		externalAPIURL = fmt.Sprintf("%s&start_assetid=%s", externalAPIURL, startAssetID)
	}

	return externalAPIURL
}

func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	apiKey := os.Getenv("STEAM_SUPPLY_API_KEY")
	externalAPIURL := buildExternalAPIURL(request, apiKey)

	for {
		externalAPIResponse, err := fetchExternalAPI(externalAPIURL)
		if err != nil {
			return events.APIGatewayProxyResponse{}, err
		}

		if externalAPIResponse.FakeRedirect == 1 {
			externalAPIURL = fmt.Sprintf("%s&start_assetid=%s", externalAPIURL, externalAPIResponse.LastAssetID)
		} else {
			responseJSON, err := json.Marshal(externalAPIResponse)
			if err != nil {
				return events.APIGatewayProxyResponse{}, fmt.Errorf("failed to marshal JSON response: %v", err)
			}

			return events.APIGatewayProxyResponse{
				Body:       string(responseJSON),
				StatusCode: 200,
			}, nil
		}
	}
}

func main() {
	lambda.Start(handleRequest)
}

package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

func isBareForecastRequest(text string) bool {
	switch strings.Join(strings.Fields(strings.TrimRight(strings.TrimSpace(text), "?!.。")), " ") {
	case "날씨", "오늘 날씨", "현재 날씨", "지금 날씨", "내일 날씨", "내일 아침 날씨", "내일 오후 날씨", "tomorrow's weather", "weather tomorrow":
		return true
	}
	return false
}

func weatherFollowupQuestion(question string, request map[string]interface{}) string {
	if strings.Contains(question, "날씨") || strings.Contains(strings.ToLower(question), "weather") {
		return question
	}
	if !strings.Contains(question, "스킬을 써") && !strings.Contains(question, "위치를 찾") {
		return ""
	}
	messages, _ := request["messages"].([]interface{})
	for i := len(messages) - 2; i >= max(0, len(messages)-7); i-- {
		message, _ := messages[i].(map[string]interface{})
		if message["role"] == "user" {
			text, _ := message["content"].(string)
			if strings.Contains(text, "날씨") || strings.Contains(strings.ToLower(text), "weather") {
				return text
			}
		}
	}
	return ""
}

func forecastCoordinates(raw string) string {
	var location struct {
		Latitude  *float64 `json:"lat"`
		Longitude *float64 `json:"lon"`
	}
	if json.Unmarshal([]byte(raw), &location) != nil || location.Latitude == nil || location.Longitude == nil {
		return ""
	}
	lat, lon := *location.Latitude, *location.Longitude
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return ""
	}
	return fmt.Sprintf("%.2f, %.2f", lat, lon)
}

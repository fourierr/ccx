package common

import (
	"encoding/json"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/scheduler"
	"github.com/BenedictKing/ccx/internal/utils"
)

// BuildMessagesContextRequirement 估算 Messages 请求上下文需求。
func BuildMessagesContextRequirement(bodyBytes []byte, cfg config.ContextRoutingConfig) *scheduler.ContextRequirement {
	inputTokens := utils.EstimateRequestTokens(bodyBytes)
	outputTokens, explicit := extractFirstInt(bodyBytes, "max_tokens")
	return buildContextRequirement(inputTokens, outputTokens, explicit, cfg)
}

// BuildResponsesContextRequirement 估算 Responses 请求上下文需求。
func BuildResponsesContextRequirement(bodyBytes []byte, cfg config.ContextRoutingConfig) *scheduler.ContextRequirement {
	inputTokens := utils.EstimateResponsesRequestTokens(bodyBytes)
	outputTokens, explicit := extractFirstInt(bodyBytes, "max_output_tokens", "max_tokens")
	return buildContextRequirement(inputTokens, outputTokens, explicit, cfg)
}

// BuildChatContextRequirement 估算 Chat Completions 请求上下文需求。
func BuildChatContextRequirement(bodyBytes []byte, cfg config.ContextRoutingConfig) *scheduler.ContextRequirement {
	inputTokens := utils.EstimateRequestTokens(bodyBytes)
	outputTokens, explicit := extractFirstInt(bodyBytes, "max_completion_tokens", "max_tokens")
	return buildContextRequirement(inputTokens, outputTokens, explicit, cfg)
}

// BuildGeminiContextRequirement 估算 Gemini 请求上下文需求，并计入 thinkingBudget。
func BuildGeminiContextRequirement(bodyBytes []byte, cfg config.ContextRoutingConfig) *scheduler.ContextRequirement {
	inputTokens := utils.EstimateGeminiRequestTokens(bodyBytes)
	outputTokens, explicit := extractGeminiMaxOutputTokens(bodyBytes)
	thinkingBudget := extractGeminiThinkingBudget(bodyBytes)
	return buildContextRequirement(inputTokens+thinkingBudget, outputTokens, explicit, cfg)
}

func buildContextRequirement(inputTokens, outputTokens int, explicit bool, cfg config.ContextRoutingConfig) *scheduler.ContextRequirement {
	if outputTokens <= 0 {
		outputTokens = cfg.EffectiveOutputReserveTokens()
		explicit = false
	}
	required := inputTokens + outputTokens
	if required <= 0 {
		return nil
	}
	return &scheduler.ContextRequirement{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		RequiredTokens:    required,
		ExplicitOutputMax: explicit,
	}
}

// ApplyAgentModelProfile 将下游 agent 模型预设上下文写入当前请求需求。
func ApplyAgentModelProfile(requirement *scheduler.ContextRequirement, requestModel string, cfg config.Config) {
	if requirement == nil || requestModel == "" {
		return
	}
	resolved := config.ResolveAgentModelProfile(requestModel, cfg.AgentModelProfiles)
	if !resolved.Known || resolved.MatchedPattern == "*" || resolved.Profile.ContextWindowTokens <= 0 {
		return
	}
	requirement.MinimumContextWindowTokens = resolved.Profile.ContextWindowTokens
}

func extractFirstInt(bodyBytes []byte, keys ...string) (int, bool) {
	var req map[string]interface{}
	if len(bodyBytes) == 0 || json.Unmarshal(bodyBytes, &req) != nil {
		return 0, false
	}
	for _, key := range keys {
		if n, ok := numberFromMap(req, key); ok {
			return n, true
		}
	}
	return 0, false
}

func extractGeminiMaxOutputTokens(bodyBytes []byte) (int, bool) {
	var req map[string]interface{}
	if len(bodyBytes) == 0 || json.Unmarshal(bodyBytes, &req) != nil {
		return 0, false
	}
	if n, ok := numberFromMap(req, "maxOutputTokens"); ok {
		return n, true
	}
	if generationConfig, ok := req["generationConfig"].(map[string]interface{}); ok {
		return numberFromMap(generationConfig, "maxOutputTokens")
	}
	return 0, false
}

func extractGeminiThinkingBudget(bodyBytes []byte) int {
	var req map[string]interface{}
	if len(bodyBytes) == 0 || json.Unmarshal(bodyBytes, &req) != nil {
		return 0
	}
	if thinkingConfig, ok := req["thinkingConfig"].(map[string]interface{}); ok {
		if n, ok := numberFromMap(thinkingConfig, "thinkingBudget"); ok {
			return n
		}
	}
	if generationConfig, ok := req["generationConfig"].(map[string]interface{}); ok {
		if thinkingConfig, ok := generationConfig["thinkingConfig"].(map[string]interface{}); ok {
			if n, ok := numberFromMap(thinkingConfig, "thinkingBudget"); ok {
				return n
			}
		}
	}
	return 0
}

func numberFromMap(m map[string]interface{}, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if n > 0 {
			return int(n), true
		}
	case int:
		if n > 0 {
			return n, true
		}
	}
	return 0, false
}

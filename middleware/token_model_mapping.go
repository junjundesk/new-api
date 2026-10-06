package middleware

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// applyTokenModelMapping rewrites the requested model with the key's exact-match
// mapping (one hop only) before model limits, channel selection and billing run,
// so every later step sees the target model. A non-empty reasoning effort on the
// mapping replaces the client's effort in the cached JSON body.
func applyTokenModelMapping(c *gin.Context, modelRequest *ModelRequest) error {
	if modelRequest == nil || modelRequest.Model == "" {
		return nil
	}
	mappings, ok := common.GetContextKeyType[[]setting.TokenModelMapping](c, constant.ContextKeyTokenModelMappings)
	if !ok {
		return nil
	}
	var matched *setting.TokenModelMapping
	for i := range mappings {
		if mappings[i].SourceModel == modelRequest.Model {
			matched = &mappings[i]
			break
		}
	}
	if matched == nil {
		return nil
	}
	modelRequest.Model = matched.TargetModel

	if !strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json") {
		return nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return err
	}
	body, err := storage.Bytes()
	if err != nil {
		return err
	}
	if !gjson.ValidBytes(body) {
		return nil
	}
	updated := body
	if gjson.GetBytes(updated, "model").Exists() {
		if updated, err = sjson.SetBytes(updated, "model", matched.TargetModel); err != nil {
			return err
		}
	}
	if matched.ReasoningEffort != "" {
		if updated, err = sjson.SetBytes(updated, reasoningEffortPath(c.Request.URL.Path), matched.ReasoningEffort); err != nil {
			return err
		}
	}
	if len(updated) == len(body) && string(updated) == string(body) {
		return nil
	}
	newStorage, err := common.CreateBodyStorage(updated)
	if err != nil {
		return err
	}
	_ = storage.Close()
	c.Set(common.KeyBodyStorage, newStorage)
	c.Request.ContentLength = int64(len(updated))
	return nil
}

// reasoningEffortPath returns where each relay format carries reasoning effort.
func reasoningEffortPath(path string) string {
	switch {
	case strings.Contains(path, "/v1/responses"):
		return "reasoning.effort"
	case strings.Contains(path, "/v1/messages"):
		return "output_config.effort"
	case strings.Contains(path, ":generateContent") || strings.Contains(path, ":streamGenerateContent"):
		return "generationConfig.thinkingConfig.thinkingLevel"
	default:
		return "reasoning_effort"
	}
}

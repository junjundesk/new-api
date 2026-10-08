package helper

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

// ModelMappedHelper applies the selected channel's model mapping to the
// request. A mapping entry holds one or more upstream candidates; the request
// uses the candidate at the current position and advances to the next one when
// the upstream reports the model as unavailable (see model_failover.go).
//
// Single-candidate entries keep the legacy behaviour: the candidate is treated
// as another mapping source and chains through further entries, with cycle
// detection. Multi-candidate entries use their candidates verbatim, because a
// candidate list describes alternatives of the same upstream model family and
// chaining through it would defeat the fallback.
func ModelMappedHelper(c *gin.Context, info *common.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &common.ChannelMeta{}
	}

	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap, err := model.ParseChannelModelMapping(modelMapping)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		upstreamModel, isMapped, err := resolveMappedModel(info, modelMap)
		if err != nil {
			return err
		}
		info.IsModelMapped = isMapped
		if isMapped {
			info.UpstreamModelName = upstreamModel
		}
	}

	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}

// resolveMappedModel picks the upstream model for this attempt from the
// channel mapping and records candidate progress on info.
func resolveMappedModel(info *common.RelayInfo, modelMap model.ChannelModelMapping) (string, bool, error) {
	if len(modelMap) == 0 {
		return "", false, nil
	}

	currentModel := info.OriginModelName
	visitedModels := map[string]bool{currentModel: true}
	for {
		candidates, exists := modelMap[currentModel]
		if !exists || len(candidates) == 0 {
			break
		}

		if len(candidates) > 1 {
			return selectModelCandidate(info, candidates), true, nil
		}

		// 单候选：沿用链式重定向，最终使用链尾的模型
		mappedModel := candidates[0]
		if visitedModels[mappedModel] {
			if mappedModel == currentModel {
				// 链尾自映射（a -> b -> b）是合法终点，保留链尾模型；
				// 只有首个模型自映射（a -> a）才视为未配置映射。
				if currentModel == info.OriginModelName {
					return "", false, nil
				}
				return currentModel, true, nil
			}
			return "", false, errors.New("model_mapping_contains_cycle")
		}
		visitedModels[mappedModel] = true
		currentModel = mappedModel
	}
	return currentModel, currentModel != info.OriginModelName, nil
}

// selectModelCandidate returns the candidate for the current attempt and
// advances the recorded position. The position resets when the request lands
// on a different channel, so a cross-channel retry starts from the first
// candidate again.
func selectModelCandidate(info *common.RelayInfo, candidates []string) string {
	state := info.ModelCandidate
	if state == nil || state.ChannelId != info.ChannelId {
		state = &common.ModelCandidateState{ChannelId: info.ChannelId}
		info.ModelCandidate = state
	}
	state.Candidates = candidates

	index := state.Index
	if index >= len(candidates) {
		index = len(candidates) - 1
	}
	if index < 0 {
		index = 0
	}
	return candidates[index]
}

// AdvanceModelCandidate moves the request to the next upstream candidate of
// the same channel and reports whether one is left to try.
func AdvanceModelCandidate(info *common.RelayInfo) bool {
	state := info.ModelCandidate
	if state == nil || len(state.Candidates) == 0 {
		return false
	}
	if state.Index+1 >= len(state.Candidates) {
		return false
	}
	state.Index++
	return true
}

// ModelCandidateExhausted reports whether a mapping with several candidates
// has already consumed every candidate on this channel.
func ModelCandidateExhausted(info *common.RelayInfo) bool {
	state := info.ModelCandidate
	if state == nil || len(state.Candidates) <= 1 {
		return false
	}
	return state.Index+1 >= len(state.Candidates)
}

// HasModelCandidates reports whether this attempt runs a multi-candidate
// mapping, i.e. whether candidate failover can apply at all.
func HasModelCandidates(info *common.RelayInfo) bool {
	return info != nil && info.ModelCandidate != nil && len(info.ModelCandidate.Candidates) > 1
}

// CurrentModelCandidate returns the candidate this attempt is using, or "" when
// the channel has no multi-candidate mapping.
func CurrentModelCandidate(info *common.RelayInfo) string {
	if !HasModelCandidates(info) {
		return ""
	}
	candidates := info.ModelCandidate.Candidates
	index := info.ModelCandidate.Index
	if index >= len(candidates) {
		index = len(candidates) - 1
	}
	if index < 0 {
		return ""
	}
	return candidates[index]
}

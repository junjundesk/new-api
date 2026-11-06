package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
// group2mappedModelIndex 按渠道 model_mapping 的源模型（左侧键）建立分组反向索引，
// 使请求模型只在 model_mapping 里出现、未写进渠道模型列表时也能选到渠道。
// 索引只在缓存初始化/刷新时构建，查询路径不访问数据库。
var group2mappedModelIndex map[string]*channelMappedModelIndex
var channelsIDM map[int]*Channel // all channels include disabled
// channel2advancedCustomConfig caches parsed Advanced Custom (type 58) configs so
// path-aware selection avoids re-parsing JSON per request. Refreshed on full sync.
var channel2advancedCustomConfig map[int]*dto.AdvancedCustomConfig
var channelSyncLock sync.RWMutex

// channelMappedModelIndex 保存单个分组内以 model_mapping 源模型为键的渠道索引。
// 精确键放在 exact 中，以 * 结尾的简单通配键放在 wildcard 中；
// wildcard 按前缀长度降序排列，保证最长前缀优先命中。
type channelMappedModelIndex struct {
	exact    map[string][]int
	wildcard []channelMappedWildcardEntry
}

type channelMappedWildcardEntry struct {
	prefix   string
	channels []int
}

func newChannelMappedModelIndex() *channelMappedModelIndex {
	return &channelMappedModelIndex{exact: make(map[string][]int)}
}

// wildcardMappingPrefix 判断 model_mapping 源模型是否为简单尾部通配（如
// "claude-sonnet-*"），返回去掉 * 后的前缀。"*" 本身匹配任意模型。
func wildcardMappingPrefix(source string) (string, bool) {
	if !strings.HasSuffix(source, "*") {
		return "", false
	}
	return strings.TrimSuffix(source, "*"), true
}

// add 把一个渠道登记到某个源模型下，重复登记会被忽略。
func (index *channelMappedModelIndex) add(modelKey string, channelId int) {
	if index == nil || channelId <= 0 || modelKey == "" {
		return
	}
	if prefix, isWildcard := wildcardMappingPrefix(modelKey); isWildcard {
		for i := range index.wildcard {
			if index.wildcard[i].prefix == prefix {
				index.wildcard[i].channels = appendUniqueChannelID(index.wildcard[i].channels, channelId)
				return
			}
		}
		index.wildcard = append(index.wildcard, channelMappedWildcardEntry{
			prefix:   prefix,
			channels: []int{channelId},
		})
		return
	}
	index.exact[modelKey] = appendUniqueChannelID(index.exact[modelKey], channelId)
}

// sortChannels 按渠道优先级降序整理候选列表，与 group2model2channels 保持一致。
func (index *channelMappedModelIndex) sortChannels(id2channel map[int]*Channel) {
	if index == nil {
		return
	}
	for modelKey, channelIds := range index.exact {
		sortChannelIDsByPriority(channelIds, id2channel)
		index.exact[modelKey] = channelIds
	}
	sort.SliceStable(index.wildcard, func(i, j int) bool {
		return len(index.wildcard[i].prefix) > len(index.wildcard[j].prefix)
	})
	for i := range index.wildcard {
		sortChannelIDsByPriority(index.wildcard[i].channels, id2channel)
	}
}

func sortChannelIDsByPriority(channelIds []int, id2channel map[int]*Channel) {
	sort.Slice(channelIds, func(i, j int) bool {
		left, leftOK := id2channel[channelIds[i]]
		right, rightOK := id2channel[channelIds[j]]
		if !leftOK || !rightOK {
			// 缺失的渠道排在后面，交给后续一致性错误处理。
			return leftOK && !rightOK
		}
		return left.GetPriority() > right.GetPriority()
	})
}

func appendUniqueChannelID(channelIds []int, channelId int) []int {
	for _, id := range channelIds {
		if id == channelId {
			return channelIds
		}
	}
	return append(channelIds, channelId)
}

func removeChannelID(channelIds []int, channelId int) []int {
	for i, id := range channelIds {
		if id == channelId {
			return append(channelIds[:i], channelIds[i+1:]...)
		}
	}
	return channelIds
}

// indexChannelModelMapping 把一个渠道的 model_mapping 源模型写入分组反向索引。
// 映射解析失败的渠道只记录日志并跳过，不影响其它渠道的正常选路。
func indexChannelModelMapping(indexes map[string]*channelMappedModelIndex, channel *Channel) {
	if channel == nil || channel.Status != common.ChannelStatusEnabled {
		return
	}
	rawMapping := channel.GetModelMapping()
	if rawMapping == "" {
		return
	}
	mapping, err := ParseChannelModelMapping(rawMapping)
	if err != nil {
		common.SysLog(fmt.Sprintf("渠道# %d 的 model_mapping 解析失败，已跳过映射选路索引: %s", channel.Id, err.Error()))
		return
	}
	if len(mapping) == 0 {
		return
	}
	for _, group := range strings.Split(channel.Group, ",") {
		if group == "" {
			continue
		}
		index := indexes[group]
		if index == nil {
			index = newChannelMappedModelIndex()
			indexes[group] = index
		}
		for source := range mapping {
			index.add(source, channel.Id)
		}
	}
}

// removeChannelFromMappedModelIndex 在渠道被禁用时立即把它从映射索引中剔除，
// 不必等待下一次全量刷新。
func removeChannelFromMappedModelIndex(channelId int) {
	for _, index := range group2mappedModelIndex {
		if index == nil {
			continue
		}
		for modelKey, channelIds := range index.exact {
			filtered := removeChannelID(channelIds, channelId)
			if len(filtered) == 0 {
				delete(index.exact, modelKey)
				continue
			}
			index.exact[modelKey] = filtered
		}
		wildcards := index.wildcard[:0]
		for i := range index.wildcard {
			index.wildcard[i].channels = removeChannelID(index.wildcard[i].channels, channelId)
			if len(index.wildcard[i].channels) > 0 {
				wildcards = append(wildcards, index.wildcard[i])
			}
		}
		index.wildcard = wildcards
	}
}

// getMappedModelChannels 在精确模型与归一化模型都未命中时，按渠道 model_mapping
// 的源模型查找渠道。精确键优先；否则按最长前缀命中通配键。
// Caller must hold channelSyncLock (read lock).
func getMappedModelChannels(group, modelName, requestPath string) []int {
	index := group2mappedModelIndex[group]
	if index == nil || modelName == "" {
		return nil
	}
	if channelIds, ok := index.exact[modelName]; ok {
		if filtered := filterChannelsByRequestPathAndModel(channelIds, requestPath, modelName); len(filtered) > 0 {
			return filtered
		}
	}
	for _, entry := range index.wildcard {
		if !strings.HasPrefix(modelName, entry.prefix) {
			continue
		}
		if filtered := filterChannelsByRequestPathAndModel(entry.channels, requestPath, modelName); len(filtered) > 0 {
			return filtered
		}
	}
	return nil
}

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		InvalidatePricingCache()
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	newChannel2advancedCustomConfig := make(map[int]*dto.AdvancedCustomConfig)
	var channels []*Channel
	DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
		if channel.Type == constant.ChannelTypeAdvancedCustom {
			if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
				newChannel2advancedCustomConfig[channel.Id] = config
			}
		}
	}
	var abilities []*Ability
	DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			models := strings.Split(channel.Models, ",")
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	// 以各渠道 model_mapping 的源模型建立反向索引：请求原始模型不在渠道模型列表时，
	// 仍可按映射键（含 "claude-sonnet-*" 之类的尾部通配）选到渠道。
	newGroup2mappedModelIndex := make(map[string]*channelMappedModelIndex)
	for _, channel := range channels {
		indexChannelModelMapping(newGroup2mappedModelIndex, channel)
	}
	for _, index := range newGroup2mappedModelIndex {
		index.sortChannels(newChannelId2channel)
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	group2mappedModelIndex = newGroup2mappedModelIndex
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channel2advancedCustomConfig = newChannel2advancedCustomConfig
	channelSyncLock.Unlock()
	// Lock ordering: InvalidatePricingCache acquires updatePricingLock, and
	// GetPricing (holding updatePricingLock) nests channelSyncLock.RLock via
	// loadPricingAdvancedCustomConfigs. channelSyncLock MUST be released before
	// invalidating the pricing cache, otherwise the reversed order deadlocks.
	InvalidatePricingCache()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(group string, model string, retry int, requestPath string) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return GetChannel(group, model, retry, requestPath)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := filterChannelsByRequestPathAndModel(group2model2channels[group][model], requestPath, model)

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = filterChannelsByRequestPathAndModel(group2model2channels[group][normalizedModel], requestPath, model)
	}

	// 精确与归一化都未命中时，再按渠道 model_mapping 的源模型回退查找。
	// 这样「渠道只填上游模型 + model_mapping 把 Claude 模型名指到上游」的配置
	// 也能选中渠道；后续的优先级、权重与重试逻辑保持不变。
	if len(channels) == 0 {
		channels = getMappedModelChannels(group, model, requestPath)
	}

	if len(channels) == 0 {
		return nil, nil
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	if retry >= len(uniquePriorities) {
		retry = len(uniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[retry])

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, targetPriority))
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

// filterChannelsByRequestPathAndModel restricts candidates by request path and
// model. Only Advanced Custom (type 58) channels are path-checked: they are kept
// only when one of their configured routes matches requestPath and model. All
// other channel types always pass. When requestPath is empty, filtering is skipped.
// Caller must hold channelSyncLock (read lock). The cached slice is never mutated.
func filterChannelsByRequestPathAndModel(channels []int, requestPath string, model string) []int {
	if requestPath == "" || len(channels) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelId := range channels {
		channel, ok := channelsIDM[channelId]
		if !ok {
			// keep it so the downstream consistency error is raised as before
			filtered = append(filtered, channelId)
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			filtered = append(filtered, channelId)
			continue
		}
		if config := channel2advancedCustomConfig[channelId]; config != nil && config.SupportsPathForModel(requestPath, model) {
			filtered = append(filtered, channelId)
		}
	}
	return filtered
}

// IsChannelEnabledForGroupModelOrMapping 判断渠道是否能在该分组下服务该模型：
// 既包括渠道模型列表直接命中的情况，也包括渠道 model_mapping 源模型命中的情况。
// 它让渠道亲和缓存对「只配置了映射」的渠道同样生效。
func IsChannelEnabledForGroupModelOrMapping(group string, modelName string, channelID int) bool {
	if channelID <= 0 {
		return false
	}
	if IsChannelEnabledForGroupModel(group, modelName, channelID) {
		return true
	}
	if !common.MemoryCacheEnabled || group == "" || modelName == "" {
		return false
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	return isChannelIDInList(getMappedModelChannels(group, modelName, ""), channelID)
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	// Defer before acquiring the lock so invalidation runs after unlock.
	defer InvalidatePricingCache()
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
		// 同步剔除 model_mapping 反向索引，避免禁用渠道继续被映射选路命中
		removeChannelFromMappedModelIndex(id)
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	if channel == nil {
		channelSyncLock.Unlock()
		return
	}

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	if channel2advancedCustomConfig == nil {
		channel2advancedCustomConfig = make(map[int]*dto.AdvancedCustomConfig)
	}
	delete(channel2advancedCustomConfig, channel.Id)
	if channel.Type == constant.ChannelTypeAdvancedCustom {
		if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
			channel2advancedCustomConfig[channel.Id] = config
		}
	}
	// 同步 model_mapping 反向索引：先剔除该渠道的旧条目，再按新配置重建。
	removeChannelFromMappedModelIndex(channel.Id)
	if group2mappedModelIndex == nil {
		group2mappedModelIndex = make(map[string]*channelMappedModelIndex)
	}
	indexChannelModelMapping(group2mappedModelIndex, channel)
	for _, index := range group2mappedModelIndex {
		index.sortChannels(channelsIDM)
	}
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
	// Lock ordering: do NOT hold channelSyncLock while calling
	// InvalidatePricingCache. GetPricing acquires updatePricingLock first and then
	// channelSyncLock.RLock (via loadPricingAdvancedCustomConfigs); acquiring
	// updatePricingLock while holding channelSyncLock would be an AB-BA deadlock.
	channelSyncLock.Unlock()
	InvalidatePricingCache()
}

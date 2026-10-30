package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"maps"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"gorm.io/gorm"
)

// 活动码：管理员创建，用户在有效期与限额内用它把「目标分组」加入自己的可用分组。
// 授权（ActivityGrant）在兑换时写入条款；管理员编辑活动码时会把最新条款同步到有效的
// 已发放授权（类型不可改，已用 Token、到期时间与已支付价格保持不变）。
const (
	ActivityCodeStatusEnabled  = 1
	ActivityCodeStatusDisabled = 2

	ActivityGrantStatusActive  = "active"
	ActivityGrantStatusExpired = "expired"
	ActivityGrantStatusRevoked = "revoked"

	ActivityRatioModeInherit  = "inherit"
	ActivityRatioModeOverride = "override"

	// ActivityRatioScopeGroup 默认作用范围：RatioMode/RatioOverride 作用于整个目标分组。
	ActivityRatioScopeGroup = "group"
	// ActivityRatioScopeModels 新作用范围：只对 ModelRatioOverrides 命中的模型改价，
	// 分组内其余模型按各自原价计费。
	ActivityRatioScopeModels = "models"

	ActivityGrantSourceRedeem = "redeem"
	// ActivityGrantSourceAdmin 表示管理员主动赠送，不扣余额、不占兑换次数。
	ActivityGrantSourceAdmin = "admin"

	// ActivityCodeTypeRatio 是现有的倍率优惠类型。
	ActivityCodeTypeRatio = "ratio"
	// ActivityCodeTypeBenefit 是免费权益列表类型：多分组、整组/单模型免费、可选 Token 总量。
	ActivityCodeTypeBenefit = "benefit"

	// ActivityBenefitScopeGroup 表示整组免费；ActivityBenefitScopeModel 表示只对单个模型免费。
	ActivityBenefitScopeGroup = "group"
	ActivityBenefitScopeModel = "model"

	// ActivityCodeMaxBenefits 限制一条活动码的权益条数，避免超大 JSON 落库。
	ActivityCodeMaxBenefits = 20
	// ActivityCodeMaxTokenTotal 与前端 Number 安全整数上限保持一致，避免精度丢失。
	ActivityCodeMaxTokenTotal = int64(1_000_000_000_000_000)

	// ActivityCodeMaxLength 是活动码的字符数上限（按 rune 计，中文等宽字符按 1 个算）。
	ActivityCodeMaxLength = 64
	// ActivityCodeMaxRpm 是活动码每分钟请求上限的允许最大值。
	ActivityCodeMaxRpm = 100000
	// ActivityCodeMaxPriceQuota 与用户余额列的 32 位存储保持一致。
	ActivityCodeMaxPriceQuota = math.MaxInt32
	// ActivityCodeMaxDurationSeconds 限制单次授权的有效期长度。
	ActivityCodeMaxDurationSeconds = 10 * 365 * 24 * 3600
	// ActivityCodeMaxModelRatioOverrides 是指定模型倍率的条目上限，避免超大 JSON 落库。
	ActivityCodeMaxModelRatioOverrides = 100
	// ActivityModelNameMaxLength 与 abilities.model 的 varchar(255) 保持一致。
	ActivityModelNameMaxLength = 255
)

// ActivityModelRatioOverrides 是活动码「指定模型 → 倍率」的覆盖表，落库为 JSON 对象。
// 空表写 NULL，从 NULL/空值读回时仍是空表，因此新增该列的存量数据无需迁移。
type ActivityModelRatioOverrides map[string]float64

// Scan 实现 sql.Scanner。新增列在存量行上是 NULL，必须容忍 nil 与空值。
func (m *ActivityModelRatioOverrides) Scan(value interface{}) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		*m = nil
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		*m = nil
		return nil
	}
	if len(raw) == 0 {
		*m = nil
		return nil
	}
	overrides := make(ActivityModelRatioOverrides)
	if err := common.Unmarshal(raw, &overrides); err != nil {
		return err
	}
	if len(overrides) == 0 {
		*m = nil
		return nil
	}
	*m = overrides
	return nil
}

// Value 实现 driver.Valuer：空表写 NULL，避免往 MySQL 的 json 列里写空串。
func (m ActivityModelRatioOverrides) Value() (driver.Value, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return common.Marshal(map[string]float64(m))
}

// ActivityBenefit 是「免费权益列表」类型中的一条权益：
// 目标分组 + 整组/单模型范围 + 独立 RPM + 可选 Token 总量（0 表示不限量免费）。
type ActivityBenefit struct {
	Key        string `json:"key"`
	GroupName  string `json:"group_name"`
	Scope      string `json:"scope"`
	ModelName  string `json:"model_name,omitempty"`
	RpmLimit   int    `json:"rpm_limit"`
	TokenTotal int64  `json:"token_total"`
}

// ActivityBenefits 是权益列表，落库为 JSON 数组；空列表写 NULL。
type ActivityBenefits []ActivityBenefit

// Scan 实现 sql.Scanner，兼容存量行的 NULL/空值。
func (b *ActivityBenefits) Scan(value interface{}) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		*b = nil
		return nil
	case []byte:
		if len(v) == 0 {
			*b = nil
			return nil
		}
		raw = v
	case string:
		if len(v) == 0 {
			*b = nil
			return nil
		}
		raw = []byte(v)
	default:
		*b = nil
		return nil
	}
	benefits := make(ActivityBenefits, 0)
	if err := common.Unmarshal(raw, &benefits); err != nil {
		return err
	}
	if len(benefits) == 0 {
		*b = nil
		return nil
	}
	*b = benefits
	return nil
}

// Value 实现 driver.Valuer：空列表写 NULL。
func (b ActivityBenefits) Value() (driver.Value, error) {
	if len(b) == 0 {
		return nil, nil
	}
	return common.Marshal([]ActivityBenefit(b))
}

// 兑换失败原因，由 controller 映射为 i18n 文案。
var (
	ErrActivityCodeInvalid          = errors.New("activity code invalid")
	ErrActivityCodeDisabled         = errors.New("activity code disabled")
	ErrActivityCodeNotStarted       = errors.New("activity code not started")
	ErrActivityCodeEnded            = errors.New("activity code ended")
	ErrActivityCodeSoldOut          = errors.New("activity code sold out")
	ErrActivityCodeUserLimitReached = errors.New("activity code per user limit reached")
	ErrActivityCodeGroupUnavailable = errors.New("activity code group unavailable")
	ErrActivityCodeBalanceNotEnough = errors.New("activity code wallet quota not enough")
	ErrActivityCodeNeedConfirm      = errors.New("activity code needs confirmation")
	ErrActivityCodePriceChanged     = errors.New("activity code price changed")
)

// ActivityCode 是一个可兑换的活动码。
// RatioMode 为 inherit 时使用目标分组自身的倍率；为 override 时使用 RatioOverride
// （RatioOverride 为 0 表示该分组完全免费）。
type ActivityCode struct {
	Id          int    `json:"id"`
	Code        string `json:"code" gorm:"type:varchar(64);uniqueIndex"`
	Name        string `json:"name" gorm:"type:varchar(64);index"`
	Description string `json:"description" gorm:"type:text"`
	Status      int    `json:"status" gorm:"default:1"`
	// Type 为空或 ratio 时是现有倍率类型；benefit 时使用 Benefits 列表。
	Type            string  `json:"type" gorm:"type:varchar(16);default:'ratio'"`
	GroupName       string  `json:"group_name" gorm:"type:varchar(64);index"`
	DurationSeconds int64   `json:"duration_seconds" gorm:"bigint;default:0"`
	RpmLimit        int     `json:"rpm_limit" gorm:"default:0"`
	PriceQuota      int     `json:"price_quota" gorm:"default:0"`
	RatioMode       string  `json:"ratio_mode" gorm:"type:varchar(16);default:'inherit'"`
	RatioOverride   float64 `json:"ratio_override"`
	// RatioScope 决定倍率的作用范围：group 作用于整个分组（默认），models 只作用于
	// ModelRatioOverrides 命中的模型；存量行该列为空，按 group 处理。
	RatioScope          string                      `json:"ratio_scope" gorm:"type:varchar(16);default:'group'"`
	ModelRatioOverrides ActivityModelRatioOverrides `json:"model_ratio_overrides" gorm:"type:json"`
	// Benefits 仅 benefit 类型使用；ratio 类型为空。
	Benefits      ActivityBenefits `json:"benefits" gorm:"type:json"`
	StartTime     int64            `json:"start_time" gorm:"bigint;default:0"`
	EndTime       int64            `json:"end_time" gorm:"bigint;default:0"`
	TotalLimit    int              `json:"total_limit" gorm:"default:0"`
	UsedCount     int              `json:"used_count" gorm:"default:0"`
	PerUserLimit  int              `json:"per_user_limit" gorm:"default:1"`
	PublicVisible int              `json:"public_visible" gorm:"default:0"`
	CreatedBy     int              `json:"created_by"`
	CreatedTime   int64            `json:"created_time" gorm:"bigint"`
	UpdatedTime   int64            `json:"updated_time" gorm:"bigint"`
	DeletedAt     gorm.DeletedAt   `gorm:"index"`
}

// ActivityGrant 是某个用户的一次活动授权。Status 与 ExpireTime 由本表维护，
// 读取时始终按 (status, expire_time) 过滤，因此缓存滞后不会延长授权。
type ActivityGrant struct {
	Id            int     `json:"id"`
	UserId        int     `json:"user_id" gorm:"index;index:idx_activity_grant_user_status,priority:1"`
	CodeId        int     `json:"code_id" gorm:"index"`
	Code          string  `json:"code" gorm:"type:varchar(64)"`
	CodeName      string  `json:"code_name" gorm:"type:varchar(64)"`
	GroupName     string  `json:"group_name" gorm:"type:varchar(64);index"`
	RatioMode     string  `json:"ratio_mode" gorm:"type:varchar(16);default:'inherit'"`
	RatioOverride float64 `json:"ratio_override"`
	// RatioScope 与 ModelRatioOverrides 是当前生效的活动条款；编辑活动码时同步更新。
	RatioScope          string                      `json:"ratio_scope" gorm:"type:varchar(16);default:'group'"`
	ModelRatioOverrides ActivityModelRatioOverrides `json:"model_ratio_overrides" gorm:"type:json"`
	RpmLimit            int                         `json:"rpm_limit" gorm:"default:0"`
	PriceQuota          int                         `json:"price_quota" gorm:"default:0"`
	// RedeemBatch 把同一次兑换生成的多条权益授权归为一组；存量为空字符串。
	RedeemBatch string `json:"redeem_batch" gorm:"type:varchar(64);index"`
	// BenefitKey 对应活动码权益的稳定 key；ratio 类型授权为空。
	BenefitKey string `json:"benefit_key" gorm:"type:varchar(32);index"`
	// TokenTotal/TokenUsed 仅 Token 套餐权益使用；0 表示不限量免费。
	TokenTotal  int64  `json:"token_total" gorm:"bigint;default:0"`
	TokenUsed   int64  `json:"token_used" gorm:"bigint;default:0"`
	StartTime   int64  `json:"start_time" gorm:"bigint"`
	ExpireTime  int64  `json:"expire_time" gorm:"bigint;index"`
	Status      string `json:"status" gorm:"type:varchar(32);index;index:idx_activity_grant_user_status,priority:2"`
	Source      string `json:"source" gorm:"type:varchar(32);default:'redeem'"`
	Remark      string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func (activityCode *ActivityCode) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	activityCode.CreatedTime = now
	activityCode.UpdatedTime = now
	return nil
}

func (activityCode *ActivityCode) BeforeUpdate(tx *gorm.DB) error {
	activityCode.UpdatedTime = common.GetTimestamp()
	return nil
}

func (grant *ActivityGrant) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	grant.CreatedTime = now
	grant.UpdatedTime = now
	return nil
}

func (grant *ActivityGrant) BeforeUpdate(tx *gorm.DB) error {
	grant.UpdatedTime = common.GetTimestamp()
	return nil
}

// NormalizeActivityCode 归一化用户输入的活动码：去首尾空白并转小写。
// 统一以小写形式落库，避免不同数据库大小写敏感度不一致；中文等非拉丁字符不受影响。
func NormalizeActivityCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

// IsValidActivityCode 校验活动码格式：允许任意语言的字母与数字，以及下划线和连字符。
// 不接受空格、标点、表情符号；长度按字符（rune）计算，中文一个汉字算一个字符。
func IsValidActivityCode(code string) bool {
	if code == "" || utf8.RuneCountInString(code) > ActivityCodeMaxLength {
		return false
	}
	for _, r := range code {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// NormalizeActivityCodeType 归一化活动码类型：空值按 ratio 兼容存量数据；非法值返回空串。
func NormalizeActivityCodeType(codeType string) string {
	switch strings.ToLower(strings.TrimSpace(codeType)) {
	case "", ActivityCodeTypeRatio:
		return ActivityCodeTypeRatio
	case ActivityCodeTypeBenefit:
		return ActivityCodeTypeBenefit
	default:
		return ""
	}
}

// NewActivityBenefitKey 生成权益的稳定 key，用于编辑时把新权益同步到已发放授权。
func NewActivityBenefitKey() string {
	return strings.ReplaceAll(common.GetUUID(), "-", "")
}

// IsValidActivityBenefitKey 校验权益 key 的格式（1-32 位小写字母/数字）。
func IsValidActivityBenefitKey(key string) bool {
	if key == "" || len(key) > 32 {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// IsBenefitType 判断活动码是否为免费权益列表类型。
func (activityCode *ActivityCode) IsBenefitType() bool {
	return NormalizeActivityCodeType(activityCode.Type) == ActivityCodeTypeBenefit
}

// TargetGroups 返回活动码覆盖的分组（去重，保持顺序）。
func (activityCode *ActivityCode) TargetGroups() []string {
	if !activityCode.IsBenefitType() {
		if activityCode.GroupName == "" {
			return nil
		}
		return []string{activityCode.GroupName}
	}
	groups := make([]string, 0, len(activityCode.Benefits))
	seen := make(map[string]struct{}, len(activityCode.Benefits))
	for _, benefit := range activityCode.Benefits {
		if _, exists := seen[benefit.GroupName]; exists {
			continue
		}
		seen[benefit.GroupName] = struct{}{}
		groups = append(groups, benefit.GroupName)
	}
	return groups
}

// AllGroupsAvailable 判断活动码的所有目标分组当前是否仍配置在分组倍率中。
func (activityCode *ActivityCode) AllGroupsAvailable() bool {
	for _, group := range activityCode.TargetGroups() {
		if !ratio_setting.ContainsGroupRatio(group) {
			return false
		}
	}
	return true
}

// activityGrantSpec 是把活动码权益展开成单条授权所需的配置。
type activityGrantSpec struct {
	GroupName           string
	RatioMode           string
	RatioOverride       float64
	RatioScope          string
	ModelRatioOverrides ActivityModelRatioOverrides
	RpmLimit            int
	TokenTotal          int64
	BenefitKey          string
}

func activityGrantSpecForBenefit(benefit ActivityBenefit) activityGrantSpec {
	spec := activityGrantSpec{
		GroupName:  benefit.GroupName,
		RatioMode:  ActivityRatioModeInherit,
		RatioScope: ActivityRatioScopeGroup,
		RpmLimit:   benefit.RpmLimit,
		TokenTotal: benefit.TokenTotal,
		BenefitKey: benefit.Key,
	}
	if benefit.Scope == ActivityBenefitScopeModel {
		spec.RatioScope = ActivityRatioScopeModels
		spec.ModelRatioOverrides = ActivityModelRatioOverrides{benefit.ModelName: 0}
		return spec
	}
	spec.RatioMode = ActivityRatioModeOverride
	spec.RatioOverride = 0
	return spec
}

// grantSpecs 把活动码展开成一条或多条授权规格。
func (activityCode *ActivityCode) grantSpecs() []activityGrantSpec {
	if !activityCode.IsBenefitType() {
		return []activityGrantSpec{{
			GroupName:           activityCode.GroupName,
			RatioMode:           activityCode.RatioMode,
			RatioOverride:       activityCode.RatioOverride,
			RatioScope:          activityCode.RatioScope,
			ModelRatioOverrides: maps.Clone(activityCode.ModelRatioOverrides),
			RpmLimit:            activityCode.RpmLimit,
		}}
	}
	specs := make([]activityGrantSpec, 0, len(activityCode.Benefits))
	for _, benefit := range activityCode.Benefits {
		specs = append(specs, activityGrantSpecForBenefit(benefit))
	}
	return specs
}

func (spec activityGrantSpec) buildGrant(userId int, activityCode *ActivityCode, redeemBatch string) ActivityGrant {
	return ActivityGrant{
		UserId:              userId,
		CodeId:              activityCode.Id,
		Code:                activityCode.Code,
		CodeName:            activityCode.Name,
		GroupName:           spec.GroupName,
		RatioMode:           spec.RatioMode,
		RatioOverride:       spec.RatioOverride,
		RatioScope:          spec.RatioScope,
		ModelRatioOverrides: maps.Clone(spec.ModelRatioOverrides),
		RpmLimit:            spec.RpmLimit,
		TokenTotal:          spec.TokenTotal,
		BenefitKey:          spec.BenefitKey,
		RedeemBatch:         redeemBatch,
		Status:              ActivityGrantStatusActive,
	}
}

// CountActivityRedemptions 按兑换批次统计次数：同一次兑换生成的多条权益授权只算一次；
// 历史授权没有 redeem_batch，每条算一次。
func CountActivityRedemptions(grants []ActivityGrant) int {
	batches := make(map[string]struct{}, len(grants))
	legacy := 0
	for _, grant := range grants {
		if grant.RedeemBatch == "" {
			legacy++
			continue
		}
		batches[grant.RedeemBatch] = struct{}{}
	}
	return len(batches) + legacy
}

func sortedUserIds(users map[int]struct{}) []int {
	ids := make([]int, 0, len(users))
	for userId := range users {
		ids = append(ids, userId)
	}
	sort.Ints(ids)
	return ids
}

// WindowClosed 判断活动码的可兑换窗口是否已经结束。
func (activityCode *ActivityCode) WindowClosed(now int64) bool {
	return activityCode.EndTime != 0 && now > activityCode.EndTime
}

// RemainingTotal 返回该活动码剩余可兑换次数；返回 -1 表示不限次数。
func (activityCode *ActivityCode) RemainingTotal() int {
	if activityCode.TotalLimit <= 0 {
		return -1
	}
	remaining := activityCode.TotalLimit - activityCode.UsedCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ---------------------------------------------------------------------------
// 活动码管理
// ---------------------------------------------------------------------------

// GetActivityCodes 按关键字与状态分页查询活动码。status 支持 ""、"1"、"2"、"expired"。
func GetActivityCodes(keyword string, status string, startIdx int, num int) (activityCodes []*ActivityCode, total int64, err error) {
	query := DB.Model(&ActivityCode{})
	if strings.TrimSpace(keyword) != "" {
		like := strings.TrimSpace(keyword) + "%"
		query = query.Where("code LIKE ? OR name LIKE ?", like, like)
	}
	switch status {
	case "1":
		query = query.Where("status = ?", ActivityCodeStatusEnabled)
	case "2":
		query = query.Where("status = ?", ActivityCodeStatusDisabled)
	case "expired":
		query = query.Where("end_time != 0 AND end_time < ?", common.GetTimestamp())
	}
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&activityCodes).Error
	return activityCodes, total, err
}

// GetEnabledActivityCodes 返回所有启用中的活动码，用于活动中心列表与开关判定。
func GetEnabledActivityCodes() ([]*ActivityCode, error) {
	activityCodes := make([]*ActivityCode, 0)
	err := DB.Where("status = ?", ActivityCodeStatusEnabled).Order("id desc").Find(&activityCodes).Error
	return activityCodes, err
}

// GetActivityCodeById 读取未删除的活动码。
func GetActivityCodeById(id int) (*ActivityCode, error) {
	if id <= 0 {
		return nil, errors.New("id 为空！")
	}
	activityCode := ActivityCode{Id: id}
	if err := DB.First(&activityCode, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &activityCode, nil
}

// GetActivityCodeByCode 按活动码字符串读取。includeDeleted 为真时包含软删除记录，
// 用于创建前的唯一性检查：软删除记录仍然占用唯一索引。
func GetActivityCodeByCode(code string, includeDeleted bool) (*ActivityCode, error) {
	normalized := NormalizeActivityCode(code)
	if normalized == "" {
		return nil, ErrActivityCodeInvalid
	}
	activityCode := ActivityCode{}
	query := DB
	if includeDeleted {
		query = DB.Unscoped()
	}
	if err := query.Where("code = ?", normalized).First(&activityCode).Error; err != nil {
		return nil, err
	}
	return &activityCode, nil
}

func (activityCode *ActivityCode) Insert() error {
	return DB.Create(activityCode).Error
}

// Update 只写活动码的可编辑字段，并把最新条款同步到有效的已发放授权。
// used_count 由兑换流程的 CAS 维护，code 一经创建不可修改（需要换码请新建）。
func (activityCode *ActivityCode) Update() error {
	affectedUserIds := make([]int, 0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 与并发兑换/赠送串行化：写授权前先锁活动码行。
		locked := &ActivityCode{}
		if err := lockForUpdate(tx).First(locked, "id = ?", activityCode.Id).Error; err != nil {
			return err
		}
		activityCode.UpdatedTime = common.GetTimestamp()
		if err := tx.Model(activityCode).
			Select("name", "description", "status", "group_name", "duration_seconds", "rpm_limit",
				"price_quota", "ratio_mode", "ratio_override", "ratio_scope", "model_ratio_overrides",
				"type", "benefits", "start_time", "end_time", "total_limit", "per_user_limit",
				"public_visible", "updated_time").
			Updates(activityCode).Error; err != nil {
			return err
		}
		ids, syncErr := syncActivityGrantsTx(tx, activityCode)
		if syncErr != nil {
			return syncErr
		}
		affectedUserIds = ids
		return nil
	})
	if err != nil {
		return err
	}
	for _, userId := range affectedUserIds {
		if cacheErr := InvalidateActivityGrantCache(userId); cacheErr != nil {
			common.SysError("failed to invalidate activity grant cache after sync: " + cacheErr.Error())
		}
	}
	return nil
}

// syncActivityGrantsTx 把活动码的最新条款同步到有效（active 且未过期）的授权。
// 已过期/已撤销授权不动；已用 Token、到期时间与已支付价格保持不变。
func syncActivityGrantsTx(tx *gorm.DB, activityCode *ActivityCode) ([]int, error) {
	if NormalizeActivityCodeType(activityCode.Type) != ActivityCodeTypeBenefit {
		return syncRatioActivityGrantsTx(tx, activityCode)
	}
	return syncBenefitActivityGrantsTx(tx, activityCode)
}

func loadActiveActivityGrantsForCode(tx *gorm.DB, codeId int, now int64) ([]ActivityGrant, error) {
	grants := make([]ActivityGrant, 0)
	err := tx.Where("code_id = ? AND status = ? AND (expire_time = 0 OR expire_time > ?)",
		codeId, ActivityGrantStatusActive, now).
		Order("id asc").
		Find(&grants).Error
	return grants, err
}

func syncRatioActivityGrantsTx(tx *gorm.DB, activityCode *ActivityCode) ([]int, error) {
	now := common.GetTimestamp()
	grants, err := loadActiveActivityGrantsForCode(tx, activityCode.Id, now)
	if err != nil {
		return nil, err
	}
	if len(grants) == 0 {
		return nil, nil
	}
	affected := make(map[int]struct{}, len(grants))
	for i := range grants {
		grant := &grants[i]
		affected[grant.UserId] = struct{}{}
		result := tx.Model(&ActivityGrant{}).Where("id = ?", grant.Id).
			Updates(map[string]interface{}{
				"code_name":             activityCode.Name,
				"group_name":            activityCode.GroupName,
				"ratio_mode":            activityCode.RatioMode,
				"ratio_override":        activityCode.RatioOverride,
				"ratio_scope":           activityCode.RatioScope,
				"model_ratio_overrides": activityCode.ModelRatioOverrides,
				"rpm_limit":             activityCode.RpmLimit,
				"updated_time":          now,
			})
		if result.Error != nil {
			return nil, result.Error
		}
	}
	return sortedUserIds(affected), nil
}

type activityRedeemBatchKey struct {
	UserId int
	Batch  string
}

type activityRedeemBatchTemplate struct {
	Grant         ActivityGrant
	MaxExpireTime int64
	HasPermanent  bool
}

func syncBenefitActivityGrantsTx(tx *gorm.DB, activityCode *ActivityCode) ([]int, error) {
	now := common.GetTimestamp()
	activeGrants, err := loadActiveActivityGrantsForCode(tx, activityCode.Id, now)
	if err != nil {
		return nil, err
	}
	if len(activeGrants) == 0 {
		return nil, nil
	}

	benefitByKey := make(map[string]ActivityBenefit, len(activityCode.Benefits))
	for _, benefit := range activityCode.Benefits {
		benefitByKey[benefit.Key] = benefit
	}

	// 记录每个兑换批次曾经发放过的权益 key（含已撤销/已过期），
	// 避免管理员手动撤销的权益在下次编辑时被补回来。
	issuedKeys := make(map[activityRedeemBatchKey]map[string]struct{})
	allGrants := make([]ActivityGrant, 0)
	if err := tx.Where("code_id = ?", activityCode.Id).Find(&allGrants).Error; err != nil {
		return nil, err
	}
	for i := range allGrants {
		grant := &allGrants[i]
		if grant.RedeemBatch == "" || grant.BenefitKey == "" {
			continue
		}
		key := activityRedeemBatchKey{UserId: grant.UserId, Batch: grant.RedeemBatch}
		if issuedKeys[key] == nil {
			issuedKeys[key] = make(map[string]struct{})
		}
		issuedKeys[key][grant.BenefitKey] = struct{}{}
	}

	affected := make(map[int]struct{}, len(activeGrants))
	templates := make(map[activityRedeemBatchKey]*activityRedeemBatchTemplate)
	for i := range activeGrants {
		grant := &activeGrants[i]
		affected[grant.UserId] = struct{}{}
		if grant.RedeemBatch == "" || grant.BenefitKey == "" {
			// benefit 类型正常不会出现空批次/key；保留原样，避免误删历史数据。
			continue
		}
		batchKey := activityRedeemBatchKey{UserId: grant.UserId, Batch: grant.RedeemBatch}
		template := templates[batchKey]
		if template == nil {
			template = &activityRedeemBatchTemplate{Grant: *grant}
			templates[batchKey] = template
		}
		if grant.ExpireTime == 0 {
			template.HasPermanent = true
		} else if grant.ExpireTime > template.MaxExpireTime {
			template.MaxExpireTime = grant.ExpireTime
		}

		benefit, exists := benefitByKey[grant.BenefitKey]
		if !exists {
			if result := tx.Model(&ActivityGrant{}).Where("id = ?", grant.Id).
				Updates(map[string]interface{}{
					"status":       ActivityGrantStatusRevoked,
					"updated_time": now,
				}); result.Error != nil {
				return nil, result.Error
			}
			continue
		}
		spec := activityGrantSpecForBenefit(benefit)
		if result := tx.Model(&ActivityGrant{}).Where("id = ?", grant.Id).
			Updates(map[string]interface{}{
				"code_name":             activityCode.Name,
				"group_name":            spec.GroupName,
				"ratio_mode":            spec.RatioMode,
				"ratio_override":        spec.RatioOverride,
				"ratio_scope":           spec.RatioScope,
				"model_ratio_overrides": spec.ModelRatioOverrides,
				"rpm_limit":             spec.RpmLimit,
				"token_total":           spec.TokenTotal,
				"updated_time":          now,
			}); result.Error != nil {
			return nil, result.Error
		}
	}

	// 新增权益：为每个仍有有效授权的兑换批次补一条授权。
	for batchKey, template := range templates {
		expireTime := template.MaxExpireTime
		if template.HasPermanent {
			expireTime = 0
		}
		issued := issuedKeys[batchKey]
		for _, benefit := range activityCode.Benefits {
			if _, exists := issued[benefit.Key]; exists {
				continue
			}
			grant := activityGrantSpecForBenefit(benefit).buildGrant(batchKey.UserId, activityCode, batchKey.Batch)
			grant.StartTime = template.Grant.StartTime
			grant.ExpireTime = expireTime
			grant.Source = template.Grant.Source
			grant.PriceQuota = template.Grant.PriceQuota
			grant.Remark = template.Grant.Remark
			if createErr := tx.Create(&grant).Error; createErr != nil {
				return nil, createErr
			}
		}
	}
	return sortedUserIds(affected), nil
}

func (activityCode *ActivityCode) Delete() error {
	return DB.Delete(activityCode).Error
}

// DeleteActivityCodeById 软删除活动码。已发放的授权不受影响。
func DeleteActivityCodeById(id int) error {
	activityCode, err := GetActivityCodeById(id)
	if err != nil {
		return err
	}
	return activityCode.Delete()
}

// UpdateActivityCodeStatus 单独切换启用状态。
func UpdateActivityCodeStatus(id int, status int) error {
	if status != ActivityCodeStatusEnabled && status != ActivityCodeStatusDisabled {
		return errors.New("无效的活动码状态")
	}
	result := DB.Model(&ActivityCode{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       status,
			"updated_time": common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// 授权读取
// ---------------------------------------------------------------------------

// filterActiveActivityGrants 在读时再次过滤：缓存中的记录同样必须满足
// 「状态为 active 且未到期」，因此缓存过期不会延长任何授权。
func filterActiveActivityGrants(grants []ActivityGrant) []ActivityGrant {
	now := common.GetTimestamp()
	active := make([]ActivityGrant, 0, len(grants))
	for _, grant := range grants {
		if grant.Status != ActivityGrantStatusActive {
			continue
		}
		if grant.ExpireTime != 0 && grant.ExpireTime <= now {
			continue
		}
		active = append(active, grant)
	}
	return active
}

func queryActiveActivityGrants(tx *gorm.DB, userId int) ([]ActivityGrant, error) {
	grants := make([]ActivityGrant, 0)
	err := tx.Where("user_id = ? AND status = ? AND (expire_time = 0 OR expire_time > ?)",
		userId, ActivityGrantStatusActive, common.GetTimestamp()).
		Order("id desc").
		Find(&grants).Error
	if err != nil {
		return nil, err
	}
	return grants, nil
}

// refreshActivityGrantTokenUsage 从数据库刷新 Token 套餐授权的已用量。
// 授权定义可以缓存，但 token_used 必须接近实时，否则同一缓存窗口内会重复免费。
func refreshActivityGrantTokenUsage(userId int, grants []ActivityGrant) []ActivityGrant {
	hasTokenPackage := false
	for i := range grants {
		if grants[i].TokenTotal > 0 {
			hasTokenPackage = true
			break
		}
	}
	if !hasTokenPackage {
		return grants
	}
	type tokenUsageRow struct {
		Id        int
		TokenUsed int64
	}
	rows := make([]tokenUsageRow, 0, len(grants))
	// 按 user_id 查询而不是拼 IN 列表，避免授权数量很多时触发 SQL 参数上限。
	if err := DB.Model(&ActivityGrant{}).Select("id", "token_used").
		Where("user_id = ? AND token_total > 0", userId).Find(&rows).Error; err != nil {
		common.SysError("failed to refresh activity grant token usage: " + err.Error())
		return grants
	}
	usageByGrantId := make(map[int]int64, len(rows))
	for _, row := range rows {
		usageByGrantId[row.Id] = row.TokenUsed
	}
	for i := range grants {
		if used, exists := usageByGrantId[grants[i].Id]; exists {
			grants[i].TokenUsed = used
		}
	}
	return grants
}

// GetActiveActivityGrants 返回用户当前有效的活动授权，结果带 Redis 缓存
// （缓存只加速，到期判定始终以读时过滤为准；Token 已用量每次从数据库刷新）。
func GetActiveActivityGrants(userId int) ([]ActivityGrant, error) {
	if userId <= 0 {
		return []ActivityGrant{}, nil
	}
	if common.RedisEnabled {
		cached, err := common.RedisGet(activityGrantCacheKey(userId))
		if err == nil {
			if strings.TrimSpace(cached) == "" {
				return []ActivityGrant{}, nil
			}
			grants := make([]ActivityGrant, 0)
			if unmarshalErr := common.Unmarshal([]byte(cached), &grants); unmarshalErr != nil {
				common.SysLog("failed to unmarshal activity grants cache: " + unmarshalErr.Error())
			} else {
				return filterActiveActivityGrants(refreshActivityGrantTokenUsage(userId, grants)), nil
			}
		}
	}
	grants, err := queryActiveActivityGrants(DB, userId)
	if err != nil {
		return nil, err
	}
	grants = refreshActivityGrantTokenUsage(userId, grants)
	if common.RedisEnabled {
		if payload, marshalErr := common.Marshal(grants); marshalErr == nil {
			ttl := common.RedisKeyCacheSeconds()
			if ttl <= 0 {
				ttl = 60
			}
			if cacheErr := common.RedisSet(activityGrantCacheKey(userId), string(payload), time.Duration(ttl)*time.Second); cacheErr != nil {
				common.SysLog("failed to cache activity grants: " + cacheErr.Error())
			}
		}
	}
	return grants, nil
}

func activityGrantCacheKey(userId int) string {
	return fmt.Sprintf("activity:grants:%d", userId)
}

// InvalidateActivityGrantCache 清除用户的活动授权缓存（兑换、撤销后调用）。
func InvalidateActivityGrantCache(userId int) error {
	if !common.RedisEnabled || userId <= 0 {
		return nil
	}
	return common.RedisDelKey(activityGrantCacheKey(userId))
}

// ListActivityGrantsByCode 管理员视角：某活动码的授权记录。
func ListActivityGrantsByCode(codeId int, startIdx int, num int) (grants []*ActivityGrant, total int64, err error) {
	query := DB.Model(&ActivityGrant{}).Where("code_id = ?", codeId)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&grants).Error
	return grants, total, err
}

// ListActivityGrantsByUser 用户视角：自己的授权记录（包含已过期与已撤销）。
func ListActivityGrantsByUser(userId int) ([]ActivityGrant, error) {
	grants := make([]ActivityGrant, 0)
	err := DB.Where("user_id = ?", userId).Order("id desc").Limit(100).Find(&grants).Error
	return grants, err
}

func countActivityRedemptionsForUserCode(tx *gorm.DB, userId int, codeId int) (int64, error) {
	grants := make([]ActivityGrant, 0)
	err := tx.Model(&ActivityGrant{}).Select("id", "redeem_batch").
		Where("user_id = ? AND code_id = ?", userId, codeId).
		Find(&grants).Error
	if err != nil {
		return 0, err
	}
	return int64(CountActivityRedemptions(grants)), nil
}

// AddActivityGrantTokenUsage 原子累加一次请求消耗的 token 到对应 Token 套餐授权。
// 非套餐授权（token_total=0）或已删除授权的更新会被忽略。
func AddActivityGrantTokenUsage(grantId int, tokens int64) error {
	if grantId <= 0 || tokens <= 0 {
		return nil
	}
	if tokens > ActivityCodeMaxTokenTotal {
		tokens = ActivityCodeMaxTokenTotal
	}
	result := DB.Model(&ActivityGrant{}).Where("id = ? AND token_total > 0", grantId).
		Updates(map[string]interface{}{
			"token_used":   gorm.Expr("token_used + ?", tokens),
			"updated_time": common.GetTimestamp(),
		})
	return result.Error
}

// RevokeActivityGrant 撤销一张授权，撤销后清缓存，下一次请求即不可用。
func RevokeActivityGrant(id int, remark string) error {
	if id <= 0 {
		return errors.New("id 为空！")
	}
	grant := ActivityGrant{}
	if err := DB.First(&grant, "id = ?", id).Error; err != nil {
		return err
	}
	if grant.Status != ActivityGrantStatusActive {
		return errors.New("该授权已失效")
	}
	result := DB.Model(&ActivityGrant{}).
		Where("id = ? AND status = ?", id, ActivityGrantStatusActive).
		Updates(map[string]interface{}{
			"status":       ActivityGrantStatusRevoked,
			"remark":       remark,
			"updated_time": common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("该授权已失效")
	}
	return InvalidateActivityGrantCache(grant.UserId)
}

// ExpireDueActivityGrants 把已到期的授权标记为 expired，让管理端列表状态准确；
// 实时判定仍靠读时的 expire_time 过滤。
func ExpireDueActivityGrants() (int64, error) {
	result := DB.Model(&ActivityGrant{}).
		Where("status = ? AND expire_time != 0 AND expire_time <= ?", ActivityGrantStatusActive, common.GetTimestamp()).
		Update("status", ActivityGrantStatusExpired)
	return result.RowsAffected, result.Error
}

// ---------------------------------------------------------------------------
// 兑换
// ---------------------------------------------------------------------------

type RedeemActivityCodeRequest struct {
	Code               string
	UserId             int
	Confirm            bool
	ExpectedPriceQuota int
}

type RedeemActivityCodeResult struct {
	GrantId    int      `json:"grant_id"`
	CodeId     int      `json:"code_id"`
	Code       string   `json:"code"`
	CodeName   string   `json:"code_name"`
	GroupName  string   `json:"group_name"`
	GroupNames []string `json:"group_names"`
	ExpireTime int64    `json:"expire_time"`
	// PriceQuota 是本次兑换实扣的余额（所有权益共用一次扣费）。
	PriceQuota   int `json:"price_quota"`
	BalanceAfter int `json:"balance_after"`
}

// resolveActivityGrantExpireTime 计算新授权的到期时间：在现存活授权的最晚到期时间上叠加
// 时长（同一活动码多次兑换按续期处理），已有永久授权时保持永久。返回值 0 表示永久。
func resolveActivityGrantExpireTime(tx *gorm.DB, userId int, activityCode *ActivityCode, now int64) (int64, error) {
	if activityCode.DurationSeconds <= 0 {
		return 0, nil
	}
	existing := make([]ActivityGrant, 0)
	err := tx.Model(&ActivityGrant{}).
		Where("user_id = ? AND code_id = ? AND status = ?", userId, activityCode.Id, ActivityGrantStatusActive).
		Select("id", "expire_time").
		Find(&existing).Error
	if err != nil {
		return 0, err
	}
	base := now
	for _, grant := range existing {
		if grant.ExpireTime == 0 {
			return 0, nil
		}
		if grant.ExpireTime > base {
			base = grant.ExpireTime
		}
	}
	expireTime := base + activityCode.DurationSeconds
	maxExpireTime := now + int64(ActivityCodeMaxDurationSeconds)
	if expireTime > maxExpireTime {
		expireTime = maxExpireTime
	}
	return expireTime, nil
}

// RedeemActivityCode 兑换活动码。整个流程在一个事务里完成：
// 校验活动码 → CAS 递增已兑换次数 → 扣真余额 → 写入授权。
//
// 安全性要点：
//   - used_count 的 CAS 无条件执行，既判定总量上限，也充当同一活动码并发兑换的闸门
//     （SQLite 没有行锁，CAS 同样保证只有一次成功）。
//   - 扣费只针对 users.quota（真余额），赠金（user_bonus_grants）不参与；
//     扣费语句自带 quota >= price 条件，不足时回滚且不产生任何副作用。
func RedeemActivityCode(req RedeemActivityCodeRequest) (*RedeemActivityCodeResult, error) {
	if req.UserId <= 0 {
		return nil, ErrActivityCodeInvalid
	}
	code := NormalizeActivityCode(req.Code)
	if code == "" {
		return nil, ErrActivityCodeInvalid
	}
	now := common.GetTimestamp()
	result := &RedeemActivityCodeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		activityCode := &ActivityCode{}
		if err := lockForUpdate(tx).Where("code = ?", code).First(activityCode).Error; err != nil {
			return ErrActivityCodeInvalid
		}
		if activityCode.Status != ActivityCodeStatusEnabled {
			return ErrActivityCodeDisabled
		}
		if activityCode.StartTime != 0 && now < activityCode.StartTime {
			return ErrActivityCodeNotStarted
		}
		if activityCode.WindowClosed(now) {
			return ErrActivityCodeEnded
		}
		if !activityCode.AllGroupsAvailable() {
			return ErrActivityCodeGroupUnavailable
		}
		if activityCode.PriceQuota > 0 {
			if !req.Confirm {
				return ErrActivityCodeNeedConfirm
			}
			if req.ExpectedPriceQuota != activityCode.PriceQuota {
				return ErrActivityCodePriceChanged
			}
		}
		if activityCode.PerUserLimit > 0 {
			count, countErr := countActivityRedemptionsForUserCode(tx, req.UserId, activityCode.Id)
			if countErr != nil {
				return countErr
			}
			if count >= int64(activityCode.PerUserLimit) {
				return ErrActivityCodeUserLimitReached
			}
		}

		casResult := tx.Model(&ActivityCode{}).
			Where("id = ? AND used_count = ? AND (total_limit = 0 OR used_count < total_limit)",
				activityCode.Id, activityCode.UsedCount).
			Updates(map[string]interface{}{
				"used_count":   gorm.Expr("used_count + 1"),
				"updated_time": now,
			})
		if casResult.Error != nil {
			return casResult.Error
		}
		if casResult.RowsAffected == 0 {
			return ErrActivityCodeSoldOut
		}

		chargedQuota := 0
		if activityCode.PriceQuota > 0 {
			debit := tx.Model(&User{}).
				Where("id = ? AND quota >= ?", req.UserId, activityCode.PriceQuota).
				Update("quota", gorm.Expr("quota - ?", activityCode.PriceQuota))
			if debit.Error != nil {
				return debit.Error
			}
			if debit.RowsAffected == 0 {
				return ErrActivityCodeBalanceNotEnough
			}
			chargedQuota = activityCode.PriceQuota
		}

		expireTime, expireErr := resolveActivityGrantExpireTime(tx, req.UserId, activityCode, now)
		if expireErr != nil {
			return expireErr
		}
		specs := activityCode.grantSpecs()
		if len(specs) == 0 {
			return ErrActivityCodeGroupUnavailable
		}
		redeemBatch := common.GetUUID()
		groupNames := make([]string, 0, len(specs))
		seenGroups := make(map[string]struct{}, len(specs))
		for _, spec := range specs {
			grant := spec.buildGrant(req.UserId, activityCode, redeemBatch)
			grant.PriceQuota = chargedQuota
			grant.StartTime = now
			grant.ExpireTime = expireTime
			grant.Source = ActivityGrantSourceRedeem
			if err := tx.Create(&grant).Error; err != nil {
				return err
			}
			if result.GrantId == 0 {
				result.GrantId = grant.Id
			}
			if _, exists := seenGroups[spec.GroupName]; exists {
				continue
			}
			seenGroups[spec.GroupName] = struct{}{}
			groupNames = append(groupNames, spec.GroupName)
		}
		result.CodeId = activityCode.Id
		result.Code = activityCode.Code
		result.CodeName = activityCode.Name
		result.GroupNames = groupNames
		result.GroupName = groupNames[0]
		result.ExpireTime = expireTime
		result.PriceQuota = chargedQuota
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务已提交：同步缓存与流水。
	if err := InvalidateActivityGrantCache(req.UserId); err != nil {
		common.SysError("failed to invalidate activity grant cache: " + err.Error())
	}
	if result.PriceQuota > 0 {
		SyncDebitUserQuotaCache(req.UserId, result.PriceQuota, "activity redeem")
	}
	if quota, err := GetUserQuota(req.UserId, true); err == nil {
		result.BalanceAfter = quota
	} else {
		common.SysError("failed to read user quota after activity redeem: " + err.Error())
	}
	expireText := "永久有效"
	if result.ExpireTime > 0 {
		expireText = "有效期至 " + time.Unix(result.ExpireTime, 0).Format("2006-01-02 15:04:05")
	}
	RecordLog(req.UserId, LogTypeSystem, fmt.Sprintf("通过活动码 %s 兑换分组 %s，消耗 %s，%s",
		result.Code, strings.Join(result.GroupNames, "、"), logger.LogQuota(result.PriceQuota), expireText))
	return result, nil
}

// ---------------------------------------------------------------------------
// 管理员赠送
// ---------------------------------------------------------------------------

type GrantActivityCodeRequest struct {
	CodeId int
	UserId int
	Remark string
}

type GrantActivityCodeResult struct {
	GrantId    int      `json:"grant_id"`
	CodeId     int      `json:"code_id"`
	Code       string   `json:"code"`
	CodeName   string   `json:"code_name"`
	GroupName  string   `json:"group_name"`
	GroupNames []string `json:"group_names"`
	ExpireTime int64    `json:"expire_time"`
}

// GrantActivityCode 由管理员主动把活动码授权赠送给用户，无需用户确认。
//
// 与兑换的区别：
//   - 不扣用户余额（PriceQuota 记为 0），也不要求支付合规确认；
//   - 不占 used_count，不校验总量与每人上限——管理员赠送是运营动作，
//     不受面向用户兑换的名额约束；
//   - 授权来源记为 admin，便于与用户自行兑换区分。
//
// 仍然要求活动码存在、启用、处于可赠送时间窗口内，且目标分组仍然可用；
// 同一活动码重复赠送按续期处理，到期时间在现存活授权之后叠加。
func GrantActivityCode(req GrantActivityCodeRequest) (*GrantActivityCodeResult, error) {
	if req.UserId <= 0 {
		return nil, ErrActivityCodeInvalid
	}
	if req.CodeId <= 0 {
		return nil, ErrActivityCodeInvalid
	}
	now := common.GetTimestamp()
	result := &GrantActivityCodeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		activityCode := &ActivityCode{}
		if err := lockForUpdate(tx).First(activityCode, "id = ?", req.CodeId).Error; err != nil {
			return err
		}
		if activityCode.Status != ActivityCodeStatusEnabled {
			return ErrActivityCodeDisabled
		}
		if activityCode.StartTime != 0 && now < activityCode.StartTime {
			return ErrActivityCodeNotStarted
		}
		if activityCode.WindowClosed(now) {
			return ErrActivityCodeEnded
		}
		if !activityCode.AllGroupsAvailable() {
			return ErrActivityCodeGroupUnavailable
		}

		expireTime, expireErr := resolveActivityGrantExpireTime(tx, req.UserId, activityCode, now)
		if expireErr != nil {
			return expireErr
		}
		specs := activityCode.grantSpecs()
		if len(specs) == 0 {
			return ErrActivityCodeGroupUnavailable
		}
		redeemBatch := common.GetUUID()
		groupNames := make([]string, 0, len(specs))
		seenGroups := make(map[string]struct{}, len(specs))
		for _, spec := range specs {
			grant := spec.buildGrant(req.UserId, activityCode, redeemBatch)
			grant.StartTime = now
			grant.ExpireTime = expireTime
			grant.Source = ActivityGrantSourceAdmin
			grant.Remark = req.Remark
			if err := tx.Create(&grant).Error; err != nil {
				return err
			}
			if result.GrantId == 0 {
				result.GrantId = grant.Id
			}
			if _, exists := seenGroups[spec.GroupName]; exists {
				continue
			}
			seenGroups[spec.GroupName] = struct{}{}
			groupNames = append(groupNames, spec.GroupName)
		}
		result.CodeId = activityCode.Id
		result.Code = activityCode.Code
		result.CodeName = activityCode.Name
		result.GroupNames = groupNames
		result.GroupName = groupNames[0]
		result.ExpireTime = expireTime
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := InvalidateActivityGrantCache(req.UserId); err != nil {
		common.SysError("failed to invalidate activity grant cache: " + err.Error())
	}
	expireText := "永久有效"
	if result.ExpireTime > 0 {
		expireText = "有效期至 " + time.Unix(result.ExpireTime, 0).Format("2006-01-02 15:04:05")
	}
	RecordLog(req.UserId, LogTypeSystem, fmt.Sprintf("管理员赠送活动码 %s 的分组 %s，%s",
		result.Code, strings.Join(result.GroupNames, "、"), expireText))
	return result, nil
}

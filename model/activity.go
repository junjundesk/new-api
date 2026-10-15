package model

import (
	"errors"
	"fmt"
	"math"
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
// 授权（ActivityGrant）在兑换时快照活动码的条款，之后修改活动码不影响已发放的授权。
const (
	ActivityCodeStatusEnabled  = 1
	ActivityCodeStatusDisabled = 2

	ActivityGrantStatusActive  = "active"
	ActivityGrantStatusExpired = "expired"
	ActivityGrantStatusRevoked = "revoked"

	ActivityRatioModeInherit  = "inherit"
	ActivityRatioModeOverride = "override"

	ActivityGrantSourceRedeem = "redeem"

	// ActivityCodeMaxLength 是活动码的字符数上限（按 rune 计，中文等宽字符按 1 个算）。
	ActivityCodeMaxLength = 64
	// ActivityCodeMaxRpm 是活动码每分钟请求上限的允许最大值。
	ActivityCodeMaxRpm = 100000
	// ActivityCodeMaxPriceQuota 与用户余额列的 32 位存储保持一致。
	ActivityCodeMaxPriceQuota = math.MaxInt32
	// ActivityCodeMaxDurationSeconds 限制单次授权的有效期长度。
	ActivityCodeMaxDurationSeconds = 10 * 365 * 24 * 3600
)

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
	Id              int            `json:"id"`
	Code            string         `json:"code" gorm:"type:varchar(64);uniqueIndex"`
	Name            string         `json:"name" gorm:"type:varchar(64);index"`
	Description     string         `json:"description" gorm:"type:text"`
	Status          int            `json:"status" gorm:"default:1"`
	GroupName       string         `json:"group_name" gorm:"type:varchar(64);index"`
	DurationSeconds int64          `json:"duration_seconds" gorm:"bigint;default:0"`
	RpmLimit        int            `json:"rpm_limit" gorm:"default:0"`
	PriceQuota      int            `json:"price_quota" gorm:"default:0"`
	RatioMode       string         `json:"ratio_mode" gorm:"type:varchar(16);default:'inherit'"`
	RatioOverride   float64        `json:"ratio_override"`
	StartTime       int64          `json:"start_time" gorm:"bigint;default:0"`
	EndTime         int64          `json:"end_time" gorm:"bigint;default:0"`
	TotalLimit      int            `json:"total_limit" gorm:"default:0"`
	UsedCount       int            `json:"used_count" gorm:"default:0"`
	PerUserLimit    int            `json:"per_user_limit" gorm:"default:1"`
	PublicVisible   int            `json:"public_visible" gorm:"default:0"`
	CreatedBy       int            `json:"created_by"`
	CreatedTime     int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
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
	RpmLimit      int     `json:"rpm_limit" gorm:"default:0"`
	PriceQuota    int     `json:"price_quota" gorm:"default:0"`
	StartTime     int64   `json:"start_time" gorm:"bigint"`
	ExpireTime    int64   `json:"expire_time" gorm:"bigint;index"`
	Status        string  `json:"status" gorm:"type:varchar(32);index;index:idx_activity_grant_user_status,priority:2"`
	Source        string  `json:"source" gorm:"type:varchar(32);default:'redeem'"`
	Remark        string  `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedTime   int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime   int64   `json:"updated_time" gorm:"bigint"`
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

// Update 只写活动码的可编辑字段。used_count 由兑换流程的 CAS 维护，
// code 一经创建不可修改（需要换码请新建）。
func (activityCode *ActivityCode) Update() error {
	return DB.Model(activityCode).
		Select("name", "description", "status", "group_name", "duration_seconds", "rpm_limit",
			"price_quota", "ratio_mode", "ratio_override", "start_time", "end_time",
			"total_limit", "per_user_limit", "public_visible", "updated_time").
		Updates(activityCode).Error
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

// GetActiveActivityGrants 返回用户当前有效的活动授权，结果带 Redis 缓存
// （缓存只加速，到期判定始终以读时过滤为准）。
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
				return filterActiveActivityGrants(grants), nil
			}
		}
	}
	grants, err := queryActiveActivityGrants(DB, userId)
	if err != nil {
		return nil, err
	}
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

func countActivityGrantsForUserCode(tx *gorm.DB, userId int, codeId int) (int64, error) {
	var count int64
	err := tx.Model(&ActivityGrant{}).Where("user_id = ? AND code_id = ?", userId, codeId).Count(&count).Error
	return count, err
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
	GrantId      int    `json:"grant_id"`
	CodeId       int    `json:"code_id"`
	Code         string `json:"code"`
	CodeName     string `json:"code_name"`
	GroupName    string `json:"group_name"`
	ExpireTime   int64  `json:"expire_time"`
	PriceQuota   int    `json:"price_quota"`
	BalanceAfter int    `json:"balance_after"`
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
		if !ratio_setting.ContainsGroupRatio(activityCode.GroupName) {
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
			count, countErr := countActivityGrantsForUserCode(tx, req.UserId, activityCode.Id)
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
		grant := &ActivityGrant{
			UserId:        req.UserId,
			CodeId:        activityCode.Id,
			Code:          activityCode.Code,
			CodeName:      activityCode.Name,
			GroupName:     activityCode.GroupName,
			RatioMode:     activityCode.RatioMode,
			RatioOverride: activityCode.RatioOverride,
			RpmLimit:      activityCode.RpmLimit,
			PriceQuota:    chargedQuota,
			StartTime:     now,
			ExpireTime:    expireTime,
			Status:        ActivityGrantStatusActive,
			Source:        ActivityGrantSourceRedeem,
		}
		if err := tx.Create(grant).Error; err != nil {
			return err
		}
		result.GrantId = grant.Id
		result.CodeId = activityCode.Id
		result.Code = activityCode.Code
		result.CodeName = activityCode.Name
		result.GroupName = activityCode.GroupName
		result.ExpireTime = grant.ExpireTime
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
		result.Code, result.GroupName, logger.LogQuota(result.PriceQuota), expireText))
	return result, nil
}

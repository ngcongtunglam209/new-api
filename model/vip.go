package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"

	"gorm.io/gorm"
)

var (
	ErrVipDisabled     = errors.New("VIP 功能未启用")
	ErrVipTierNotFound = errors.New("VIP 等级不存在")
)

const vipDefaultGroup = "default"

// UserVipStatus is the VIP view of a user, shared by the self and admin endpoints.
type UserVipStatus struct {
	Enabled          bool   `json:"enabled"`
	Tier             string `json:"tier"`
	Group            string `json:"group"`
	ExpiresAt        int64  `json:"expires_at"`
	Locked           bool   `json:"locked"`
	Spend            int64  `json:"spend"`
	TotalPaid        int64  `json:"total_paid"`
	NextTier         string `json:"next_tier"`
	NextTierMinSpend int64  `json:"next_tier_min_spend"`
	WindowDays       int    `json:"window_days"`
	// SubscriptionHeld reports that a paid subscription currently owns the
	// user's group, so the VIP group grant is recorded but not applied.
	SubscriptionHeld bool `json:"subscription_held"`
}

// hasActiveSubscriptionUpgradeTx reports whether a paid subscription currently
// owns this user's group. Both features write users.group and both keep their
// own "previous group" snapshot, so only one of them may write it at a time —
// otherwise the snapshots reference each other and the user is stranded on the
// wrong group when either one expires.
func hasActiveSubscriptionUpgradeTx(tx *gorm.DB, userId int, now int64) (bool, error) {
	var sub UserSubscription
	res := tx.Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group <> ''",
		userId, "active", now).
		Limit(1).
		Find(&sub)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func vipRevertTarget(user *User) string {
	target := strings.TrimSpace(user.VipPrevGroup)
	if target == "" {
		target = vipDefaultGroup
	}
	return target
}

// AddVipSpendTx accrues real money paid, in quota units, toward a user's VIP
// window. It must run inside the transaction that records the payment so a
// crash cannot credit quota while losing the spend that paid for it.
//
// The window rolls: while it is open every payment extends it, and a payment
// arriving after it closed starts a fresh one from zero. Qualifying spend is
// therefore always "paid inside the current window", never a lifetime total —
// vip_total_paid keeps the lifetime figure for reporting.
func AddVipSpendTx(tx *gorm.DB, userId int, quota int64) error {
	if tx == nil {
		return errors.New("nil transaction")
	}
	if !setting.VipEnabled() || userId <= 0 || quota <= 0 {
		return nil
	}
	var user User
	if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	windowEnd := now + int64(setting.VipWindowDays())*86400
	updates := map[string]interface{}{
		"vip_total_paid": gorm.Expr("vip_total_paid + ?", quota),
	}
	switch {
	case user.VipLocked:
		// An admin-pinned tier owns its own expiry, so only the counters move.
		updates["vip_spend"] = gorm.Expr("vip_spend + ?", quota)
	case user.VipExpiresAt > now:
		updates["vip_spend"] = gorm.Expr("vip_spend + ?", quota)
		updates["vip_expires_at"] = windowEnd
	default:
		updates["vip_spend"] = quota
		updates["vip_expires_at"] = windowEnd
	}
	return tx.Model(&User{}).Where("id = ?", userId).Updates(updates).Error
}

// SetUserVipTier grants a tier to a user for the given number of days. Passing
// locked pins the tier so the expiry task leaves it alone.
func SetUserVipTier(userId int, tierKey string, days int, locked bool) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	if !setting.VipEnabled() {
		return "", ErrVipDisabled
	}
	tier, ok := setting.GetVipTierByKey(tierKey)
	if !ok {
		return "", ErrVipTierNotFound
	}
	if days <= 0 {
		days = setting.VipWindowDays()
	}
	now := common.GetTimestamp()
	appliedGroup := ""
	groupChanged := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		heldBySubscription, err := hasActiveSubscriptionUpgradeTx(tx, userId, GetDBTimestamp())
		if err != nil {
			return err
		}
		prevGroup := strings.TrimSpace(user.VipPrevGroup)
		if !setting.IsVipManagedGroup(user.Group) {
			prevGroup = user.Group
		}
		updates := map[string]interface{}{
			"vip_tier":       tier.Key,
			"vip_expires_at": now + int64(days)*86400,
			"vip_locked":     locked,
			"vip_prev_group": prevGroup,
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).Updates(updates).Error; err != nil {
			return err
		}
		appliedGroup = user.Group
		if heldBySubscription || user.Group == tier.Group {
			return nil
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("group", tier.Group).Error; err != nil {
			return err
		}
		appliedGroup = tier.Group
		groupChanged = true
		return nil
	})
	if err != nil {
		return "", err
	}
	if groupChanged {
		refreshVipUserGroupCache(userId, "vip tier grant")
	}
	return appliedGroup, nil
}

// ClearUserVipTier removes a user's tier and returns them to the group held
// before the grant.
func ClearUserVipTier(userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	revertedGroup := ""
	groupChanged := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Updates(map[string]interface{}{
				"vip_tier":       "",
				"vip_expires_at": 0,
				"vip_locked":     false,
				"vip_prev_group": "",
				"vip_spend":      0,
			}).Error; err != nil {
			return err
		}
		revertedGroup = user.Group
		heldBySubscription, err := hasActiveSubscriptionUpgradeTx(tx, userId, GetDBTimestamp())
		if err != nil {
			return err
		}
		if heldBySubscription || !setting.IsVipManagedGroup(user.Group) {
			return nil
		}
		target := vipRevertTarget(&user)
		if target == user.Group {
			return nil
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("group", target).Error; err != nil {
			return err
		}
		revertedGroup = target
		groupChanged = true
		return nil
	})
	if err != nil {
		return "", err
	}
	if groupChanged {
		refreshVipUserGroupCache(userId, "vip tier clear")
	}
	return revertedGroup, nil
}

// InitializeVipLockedFlags backfills users.vip_locked for rows that predate the
// column. AutoMigrate adds the boolean without a default, so existing rows read
// NULL, and NULL fails both the "= false" predicate the sweeps rely on and the
// scan into the bool field: every pre-existing user would be invisible to VIP.
func InitializeVipLockedFlags() error {
	return DB.Model(&User{}).Where("vip_locked IS NULL").Update("vip_locked", false).Error
}

// MaybeAutoPromoteVipTier promotes a user whose qualifying spend now clears a
// higher tier. Call it after the payment transaction has committed, so the
// group cache is refreshed against committed state.
//
// It only ever promotes. A tier is lost when its window expires, never in the
// middle of one, so a customer cannot be demoted by a threshold edit while they
// are still paying for the level they were promised.
func MaybeAutoPromoteVipTier(userId int) {
	if !setting.VipAutoPromoteEnabled() {
		return
	}
	promoted, err := promoteVipTierIfQualified(userId)
	if err != nil {
		common.SysError(fmt.Sprintf("vip auto promote failed: user_id=%d error=%v", userId, err))
		return
	}
	if promoted {
		refreshVipUserGroupCache(userId, "vip auto promote")
	}
}

// promoteVipTierIfQualified reports whether the user's group was rewritten.
func promoteVipTierIfQualified(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	groupChanged := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if user.VipLocked {
			return nil
		}
		now := GetDBTimestamp()
		if user.VipExpiresAt <= now {
			// The qualifying window is closed; the expiry task owns this user.
			return nil
		}
		target, ok := setting.ResolveVipTierForSpend(user.VipSpend)
		if !ok || target.Key == user.VipTier {
			return nil
		}
		if current, ok := setting.GetVipTierByKey(user.VipTier); ok && target.MinSpend <= current.MinSpend {
			return nil
		}
		prevGroup := strings.TrimSpace(user.VipPrevGroup)
		if !setting.IsVipManagedGroup(user.Group) {
			prevGroup = user.Group
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Updates(map[string]interface{}{
				"vip_tier":       target.Key,
				"vip_prev_group": prevGroup,
			}).Error; err != nil {
			return err
		}
		heldBySubscription, err := hasActiveSubscriptionUpgradeTx(tx, userId, now)
		if err != nil {
			return err
		}
		if heldBySubscription || user.Group == target.Group {
			return nil
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("group", target.Group).Error; err != nil {
			return err
		}
		groupChanged = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return groupChanged, nil
}

// AutoPromoteDueVipTiers is the backstop sweep for users whose spend qualifies
// them but whose promotion was missed — spend edited by an admin, a payment
// path that failed after commit, or a threshold that was just lowered.
func AutoPromoteDueVipTiers(limit int) (int, error) {
	if !setting.VipAutoPromoteEnabled() {
		return 0, nil
	}
	lowest := setting.LowestVipThreshold()
	if lowest <= 0 {
		return 0, nil
	}
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var users []User
	if err := DB.Select("id").
		Where("COALESCE(vip_locked, ?) = ? AND vip_spend >= ? AND vip_expires_at > ?", false, false, lowest, now).
		Order("id asc").
		Limit(limit).
		Find(&users).Error; err != nil {
		return 0, err
	}
	promoted := 0
	for _, u := range users {
		changed, err := promoteVipTierIfQualified(u.Id)
		if err != nil {
			return promoted, err
		}
		if changed {
			refreshVipUserGroupCache(u.Id, "vip auto promote sweep")
			promoted++
		}
	}
	return promoted, nil
}

// ExpireDueVipTiers drops expired tiers and returns how many users were reverted.
func ExpireDueVipTiers(limit int) (int, error) {
	if !setting.VipEnabled() {
		return 0, nil
	}
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var users []User
	if err := DB.Select("id").
		Where("vip_tier <> '' AND COALESCE(vip_locked, ?) = ? AND vip_expires_at > 0 AND vip_expires_at <= ?", false, false, now).
		Order("vip_expires_at asc, id asc").
		Limit(limit).
		Find(&users).Error; err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, nil
	}
	expired := 0
	for _, u := range users {
		userId := u.Id
		groupChanged := false
		err := DB.Transaction(func(tx *gorm.DB) error {
			var user User
			if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
				return err
			}
			if user.VipTier == "" || user.VipLocked || user.VipExpiresAt <= 0 || user.VipExpiresAt > now {
				return nil
			}
			// The window is over, so qualifying spend restarts from zero:
			// otherwise a single small payment would immediately restore the
			// highest tier the user ever reached.
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Updates(map[string]interface{}{
					"vip_tier":       "",
					"vip_expires_at": 0,
					"vip_prev_group": "",
					"vip_spend":      0,
				}).Error; err != nil {
				return err
			}
			expired++
			heldBySubscription, err := hasActiveSubscriptionUpgradeTx(tx, userId, now)
			if err != nil {
				return err
			}
			if heldBySubscription || !setting.IsVipManagedGroup(user.Group) {
				return nil
			}
			target := vipRevertTarget(&user)
			if target == user.Group {
				return nil
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", target).Error; err != nil {
				return err
			}
			groupChanged = true
			return nil
		})
		if err != nil {
			return expired, err
		}
		if groupChanged {
			refreshVipUserGroupCache(userId, "vip tier expiration")
		}
	}
	return expired, nil
}

func GetUserVipStatus(userId int) (*UserVipStatus, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var user User
	if err := DB.Select("id", commonGroupCol, "vip_tier", "vip_spend", "vip_total_paid", "vip_expires_at", "vip_locked").
		Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}
	status := &UserVipStatus{
		Enabled:    setting.VipEnabled(),
		Tier:       user.VipTier,
		Group:      user.Group,
		ExpiresAt:  user.VipExpiresAt,
		Locked:     user.VipLocked,
		Spend:      user.VipSpend,
		TotalPaid:  user.VipTotalPaid,
		WindowDays: setting.VipWindowDays(),
	}
	if next, ok := setting.NextVipTierForSpend(user.VipSpend); ok {
		status.NextTier = next.Key
		status.NextTierMinSpend = next.MinSpend
	}
	if tier, ok := setting.GetVipTierByKey(user.VipTier); ok && tier.Group != user.Group {
		status.SubscriptionHeld = true
	}
	return status, nil
}

func refreshVipUserGroupCache(userId int, operation string) {
	if err := RefreshUserGroupCache(userId); err != nil {
		common.SysError(fmt.Sprintf("failed to refresh user group cache after %s: user_id=%d error=%v", operation, userId, err))
	}
}

package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupVipTestDB(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&User{}, &UserSubscription{}, &Redemption{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)

	vipSetting := setting.GetVipSetting()
	previousEnabled := vipSetting.Enabled
	vipSetting.Enabled = true

	t.Cleanup(func() {
		vipSetting.Enabled = previousEnabled
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		_ = sqlDB.Close()
	})
}

func createVipTestUser(t *testing.T, username, group string) *User {
	t.Helper()
	user := User{
		Username:    username,
		Password:    "unused-password-hash",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       group,
		AffCode:     "aff-" + username,
		AuthVersion: 1,
	}
	require.NoError(t, DB.Create(&user).Error)
	return &user
}

func reloadVipTestUser(t *testing.T, userId int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.Where("id = ?", userId).First(&user).Error)
	return user
}

func TestSetAndClearUserVipTierMovesGroup(t *testing.T) {
	setupVipTestDB(t)
	user := createVipTestUser(t, "vip-grant", "default")

	group, err := SetUserVipTier(user.Id, "vip2", 30, false)
	require.NoError(t, err)
	assert.Equal(t, "vip2", group)

	stored := reloadVipTestUser(t, user.Id)
	assert.Equal(t, "vip2", stored.Group)
	assert.Equal(t, "vip2", stored.VipTier)
	assert.Equal(t, "default", stored.VipPrevGroup)
	assert.Greater(t, stored.VipExpiresAt, common.GetTimestamp())

	// Re-grant a different tier must keep the original pre-VIP group, otherwise
	// clearing the tier would strand the user on a VIP group.
	group, err = SetUserVipTier(user.Id, "vip3", 30, false)
	require.NoError(t, err)
	assert.Equal(t, "vip3", group)
	stored = reloadVipTestUser(t, user.Id)
	assert.Equal(t, "default", stored.VipPrevGroup)

	group, err = ClearUserVipTier(user.Id)
	require.NoError(t, err)
	assert.Equal(t, "default", group)
	stored = reloadVipTestUser(t, user.Id)
	assert.Equal(t, "default", stored.Group)
	assert.Empty(t, stored.VipTier)
	assert.Zero(t, stored.VipExpiresAt)
}

func TestSetUserVipTierYieldsToActiveSubscription(t *testing.T) {
	setupVipTestDB(t)
	user := createVipTestUser(t, "vip-subscription", "pro")

	require.NoError(t, DB.Create(&UserSubscription{
		UserId:       user.Id,
		Status:       "active",
		EndTime:      GetDBTimestamp() + 86400,
		UpgradeGroup: "pro",
	}).Error)

	group, err := SetUserVipTier(user.Id, "vip4", 30, false)
	require.NoError(t, err)
	assert.Equal(t, "pro", group, "an active subscription owns users.group")

	stored := reloadVipTestUser(t, user.Id)
	assert.Equal(t, "pro", stored.Group)
	assert.Equal(t, "vip4", stored.VipTier, "the tier is still recorded for the badge")
	assert.Equal(t, "pro", stored.VipPrevGroup)
}

func TestExpireDueVipTiersRevertsGroupAndResetsSpend(t *testing.T) {
	setupVipTestDB(t)
	now := GetDBTimestamp()

	expiring := createVipTestUser(t, "vip-expiring", "vip2")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", expiring.Id).Updates(map[string]interface{}{
		"vip_tier":       "vip2",
		"vip_spend":      12345,
		"vip_total_paid": 12345,
		"vip_expires_at": now - 10,
		"vip_prev_group": "default",
	}).Error)

	locked := createVipTestUser(t, "vip-locked", "vip3")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", locked.Id).Updates(map[string]interface{}{
		"vip_tier":       "vip3",
		"vip_spend":      999,
		"vip_expires_at": now - 10,
		"vip_prev_group": "default",
		"vip_locked":     true,
	}).Error)

	future := createVipTestUser(t, "vip-future", "vip1")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", future.Id).Updates(map[string]interface{}{
		"vip_tier":       "vip1",
		"vip_expires_at": now + 86400,
		"vip_prev_group": "default",
	}).Error)

	count, err := ExpireDueVipTiers(100)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	stored := reloadVipTestUser(t, expiring.Id)
	assert.Equal(t, "default", stored.Group)
	assert.Empty(t, stored.VipTier)
	assert.Zero(t, stored.VipSpend, "qualifying spend restarts with the new window")
	assert.EqualValues(t, 12345, stored.VipTotalPaid, "lifetime paid is reporting data and must survive")

	storedLocked := reloadVipTestUser(t, locked.Id)
	assert.Equal(t, "vip3", storedLocked.Group)
	assert.Equal(t, "vip3", storedLocked.VipTier)

	storedFuture := reloadVipTestUser(t, future.Id)
	assert.Equal(t, "vip1", storedFuture.Group)
	assert.Equal(t, "vip1", storedFuture.VipTier)
}

func TestExpireDueVipTiersKeepsSubscriptionGroup(t *testing.T) {
	setupVipTestDB(t)
	now := GetDBTimestamp()
	user := createVipTestUser(t, "vip-expiring-subscribed", "pro")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"vip_tier":       "vip2",
		"vip_expires_at": now - 10,
		"vip_prev_group": "default",
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:       user.Id,
		Status:       "active",
		EndTime:      now + 86400,
		UpgradeGroup: "pro",
	}).Error)

	count, err := ExpireDueVipTiers(100)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	stored := reloadVipTestUser(t, user.Id)
	assert.Equal(t, "pro", stored.Group, "VIP expiry must not steal the subscription's group")
	assert.Empty(t, stored.VipTier)
}

func TestAddVipSpendTxRollsTheWindow(t *testing.T) {
	setupVipTestDB(t)
	user := createVipTestUser(t, "vip-spend", "default")
	windowSeconds := int64(setting.VipWindowDays()) * 86400

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return AddVipSpendTx(tx, user.Id, 1000)
	}))
	stored := reloadVipTestUser(t, user.Id)
	assert.EqualValues(t, 1000, stored.VipSpend)
	assert.EqualValues(t, 1000, stored.VipTotalPaid)
	firstWindowEnd := stored.VipExpiresAt
	assert.InDelta(t, common.GetTimestamp()+windowSeconds, firstWindowEnd, 5)

	// A payment inside the open window adds up and pushes the window out.
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return AddVipSpendTx(tx, user.Id, 500)
	}))
	stored = reloadVipTestUser(t, user.Id)
	assert.EqualValues(t, 1500, stored.VipSpend)
	assert.EqualValues(t, 1500, stored.VipTotalPaid)
	assert.GreaterOrEqual(t, stored.VipExpiresAt, firstWindowEnd)

	// A payment after the window closed starts a fresh window from zero, so a
	// lapsed customer cannot restore an old tier with one small purchase.
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).
		Update("vip_expires_at", common.GetTimestamp()-10).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return AddVipSpendTx(tx, user.Id, 700)
	}))
	stored = reloadVipTestUser(t, user.Id)
	assert.EqualValues(t, 700, stored.VipSpend)
	assert.EqualValues(t, 2200, stored.VipTotalPaid, "lifetime paid never resets")

	// Non-positive amounts must not move anything.
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return AddVipSpendTx(tx, user.Id, 0)
	}))
	assert.EqualValues(t, 700, reloadVipTestUser(t, user.Id).VipSpend)
}

func TestAddVipSpendTxKeepsPinnedTierExpiry(t *testing.T) {
	setupVipTestDB(t)
	user := createVipTestUser(t, "vip-spend-locked", "vip2")
	pinnedUntil := common.GetTimestamp() + 3600
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"vip_tier":       "vip2",
		"vip_locked":     true,
		"vip_expires_at": pinnedUntil,
	}).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return AddVipSpendTx(tx, user.Id, 250)
	}))
	stored := reloadVipTestUser(t, user.Id)
	assert.EqualValues(t, 250, stored.VipSpend)
	assert.EqualValues(t, pinnedUntil, stored.VipExpiresAt, "an admin-pinned tier keeps its own expiry")
}

func TestCreditTopUpQuotaAccruesVipSpend(t *testing.T) {
	setupVipTestDB(t)
	user := createVipTestUser(t, "vip-topup", "default")

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return creditTopUpQuota(tx, user.Id, 2500, nil)
	}))

	stored := reloadVipTestUser(t, user.Id)
	assert.EqualValues(t, 2500, stored.Quota)
	assert.EqualValues(t, 2500, stored.VipSpend, "every paid top-up path credits quota through here")
	assert.EqualValues(t, 2500, stored.VipTotalPaid)
}

func TestRedeemAccruesVipSpendOnlyForSoldCodes(t *testing.T) {
	setupVipTestDB(t)
	user := createVipTestUser(t, "vip-redeem", "default")

	require.NoError(t, DB.Create(&Redemption{
		Key:    "sale-code-key-000000000000000001",
		Name:   "sale-august",
		Status: common.RedemptionCodeStatusEnabled,
		Quota:  700,
	}).Error)
	require.NoError(t, DB.Create(&Redemption{
		Key:    "gift-code-key-000000000000000002",
		Name:   "gift-welcome",
		Status: common.RedemptionCodeStatusEnabled,
		Quota:  500,
	}).Error)

	quota, err := Redeem("sale-code-key-000000000000000001", user.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 700, quota)
	stored := reloadVipTestUser(t, user.Id)
	assert.EqualValues(t, 700, stored.VipSpend)

	quota, err = Redeem("gift-code-key-000000000000000002", user.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 500, quota)
	stored = reloadVipTestUser(t, user.Id)
	assert.EqualValues(t, 1200, stored.Quota, "the gift still credits quota")
	assert.EqualValues(t, 700, stored.VipSpend, "but a gift code is not money paid")
}

func enableVipAutoPromote(t *testing.T) {
	t.Helper()
	vipSetting := setting.GetVipSetting()
	previous := vipSetting.AutoPromoteEnabled
	vipSetting.AutoPromoteEnabled = true
	t.Cleanup(func() {
		vipSetting.AutoPromoteEnabled = previous
	})
}

func TestAutoPromoteLiftsTierWhenSpendQualifies(t *testing.T) {
	setupVipTestDB(t)
	enableVipAutoPromote(t)
	user := createVipTestUser(t, "vip-promote", "default")

	tier, ok := setting.GetVipTierByKey("vip2")
	require.True(t, ok)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return AddVipSpendTx(tx, user.Id, tier.MinSpend)
	}))
	MaybeAutoPromoteVipTier(user.Id)

	stored := reloadVipTestUser(t, user.Id)
	assert.Equal(t, "vip2", stored.VipTier)
	assert.Equal(t, tier.Group, stored.Group)
	assert.Equal(t, "default", stored.VipPrevGroup)
}

func TestAutoPromoteNeverDemotesMidWindow(t *testing.T) {
	setupVipTestDB(t)
	enableVipAutoPromote(t)
	user := createVipTestUser(t, "vip-no-demote", "vip3")

	// Spend only clears vip1 while the user already holds vip3 — the tier they
	// were promised for this window must survive.
	vip1, ok := setting.GetVipTierByKey("vip1")
	require.True(t, ok)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"vip_tier":       "vip3",
		"vip_spend":      vip1.MinSpend,
		"vip_expires_at": common.GetTimestamp() + 86400,
		"vip_prev_group": "default",
	}).Error)

	MaybeAutoPromoteVipTier(user.Id)

	stored := reloadVipTestUser(t, user.Id)
	assert.Equal(t, "vip3", stored.VipTier)
	assert.Equal(t, "vip3", stored.Group)
}

func TestAutoPromoteSkipsPinnedAndDisabledCases(t *testing.T) {
	setupVipTestDB(t)
	vip2, ok := setting.GetVipTierByKey("vip2")
	require.True(t, ok)

	t.Run("disabled flag keeps everything untouched", func(t *testing.T) {
		user := createVipTestUser(t, "vip-promote-off", "default")
		require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
			"vip_spend":      vip2.MinSpend,
			"vip_expires_at": common.GetTimestamp() + 86400,
		}).Error)

		MaybeAutoPromoteVipTier(user.Id)

		stored := reloadVipTestUser(t, user.Id)
		assert.Empty(t, stored.VipTier)
		assert.Equal(t, "default", stored.Group)
	})

	t.Run("pinned tier is left to the admin", func(t *testing.T) {
		enableVipAutoPromote(t)
		user := createVipTestUser(t, "vip-promote-pinned", "vip1")
		require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
			"vip_tier":       "vip1",
			"vip_locked":     true,
			"vip_spend":      vip2.MinSpend,
			"vip_expires_at": common.GetTimestamp() + 86400,
			"vip_prev_group": "default",
		}).Error)

		MaybeAutoPromoteVipTier(user.Id)

		stored := reloadVipTestUser(t, user.Id)
		assert.Equal(t, "vip1", stored.VipTier)
		assert.Equal(t, "vip1", stored.Group)
	})
}

func TestAutoPromoteYieldsToActiveSubscription(t *testing.T) {
	setupVipTestDB(t)
	enableVipAutoPromote(t)
	user := createVipTestUser(t, "vip-promote-subscribed", "pro")
	vip2, ok := setting.GetVipTierByKey("vip2")
	require.True(t, ok)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"vip_spend":      vip2.MinSpend,
		"vip_expires_at": common.GetTimestamp() + 86400,
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:       user.Id,
		Status:       "active",
		EndTime:      GetDBTimestamp() + 86400,
		UpgradeGroup: "pro",
	}).Error)

	MaybeAutoPromoteVipTier(user.Id)

	stored := reloadVipTestUser(t, user.Id)
	assert.Equal(t, "vip2", stored.VipTier, "the tier is recorded for the badge")
	assert.Equal(t, "pro", stored.Group, "but the subscription keeps the group")
}

func TestAutoPromoteSweepCatchesMissedUsers(t *testing.T) {
	setupVipTestDB(t)
	enableVipAutoPromote(t)
	vip2, ok := setting.GetVipTierByKey("vip2")
	require.True(t, ok)

	qualified := createVipTestUser(t, "vip-sweep-qualified", "default")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", qualified.Id).Updates(map[string]interface{}{
		"vip_spend":      vip2.MinSpend,
		"vip_expires_at": common.GetTimestamp() + 86400,
	}).Error)

	// Window already closed: the expiry path owns this user, not the sweep.
	lapsed := createVipTestUser(t, "vip-sweep-lapsed", "default")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lapsed.Id).Updates(map[string]interface{}{
		"vip_spend":      vip2.MinSpend,
		"vip_expires_at": common.GetTimestamp() - 10,
	}).Error)

	promoted, err := AutoPromoteDueVipTiers(100)
	require.NoError(t, err)
	assert.Equal(t, 1, promoted)
	assert.Equal(t, "vip2", reloadVipTestUser(t, qualified.Id).VipTier)
	assert.Empty(t, reloadVipTestUser(t, lapsed.Id).VipTier)
}

func TestInitializeVipLockedFlagsMakesLegacyUsersVisibleToPromotion(t *testing.T) {
	setupVipTestDB(t)
	enableVipAutoPromote(t)
	vip2, ok := setting.GetVipTierByKey("vip2")
	require.True(t, ok)

	user := createVipTestUser(t, "vip-legacy-null-locked", "default")
	// AutoMigrate adds vip_locked without a default, so rows that predate the
	// column read NULL. NULL never satisfies "vip_locked = false", which would
	// hide every pre-existing user from the promotion and expiry sweeps.
	require.NoError(t, DB.Exec(
		"UPDATE users SET vip_locked = NULL, vip_spend = ?, vip_expires_at = ? WHERE id = ?",
		vip2.MinSpend, common.GetTimestamp()+86400, user.Id).Error)

	require.NoError(t, InitializeVipLockedFlags())

	promoted, err := AutoPromoteDueVipTiers(100)
	require.NoError(t, err)
	assert.Equal(t, 1, promoted)

	stored := reloadVipTestUser(t, user.Id)
	assert.False(t, stored.VipLocked)
	assert.Equal(t, "vip2", stored.VipTier)
	assert.Equal(t, vip2.Group, stored.Group)
}

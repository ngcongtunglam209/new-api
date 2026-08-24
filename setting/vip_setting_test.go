package setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withVipTiers(t *testing.T, tiers []VipTier) {
	t.Helper()
	previous := vipSetting.Tiers
	vipSetting.Tiers = tiers
	t.Cleanup(func() {
		vipSetting.Tiers = previous
	})
}

func withGroupRatios(t *testing.T, jsonStr string) {
	t.Helper()
	previous := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(jsonStr))
	t.Cleanup(func() {
		_ = ratio_setting.UpdateGroupRatioByJSONString(previous)
	})
}

func TestResolveVipTierForSpend(t *testing.T) {
	withVipTiers(t, []VipTier{
		{Key: "vip1", Group: "vip1", MinSpend: 100, Enabled: true},
		{Key: "vip2", Group: "vip2", MinSpend: 300, Enabled: true},
		{Key: "vip3", Group: "vip3", MinSpend: 900, Enabled: false},
	})

	cases := []struct {
		name      string
		spend     int64
		wantKey   string
		wantFound bool
	}{
		{name: "below first threshold", spend: 99, wantFound: false},
		{name: "exactly at threshold qualifies", spend: 100, wantKey: "vip1", wantFound: true},
		{name: "between thresholds keeps lower tier", spend: 299, wantKey: "vip1", wantFound: true},
		{name: "highest qualifying tier wins", spend: 5000, wantKey: "vip2", wantFound: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tier, found := ResolveVipTierForSpend(tc.spend)
			assert.Equal(t, tc.wantFound, found)
			assert.Equal(t, tc.wantKey, tier.Key)
		})
	}
}

func TestNextVipTierForSpend(t *testing.T) {
	withVipTiers(t, []VipTier{
		{Key: "vip1", Group: "vip1", MinSpend: 100, Enabled: true},
		{Key: "vip2", Group: "vip2", MinSpend: 300, Enabled: true},
		{Key: "vip3", Group: "vip3", MinSpend: 900, Enabled: false},
	})

	tier, found := NextVipTierForSpend(0)
	require.True(t, found)
	assert.Equal(t, "vip1", tier.Key)

	tier, found = NextVipTierForSpend(100)
	require.True(t, found)
	assert.Equal(t, "vip2", tier.Key)

	// vip3 is disabled, so a user past vip2 has nothing left to climb toward.
	_, found = NextVipTierForSpend(300)
	assert.False(t, found)
}

func TestRedemptionCountsAsVipSpend(t *testing.T) {
	previous := vipSetting.RedemptionExcludePrefixes
	vipSetting.RedemptionExcludePrefixes = "gift-, test-"
	t.Cleanup(func() {
		vipSetting.RedemptionExcludePrefixes = previous
	})

	assert.True(t, RedemptionCountsAsVipSpend("sale-2026-08"))
	assert.False(t, RedemptionCountsAsVipSpend("gift-newyear"))
	assert.False(t, RedemptionCountsAsVipSpend("test-code"))
	assert.True(t, RedemptionCountsAsVipSpend(""))
}

func TestCheckVipTiers(t *testing.T) {
	withGroupRatios(t, `{"default":1,"vip1":0.95,"vip2":0.9,"vip3":0.82,"cheap":0.5}`)

	t.Run("accepts a rising ladder with falling ratios", func(t *testing.T) {
		require.NoError(t, CheckVipTiers(`[
			{"key":"vip1","group":"vip1","min_spend":100,"enabled":true},
			{"key":"vip2","group":"vip2","min_spend":300,"enabled":true},
			{"key":"vip3","group":"vip3","min_spend":900,"enabled":true}
		]`))
	})

	t.Run("rejects a cheaper tier priced below a dearer one", func(t *testing.T) {
		err := CheckVipTiers(`[
			{"key":"vip1","group":"cheap","min_spend":100,"enabled":true},
			{"key":"vip2","group":"vip2","min_spend":300,"enabled":true}
		]`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not price above a cheaper tier")
	})

	t.Run("rejects a group with no configured ratio", func(t *testing.T) {
		err := CheckVipTiers(`[{"key":"vip9","group":"vip9","min_spend":100,"enabled":true}]`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no group ratio configured")
	})

	t.Run("rejects duplicate keys and groups", func(t *testing.T) {
		err := CheckVipTiers(`[
			{"key":"vip1","group":"vip1","min_spend":100,"enabled":true},
			{"key":"vip1","group":"vip2","min_spend":300,"enabled":true}
		]`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate vip tier key")

		err = CheckVipTiers(`[
			{"key":"vip1","group":"vip1","min_spend":100,"enabled":true},
			{"key":"vip2","group":"vip1","min_spend":300,"enabled":true}
		]`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate vip tier group")
	})

	t.Run("rejects equal thresholds and non-positive thresholds", func(t *testing.T) {
		err := CheckVipTiers(`[
			{"key":"vip1","group":"vip1","min_spend":100,"enabled":true},
			{"key":"vip2","group":"vip2","min_spend":100,"enabled":true}
		]`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "strictly increasing")

		err = CheckVipTiers(`[{"key":"vip1","group":"vip1","min_spend":0,"enabled":true}]`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "greater than 0")
	})
}

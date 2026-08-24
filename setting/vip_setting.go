package setting

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	// MaxVipTiers bounds the ladder so a malformed payload cannot create an
	// unbounded number of managed groups.
	MaxVipTiers = 10
	// VipTiersOptionKey is the option row that stores the ladder.
	VipTiersOptionKey = "vip_setting.tiers"
)

// VipTier is one rung of the VIP ladder. Deliberately carries no price: the
// discount lives in GroupRatio/GroupGroupRatio and the request limit lives in
// ModelRequestRateLimitGroup, both keyed by Group. Keeping pricing out of here
// means a tier is only a threshold plus the group it grants.
type VipTier struct {
	Key      string `json:"key"`
	Group    string `json:"group"`
	MinSpend int64  `json:"min_spend"` // qualifying spend in quota units
	Enabled  bool   `json:"enabled"`
}

type VipSetting struct {
	// Enabled is the master switch: manual tier assignment and the expiry task
	// both no-op while it is false.
	Enabled bool `json:"enabled"`
	// AutoPromoteEnabled turns on threshold-based promotion. Kept separate so
	// spend can accumulate for a while before any tier is granted from it.
	AutoPromoteEnabled bool `json:"auto_promote_enabled"`
	// WindowDays is how long a granted tier lasts, and the window qualifying
	// spend accumulates over. Every new payment extends it.
	WindowDays int `json:"window_days"`
	// RedemptionExcludePrefixes lists redemption-code name prefixes that must
	// not count as spend (gift and test codes), comma separated.
	RedemptionExcludePrefixes string    `json:"redemption_exclude_prefixes"`
	Tiers                     []VipTier `json:"tiers"`
}

var vipSetting = VipSetting{
	Enabled:                   false,
	AutoPromoteEnabled:        false,
	WindowDays:                90,
	RedemptionExcludePrefixes: "gift-,test-",
	Tiers: []VipTier{
		{Key: "vip1", Group: "vip1", MinSpend: 5 * int64(common.QuotaPerUnit), Enabled: true},
		{Key: "vip2", Group: "vip2", MinSpend: 15 * int64(common.QuotaPerUnit), Enabled: true},
		{Key: "vip3", Group: "vip3", MinSpend: 40 * int64(common.QuotaPerUnit), Enabled: true},
		{Key: "vip4", Group: "vip4", MinSpend: 100 * int64(common.QuotaPerUnit), Enabled: true},
	},
}

func init() {
	config.GlobalConfig.Register("vip_setting", &vipSetting)
}

func GetVipSetting() *VipSetting {
	return &vipSetting
}

func VipEnabled() bool {
	return vipSetting.Enabled
}

func VipAutoPromoteEnabled() bool {
	return vipSetting.Enabled && vipSetting.AutoPromoteEnabled
}

// LowestVipThreshold returns the cheapest enabled threshold, so the promotion
// sweep can skip users who cannot possibly qualify. Returns 0 when the ladder
// has no enabled tier.
func LowestVipThreshold() int64 {
	lowest := int64(0)
	for _, tier := range vipSetting.Tiers {
		if !tier.Enabled || tier.MinSpend <= 0 {
			continue
		}
		if lowest == 0 || tier.MinSpend < lowest {
			lowest = tier.MinSpend
		}
	}
	return lowest
}

func VipWindowDays() int {
	if vipSetting.WindowDays <= 0 {
		return 90
	}
	return vipSetting.WindowDays
}

// GetVipTiers returns the ladder ordered by ascending threshold.
func GetVipTiers() []VipTier {
	tiers := make([]VipTier, len(vipSetting.Tiers))
	copy(tiers, vipSetting.Tiers)
	for i := 1; i < len(tiers); i++ {
		for j := i; j > 0 && tiers[j].MinSpend < tiers[j-1].MinSpend; j-- {
			tiers[j], tiers[j-1] = tiers[j-1], tiers[j]
		}
	}
	return tiers
}

func GetVipTierByKey(key string) (VipTier, bool) {
	for _, tier := range vipSetting.Tiers {
		if tier.Key == key {
			return tier, true
		}
	}
	return VipTier{}, false
}

// IsVipManagedGroup reports whether a group is granted by the ladder. Used to
// decide whether a user's current group may be reverted on expiry.
func IsVipManagedGroup(group string) bool {
	if group == "" {
		return false
	}
	for _, tier := range vipSetting.Tiers {
		if tier.Group == group {
			return true
		}
	}
	return false
}

// ResolveVipTierForSpend returns the highest enabled tier the spend qualifies
// for, or false when the spend clears none.
func ResolveVipTierForSpend(spend int64) (VipTier, bool) {
	resolved := VipTier{}
	found := false
	for _, tier := range vipSetting.Tiers {
		if !tier.Enabled || tier.MinSpend <= 0 || spend < tier.MinSpend {
			continue
		}
		if !found || tier.MinSpend > resolved.MinSpend {
			resolved = tier
			found = true
		}
	}
	return resolved, found
}

// NextVipTierForSpend returns the cheapest enabled tier the spend has not
// reached yet, so the frontend can render progress toward it.
func NextVipTierForSpend(spend int64) (VipTier, bool) {
	next := VipTier{}
	found := false
	for _, tier := range vipSetting.Tiers {
		if !tier.Enabled || tier.MinSpend <= 0 || spend >= tier.MinSpend {
			continue
		}
		if !found || tier.MinSpend < next.MinSpend {
			next = tier
			found = true
		}
	}
	return next, found
}

// RedemptionCountsAsVipSpend reports whether a redemption code name is a paid
// sale rather than a gift or test grant.
func RedemptionCountsAsVipSpend(name string) bool {
	for _, prefix := range strings.Split(vipSetting.RedemptionExcludePrefixes, ",") {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

// CheckVipTiers validates a ladder payload before it is persisted. Thresholds
// must rise and group ratios must fall, otherwise a cheaper tier would price
// below a more expensive one.
func CheckVipTiers(jsonStr string) error {
	var tiers []VipTier
	if err := common.UnmarshalJsonStr(jsonStr, &tiers); err != nil {
		return err
	}
	if len(tiers) > MaxVipTiers {
		return fmt.Errorf("vip tier count must not exceed %d", MaxVipTiers)
	}
	seenKeys := make(map[string]struct{}, len(tiers))
	seenGroups := make(map[string]struct{}, len(tiers))
	sorted := make([]VipTier, len(tiers))
	copy(sorted, tiers)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].MinSpend < sorted[j-1].MinSpend; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	prevRatio := -1.0
	for i, tier := range sorted {
		if strings.TrimSpace(tier.Key) == "" {
			return errors.New("vip tier key must not be empty")
		}
		if strings.TrimSpace(tier.Group) == "" {
			return errors.New("vip tier group must not be empty: " + tier.Key)
		}
		if _, ok := seenKeys[tier.Key]; ok {
			return errors.New("duplicate vip tier key: " + tier.Key)
		}
		seenKeys[tier.Key] = struct{}{}
		if _, ok := seenGroups[tier.Group]; ok {
			return errors.New("duplicate vip tier group: " + tier.Group)
		}
		seenGroups[tier.Group] = struct{}{}
		if tier.MinSpend <= 0 {
			return errors.New("vip tier min_spend must be greater than 0: " + tier.Key)
		}
		if i > 0 && tier.MinSpend == sorted[i-1].MinSpend {
			return errors.New("vip tier min_spend must be strictly increasing: " + tier.Key)
		}
		if !ratio_setting.ContainsGroupRatio(tier.Group) {
			return errors.New("vip tier group has no group ratio configured: " + tier.Group)
		}
		ratio := ratio_setting.GetGroupRatio(tier.Group)
		if prevRatio >= 0 && ratio > prevRatio {
			return fmt.Errorf("vip tier %s must not price above a cheaper tier: group ratio %v > %v", tier.Key, ratio, prevRatio)
		}
		prevRatio = ratio
	}
	return nil
}

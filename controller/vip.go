package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

type AdminSetUserVipTierRequest struct {
	Tier   string `json:"tier"`
	Days   int    `json:"days"`
	Locked bool   `json:"locked"`
}

// GetVipTiers exposes the ladder so both the user badge and the admin editor
// can render thresholds without duplicating them.
func GetVipTiers(c *gin.Context) {
	vipSetting := setting.GetVipSetting()
	common.ApiSuccess(c, gin.H{
		"enabled":              vipSetting.Enabled,
		"auto_promote_enabled": vipSetting.AutoPromoteEnabled,
		"window_days":          setting.VipWindowDays(),
		"tiers":                setting.GetVipTiers(),
		"quota_per_unit":       common.QuotaPerUnit,
	})
}

func GetVipSelf(c *gin.Context) {
	status, err := model.GetUserVipStatus(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func AdminGetUserVip(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	status, err := model.GetUserVipStatus(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func AdminSetUserVipTier(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	var req AdminSetUserVipTierRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Tier == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Days < 0 {
		common.ApiErrorMsg(c, "有效天数不能为负数")
		return
	}
	group, err := model.SetUserVipTier(userId, req.Tier, req.Days, req.Locked)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"group": group})
}

func AdminClearUserVipTier(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	group, err := model.ClearUserVipTier(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"group": group})
}

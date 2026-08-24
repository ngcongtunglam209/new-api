package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// siteverify is a blocking call on every login, register, password reset and
// check-in request, so it must never inherit http.DefaultClient's lack of a
// timeout.
var turnstileClient = &http.Client{Timeout: 10 * time.Second}

type turnstileCheckResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func TurnstileCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.TurnstileCheckEnabled {
			c.Next()
			return
		}
		response := c.Query("turnstile")
		if response == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Turnstile token 为空",
			})
			c.Abort()
			return
		}
		if err := verifyTurnstileToken(response, c.ClientIP()); err != nil {
			// The failure detail can expose the secret key state and the raw
			// upstream body, so only the log receives it.
			common.SysError("Turnstile verification failed: " + err.Error())
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Turnstile 校验失败，请刷新重试！",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func verifyTurnstileToken(token string, remoteIP string) error {
	if common.TurnstileSecretKey == "" {
		return fmt.Errorf("turnstile secret key is not configured")
	}
	form := url.Values{
		"secret":   {common.TurnstileSecretKey},
		"response": {token},
	}
	// Cloudflare rejects the token when remoteip does not match the address
	// that solved the challenge. Behind a reverse proxy without trusted proxy
	// configuration ClientIP() is the proxy address, so loopback values are
	// dropped instead of guaranteeing a mismatch.
	if remoteIP != "" && remoteIP != "::1" && remoteIP != "127.0.0.1" {
		form.Set("remoteip", remoteIP)
	}
	rawRes, err := turnstileClient.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", form)
	if err != nil {
		return err
	}
	defer rawRes.Body.Close()
	if rawRes.StatusCode != http.StatusOK {
		return fmt.Errorf("siteverify returned status %d", rawRes.StatusCode)
	}
	var res turnstileCheckResponse
	if err := common.DecodeJson(rawRes.Body, &res); err != nil {
		return err
	}
	if !res.Success {
		if len(res.ErrorCodes) == 0 {
			return fmt.Errorf("siteverify rejected the token")
		}
		return fmt.Errorf("siteverify rejected the token: %s", strings.Join(res.ErrorCodes, ", "))
	}
	return nil
}

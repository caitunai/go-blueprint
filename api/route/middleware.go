package route

import (
	"crypto/rsa"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/caitunai/go-blueprint/api/base"
	"github.com/caitunai/go-blueprint/db"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	publicKey         *rsa.PublicKey
	oauthCallbackPath = "oauth/path/to/callback"
)

func InitMiddleware() {
	publicKeyByte, err := os.ReadFile(viper.GetString("oauth.publicKeyPath"))
	if err != nil {
		log.Error().Err(err).Msg("read oauth public key failed")
		return
	}
	publicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyByte)
	if err != nil {
		log.Error().Err(err).Msg("parse oauth public key failed")
		return
	}
}

func authorized(c *base.Context) {
	uid := c.GetUint("uid")
	if uid == 0 {
		login(c)
		c.Abort()
		return
	}

	c.Next()
}

func AttemptAuth() base.HandlerFunc {
	return func(c *base.Context) {
		var uid uint64
		id, _ := c.DecodeCookie("session_id")
		if id != "" {
			uid, _ = strconv.ParseUint(id, 10, 64)
		}
		if uid == 0 {
			bearerToken := c.GetHeader("Authorization")
			bearerToken = strings.TrimPrefix(bearerToken, "Bearer")
			bearerToken = strings.TrimSpace(bearerToken)
			if bearerToken != "" {
				var accountID uint64
				token, err := jwt.Parse(bearerToken, func(token *jwt.Token) (any, error) {
					if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
						return nil, errors.New("sign method error")
					}
					if publicKey == nil {
						return nil, errors.New("jwt public key not found")
					}
					return publicKey, nil
				})
				if err != nil {
					log.Error().Err(err).Msgf("parse token error: %s", bearerToken)
					accountID = 0
				} else {
					sub, err := token.Claims.GetSubject()
					if err != nil {
						log.Error().Err(err).Msg("get token id error")
					}
					accountID, _ = strconv.ParseUint(sub, 10, 64)
				}
				if accountID > 0 {
					u, err := db.RegisterUser(c.Request.Context(), uint(accountID))
					if err != nil {
						uid = 0
					} else if u != nil {
						c.SetUser(u)
						uid = uint64(u.ID)
					}
				}
			}
		}
		if uid == 0 && !c.IsWechatMiniProgram() {
			// is WeChat H5 but not WeChat program
			ag := strings.ToLower(c.GetHeader("user-agent"))
			isWechat := strings.Contains(ag, "micromessenger")
			isCallback := strings.Contains(c.Request.URL.Path, oauthCallbackPath)
			if isWechat && !isCallback {
				login(c)
				c.Abort()
				return
			}
		}

		c.Set("uid", uint(uid))
		c.Next()
	}
}

func AuthorizedAllowSpider() base.HandlerFunc {
	return func(c *base.Context) {
		ag := strings.ToLower(c.GetHeader("user-agent"))
		if strings.Contains(ag, "twitterbot") ||
			c.Request.Method == "GET" ||
			c.Request.Method == "HEAD" ||
			c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}
		// 处理登陆逻辑
		authorized(c)
	}
}

func login(c *base.Context) {
	c.Unauthorized("you should login", gin.H{
		"result": "you are not implement the login function",
	})
}

package base

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/caitunai/go-blueprint/db"
	"github.com/caitunai/go-blueprint/embed"
	"github.com/caitunai/go-blueprint/util"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var (
	ErrCookieURLParse = errors.New("parse url from configuration failed")
	ErrCookieDecode   = errors.New("decode cookie failed")
)

type Context struct {
	*gin.Context
	user *db.User
	cmx  sync.RWMutex
}

func (c *Context) getCSSJsFiles(entry string) (css, js []string) {
	if viper.GetString("mode") != "release" {
		return
	}
	manifest := embed.ParseManifest()
	css = manifest.GetCSSFiles(entry)
	js = manifest.GetJsFiles(entry)
	prefix := viper.GetString("url")
	for i, v := range css {
		css[i] = prefix + "/" + v
	}
	for i, v := range js {
		js[i] = prefix + "/" + v
	}
	return css, js
}

func (c *Context) Ok(body string) {
	c.String(http.StatusOK, body)
}

func (c *Context) Success(data gin.H) {
	c.JSON(http.StatusOK, gin.H{
		"status":  0,
		"message": "ok",
		"data":    data,
	})
}

// Error response with http status 500
//
// code the error code
//
// message the message of this error
//
// data the data map for this response
func (c *Context) Error(code int, message string, data gin.H) {
	c.Header("x-error-code", strconv.Itoa(code))
	c.JSON(http.StatusInternalServerError, gin.H{
		"status":  code,
		"message": message,
		"data":    data,
	})
}

// ErrorMessage response with http status 500
//
// message the error message to response
func (c *Context) ErrorMessage(message string) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"status":  http.StatusInternalServerError,
		"message": message,
	})
}

func (c *Context) Unauthorized(message string, data gin.H) {
	c.Header(
		"WWW-Authenticate",
		"Bearer error=\"invalid_token\", error_description=\"The token is invalid, request with a new token\"",
	)
	c.JSON(http.StatusUnauthorized, gin.H{
		"status":  http.StatusUnauthorized,
		"message": message,
		"data":    data,
	})
}

func (c *Context) Forbidden(message string, data gin.H) {
	c.JSON(http.StatusForbidden, gin.H{
		"status":  http.StatusForbidden,
		"message": message,
		"data":    data,
	})
}

func (c *Context) BadRequest(message string, data gin.H) {
	c.JSON(http.StatusBadRequest, gin.H{
		"status":  http.StatusBadRequest,
		"message": message,
		"data":    data,
	})
}

func (c *Context) ErrorForm(message string, data gin.H) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{
		"status":  http.StatusUnprocessableEntity,
		"message": message,
		"data":    data,
	})
}

func (c *Context) NotFound(message string, data gin.H) {
	c.JSON(http.StatusNotFound, gin.H{
		"status":  http.StatusNotFound,
		"message": message,
		"data":    data,
	})
}

func (c *Context) PayRequired(message string, data gin.H) {
	c.JSON(http.StatusPaymentRequired, gin.H{
		"status":  http.StatusPaymentRequired,
		"message": message,
		"data":    data,
	})
}

func (c *Context) SendCookie(key, value string, second int) error {
	link, err := url.Parse(viper.GetString("url"))
	if err != nil {
		return errors.Join(err, ErrCookieURLParse)
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		key,
		util.Encrypt([]byte(viper.GetString("key")), value),
		second,
		"/",
		link.Host,
		true,
		true,
	)
	return nil
}

func (c *Context) DecodeCookie(key string) (string, error) {
	cookie, err := c.Cookie(key)
	if err != nil {
		return "", errors.Join(err, ErrCookieDecode)
	}

	if cookie != "" {
		id := util.Decrypt([]byte(viper.GetString("key")), cookie)
		return id, nil
	}
	return "", nil
}

func (c *Context) LoginUser() *db.User {
	// 读锁并发安全
	c.cmx.RLock()
	if c.user != nil {
		defer c.cmx.RUnlock()
		return c.user
	}
	c.cmx.RUnlock()

	// 数据更新，会加写锁
	c.SetUser(db.GetUser(c.Request.Context(), c.GetUint("uid")))

	// 返回赋值需要加读锁
	c.cmx.RLock()
	defer c.cmx.RUnlock()
	return c.user
}

func (c *Context) SetUser(u *db.User) {
	// 写锁并发安全
	c.cmx.Lock()
	defer c.cmx.Unlock()
	c.user = u
}

func (c *Context) IsWechatMiniProgram() bool {
	return strings.Contains(c.GetHeader("referer"), "https://servicewechat.com")
}

func (c *Context) GetWechatAppID() string {
	refererList := strings.Split(c.GetHeader("referer"), "/")
	appid := ""
	if len(refererList) > 3 {
		appid = refererList[3]
	}
	return appid
}

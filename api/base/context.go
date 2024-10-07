package base

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/caitunai/go-blueprint/db"
	"github.com/caitunai/go-blueprint/embed"
	"github.com/caitunai/go-blueprint/util"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var ErrCookieDecode = errors.New("decode cookie failed")

const (
	HTTP  = "http"
	HTTPS = "https"
)

type Context struct {
	*gin.Context
}

func (c *Context) Scheme() string {
	// Can't use `r.Request.URL.Scheme`
	// See: https://groups.google.com/forum/#!topic/golang-nuts/pMUkBlQBDF0
	if c.Request.TLS != nil {
		return HTTPS
	}
	if scheme := c.GetHeader("X-Forwarded-Proto"); scheme != "" {
		return scheme
	}
	if scheme := c.GetHeader("X-Forwarded-Protocol"); scheme != "" {
		return scheme
	}
	if ssl := c.GetHeader("X-Forwarded-Ssl"); ssl == "on" {
		return HTTPS
	}
	if scheme := c.GetHeader("X-Url-Scheme"); scheme != "" {
		return scheme
	}
	return HTTP
}

func (c *Context) Port() string {
	port := c.GetHeader("X-Forwarded-Port")
	if port == "" {
		port = c.Request.URL.Port()
	}
	if port == "" {
		port = "80"
	}
	return port
}

func (c *Context) Origin() string {
	scheme := c.Scheme()
	port := c.Port()
	if scheme == HTTP || port == "80" {
		port = ""
	}
	if scheme == HTTPS || port == "443" {
		port = ""
	}
	if c.Request.Host == "" {
		return viper.GetString("url")
	}
	if port == "" {
		return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}
	return fmt.Sprintf("%s://%s:%s", scheme, c.Request.Host, port)
}

func (c *Context) getCSSJsFiles(entry string) (css, js []string) {
	if viper.GetString("mode") != "release" {
		return
	}
	manifest := embed.ParseManifest()
	css = manifest.GetCSSFiles(entry)
	js = manifest.GetJsFiles(entry)
	prefix := c.Origin()
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
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		key,
		util.Encrypt([]byte(viper.GetString("key")), value),
		second,
		"/",
		"",
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

func (c *Context) GetUserID() uint {
	return c.GetUint("uid")
}

func (c *Context) LoginUser() *db.User {
	u, ok := c.Get("user")
	if ok {
		return u.(*db.User)
	}
	qu := db.GetUser(c.Request.Context(), c.GetUserID())
	c.SetUser(qu)
	return qu
}

func (c *Context) SetUser(u *db.User) {
	c.Set("user", u)
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

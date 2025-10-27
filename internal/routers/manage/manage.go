package manage

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/logger"
	"github.com/gocronx-team/gocron/internal/modules/utils"
)

func Slack(c *gin.Context) {
	settingModel := new(models.Setting)
	slack, err := settingModel.Slack()
	jsonResp := utils.JsonResponse{}
	var result string
	if err != nil {
		logger.Error(err)
		result = jsonResp.Success(utils.SuccessContent, nil)
	} else {
		result = jsonResp.Success(utils.SuccessContent, slack)
	}
	c.String(http.StatusOK, result)
}

func UpdateSlack(c *gin.Context) {
	var form UpdateSlackForm
	if err := c.ShouldBindJSON(&form); err != nil {
		json := utils.JsonResponse{}
		result := json.CommonFailure("表单验证失败, 请检测输入")
		c.String(http.StatusOK, result)
		return
	}
	
	settingModel := new(models.Setting)
	err := settingModel.UpdateSlack(form.Url, form.Template)
	result := utils.JsonResponseByErr(err)
	c.String(http.StatusOK, result)
}

func CreateSlackChannel(c *gin.Context) {
	var form CreateSlackChannelForm
	if err := c.ShouldBindJSON(&form); err != nil {
		json := utils.JsonResponse{}
		result := json.CommonFailure("表单验证失败, 请检测输入")
		c.String(http.StatusOK, result)
		return
	}
	
	settingModel := new(models.Setting)
	var result string
	if settingModel.IsChannelExist(form.Channel) {
		jsonResp := utils.JsonResponse{}
		result = jsonResp.CommonFailure("Channel已存在")
	} else {
		_, err := settingModel.CreateChannel(form.Channel)
		result = utils.JsonResponseByErr(err)
	}
	c.String(http.StatusOK, result)
}

func RemoveSlackChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	settingModel := new(models.Setting)
	_, err := settingModel.RemoveChannel(id)
	result := utils.JsonResponseByErr(err)
	c.String(http.StatusOK, result)
}

// endregion

// region 邮件
func Mail(c *gin.Context) {
	settingModel := new(models.Setting)
	mail, err := settingModel.Mail()
	jsonResp := utils.JsonResponse{}
	var result string
	if err != nil {
		logger.Error(err)
		result = jsonResp.Success(utils.SuccessContent, nil)
	} else {
		result = jsonResp.Success("", mail)
	}
	c.String(http.StatusOK, result)
}

type MailServerForm struct {
	Host     string `binding:"required,max=100"`
	Port     int    `binding:"required,min=1,max=65535"`
	User     string `binding:"required,email,max=64"`
	Password string `binding:"required,max=64"`
}

// CreateMailUserForm 创建邮件用户表单
type CreateMailUserForm struct {
	Username string `json:"username" binding:"required,max=50"`
	Email    string `json:"email" binding:"required,email,max=100"`
}

// UpdateSlackForm 更新Slack配置表单
type UpdateSlackForm struct {
	Url      string `json:"url" binding:"required,url,max=200"`
	Template string `json:"template" binding:"required"`
}

// UpdateWebHookForm 更新WebHook配置表单
type UpdateWebHookForm struct {
	Url      string `json:"url" binding:"required,url,max=200"`
	Template string `json:"template" binding:"required"`
}

// CreateSlackChannelForm 创建Slack频道表单
type CreateSlackChannelForm struct {
	Channel string `json:"channel" binding:"required,max=50"`
}

func UpdateMail(c *gin.Context) {
	var form MailServerForm
	if err := c.ShouldBindJSON(&form); err != nil {
		json := utils.JsonResponse{}
		result := json.CommonFailure("表单验证失败, 请检测输入")
		c.String(http.StatusOK, result)
		return
	}
	
	jsonByte, _ := json.Marshal(form)
	settingModel := new(models.Setting)
	template := strings.TrimSpace(c.Query("template"))
	err := settingModel.UpdateMail(string(jsonByte), template)
	result := utils.JsonResponseByErr(err)
	c.String(http.StatusOK, result)
}

func CreateMailUser(c *gin.Context) {
	var form CreateMailUserForm
	if err := c.ShouldBindJSON(&form); err != nil {
		json := utils.JsonResponse{}
		result := json.CommonFailure("表单验证失败, 请检测输入")
		c.String(http.StatusOK, result)
		return
	}
	
	settingModel := new(models.Setting)
	_, err := settingModel.CreateMailUser(form.Username, form.Email)
	result := utils.JsonResponseByErr(err)
	c.String(http.StatusOK, result)
}

func RemoveMailUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	settingModel := new(models.Setting)
	_, err := settingModel.RemoveMailUser(id)
	result := utils.JsonResponseByErr(err)
	c.String(http.StatusOK, result)
}

func WebHook(c *gin.Context) {
	settingModel := new(models.Setting)
	webHook, err := settingModel.Webhook()
	jsonResp := utils.JsonResponse{}
	var result string
	if err != nil {
		logger.Error(err)
		result = jsonResp.Success(utils.SuccessContent, nil)
	} else {
		result = jsonResp.Success("", webHook)
	}
	c.String(http.StatusOK, result)
}

func UpdateWebHook(c *gin.Context) {
	var form UpdateWebHookForm
	if err := c.ShouldBindJSON(&form); err != nil {
		json := utils.JsonResponse{}
		result := json.CommonFailure("表单验证失败, 请检测输入")
		c.String(http.StatusOK, result)
		return
	}
	
	settingModel := new(models.Setting)
	err := settingModel.UpdateWebHook(form.Url, form.Template)
	result := utils.JsonResponseByErr(err)
	c.String(http.StatusOK, result)
}

// endregion

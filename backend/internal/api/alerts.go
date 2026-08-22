package api

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"dockeradmin/internal/model"
)

func (s *Server) ruleList(c *gin.Context) {
	respondData(c, http.StatusOK, s.store.List())
}

func (s *Server) ruleCreate(c *gin.Context) {
	var rule model.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		respondErr(c, http.StatusBadRequest, "invalid_json", "请求体不是合法 JSON", nil)
		return
	}
	if errs := model.ValidateRule(&rule); len(errs) > 0 {
		respondErr(c, http.StatusUnprocessableEntity, "validation_error", "规则校验失败", errs)
		return
	}
	created, err := s.store.Create(&rule)
	if err != nil {
		s.log.Error("persist rule failed", "err", err)
		respondErr(c, http.StatusInternalServerError, "persist_error", "规则持久化失败", nil)
		return
	}
	c.Header("Location", "/api/alert-rules/"+created.ID)
	respondData(c, http.StatusCreated, created)
}

func (s *Server) ruleUpdate(c *gin.Context) {
	id := c.Param("id")
	var rule model.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		respondErr(c, http.StatusBadRequest, "invalid_json", "请求体不是合法 JSON", nil)
		return
	}
	if errs := model.ValidateRule(&rule); len(errs) > 0 {
		respondErr(c, http.StatusUnprocessableEntity, "validation_error", "规则校验失败", errs)
		return
	}
	updated, found, err := s.store.Update(id, &rule)
	if err != nil {
		s.log.Error("persist rule failed", "err", err)
		respondErr(c, http.StatusInternalServerError, "persist_error", "规则持久化失败", nil)
		return
	}
	if !found {
		respondErr(c, http.StatusNotFound, "not_found", "规则不存在", nil)
		return
	}
	respondData(c, http.StatusOK, updated)
}

func (s *Server) ruleDelete(c *gin.Context) {
	found, err := s.store.Delete(c.Param("id"))
	if err != nil {
		s.log.Error("persist delete failed", "err", err)
		respondErr(c, http.StatusInternalServerError, "persist_error", "规则持久化失败", nil)
		return
	}
	if !found {
		respondErr(c, http.StatusNotFound, "not_found", "规则不存在", nil)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) eventList(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 1 && parsed <= 200 {
			limit = parsed
		}
	}
	respondData(c, http.StatusOK, s.store.Events(limit))
}

func (s *Server) mockWebhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 64*1024))
	if err != nil {
		respondErr(c, http.StatusBadRequest, "read_error", "读取请求体失败", nil)
		return
	}
	s.receiptsMu.Lock()
	s.receiptSeq++
	receipt := model.WebhookReceipt{ID: s.receiptSeq, ReceivedAt: time.Now(), Payload: string(body)}
	s.receipts = append([]model.WebhookReceipt{receipt}, s.receipts...)
	if len(s.receipts) > 100 {
		s.receipts = s.receipts[:100]
	}
	s.receiptsMu.Unlock()
	s.log.Info("mock webhook received", "id", receipt.ID, "bytes", len(body))
	respondData(c, http.StatusOK, gin.H{"received": true, "id": receipt.ID})
}

func (s *Server) mockReceipts(c *gin.Context) {
	s.receiptsMu.Lock()
	out := make([]model.WebhookReceipt, len(s.receipts))
	copy(out, s.receipts)
	s.receiptsMu.Unlock()
	respondData(c, http.StatusOK, out)
}

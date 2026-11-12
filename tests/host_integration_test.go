package router

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file is injected into an isolated host checkout by run-integration.sh.
// It deliberately uses production handlers and accounting, without mocked JS
// hooks or a fake BillingSettler. All credentials and provider data are fake.
func TestGeminiDecisionsIntegration(t *testing.T) {
	const initialQuota = 1_000_000
	const reservedQuota = 262_144 // The fixture's $2 / million token price gives one quota per token.
	const validBody = `{"model":"gemini-decision-1","input":"Choose a plan","questions":{"pick":{"type":"choice","instructions":"Pick one","criteria":{"A":"First","B":"Second"}}}}`
	const answers = `"answers":{"pick":{"type":"choice","choice":"A","probabilities":{"A":0.8,"B":0.2},"confidence":0.8}}`
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	source, err := os.ReadFile(os.Getenv("GEMINI_DECISIONS_PLUGIN_SOURCE"))
	require.NoError(t, err, "GEMINI_DECISIONS_PLUGIN_SOURCE must name plugin.js")

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMemory, previousRedis := common.MemoryCacheEnabled, common.RedisEnabled
	previousBatch, previousLog, previousRetry := common.BatchUpdateEnabled, common.LogConsumeEnabled, common.RetryTimes
	previousQuotaPerUnit := common.QuotaPerUnit
	previousTransport := http.DefaultTransport
	previousMaster, previousSQLitePath := common.IsMasterNode, common.SQLitePath
	previousDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled = false, false, false
	common.LogConsumeEnabled, common.RetryTimes, common.QuotaPerUnit = true, 0, 500_000
	common.IsMasterNode = false
	common.SQLitePath = "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	t.Setenv("SQL_DSN", "local")
	t.Setenv("LOG_SQL_DSN", "")
	// InitDB also initializes the host's dialect-specific SQL column names.
	require.NoError(t, model.InitDB())
	database := model.DB
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Ability{}, &model.Task{}, &model.Log{}))
	model.DB, model.LOG_DB = database, database
	t.Cleanup(func() {
		jsplugin.DefaultRegistry.Unregister("gemini-decisions")
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.MemoryCacheEnabled, common.RedisEnabled = previousMemory, previousRedis
		common.BatchUpdateEnabled, common.LogConsumeEnabled, common.RetryTimes = previousBatch, previousLog, previousRetry
		common.QuotaPerUnit, http.DefaultTransport = previousQuotaPerUnit, previousTransport
		common.IsMasterNode, common.SQLitePath = previousMaster, previousSQLitePath
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		_ = sqlDB.Close()
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.plugin_billing_expr":  `{"gemini-decisions::gemini-decision-1":"u(\"input_tokens\") * 2 / 1000000"}`,
		"quota_setting.pre_consume_multiplier": "1",
		"quota_setting.trust_quota_usd":        "0",
		"group_ratio_setting.group_ratio":      `{"default":1}`,
	}))
	user := model.User{Username: "gemini-integration", Status: common.UserStatusEnabled, Group: "default", Quota: initialQuota, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, database.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "fakegatewaytoken", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: initialQuota}
	require.NoError(t, database.Create(&token).Error)

	var calls atomic.Int32
	var gatewayUpstream atomic.Bool
	var providerStatus atomic.Int32
	providerStatus.Store(http.StatusOK)
	var providerBody atomic.Value
	providerBody.Store(`{` + answers + `,"usage":{"total_input_tokens":100,"total_output_tokens":0,"total_tokens":100}}`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method)
		if gatewayUpstream.Load() {
			assert.Equal(t, "/gemini-decisions/v1beta/decisions", r.URL.Path)
			assert.Equal(t, "Bearer fakegooglekey", r.Header.Get("Authorization"))
			assert.Empty(t, r.Header.Get("x-goog-api-key"))
		} else {
			assert.Equal(t, "/v1beta/decisions", r.URL.Path)
			assert.Equal(t, "fakegooglekey", r.Header.Get("x-goog-api-key"))
			assert.Empty(t, r.Header.Get("Authorization"), "gateway token must not reach Google")
		}
		assert.Empty(t, r.Header.Get("Cookie"))
		assert.Empty(t, r.URL.RawQuery, "client query strings must not carry credentials upstream")
		body, readErr := io.ReadAll(r.Body)
		require.NoError(t, readErr)
		assert.JSONEq(t, validBody, string(body), "Google's native payload must retain its shape")
		var heldUser model.User
		var heldToken model.Token
		require.NoError(t, database.First(&heldUser, user.Id).Error)
		require.NoError(t, database.First(&heldToken, token.Id).Error)
		assert.Equal(t, initialQuota-reservedQuota, heldUser.Quota, "wallet reservation must precede the provider call")
		assert.Equal(t, initialQuota-reservedQuota, heldToken.RemainQuota, "token reservation must precede the provider call")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(providerStatus.Load()))
		_, _ = io.WriteString(w, providerBody.Load().(string))
	}))
	defer upstream.Close()

	// Restrict the production relay transport to the mock's exact address.
	// An accidental hard-coded Google URL fails locally instead of contacting it.
	allowedAddress := strings.TrimPrefix(upstream.URL, "http://")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != allowedAddress {
			return nil, fmt.Errorf("integration test blocked external network target %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	http.DefaultTransport = transport
	service.InitHttpClient()
	defer transport.CloseIdleConnections()
	setting := `{"task_plugin_key":"gemini-decisions"}`
	channel := model.Channel{Type: constant.ChannelTypeTaskPlugin, Name: "gemini-local-mock", Key: "fakegooglekey", BaseURL: &upstream.URL,
		Status: common.ChannelStatusEnabled, Models: "gemini-decision-1", Group: "default", Setting: &setting}
	require.NoError(t, channel.Insert())
	_, err = jsplugin.DefaultRegistry.Register(string(source), jsplugin.Options{})
	require.NoError(t, err, "must compile in the host's real moejs runtime")
	engine := gin.New()
	// Use the host's normal registration order so static Gemini, Responses,
	// video and task routes participate in plugin routing conflict checks.
	SetRelayRouter(engine)
	SetTaskPluginProtocolRouter(engine)
	SetVideoRouter(engine)
	SetTaskRouter(engine)
	engine.NoRoute(SetPluginRouter(engine))
	require.Empty(t, jsplugin.DefaultRegistry.RoutingErrors())

	request := func(path, body, authorization, googleKey string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", authorization)
		req.Header.Set("x-goog-api-key", googleKey)
		req.Header.Set("Cookie", "session=fakeclientsession")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		return recorder
	}
	reset := func() {
		require.NoError(t, database.Model(&model.User{}).Where("id = ?", user.Id).Updates(map[string]any{"quota": initialQuota, "used_quota": 0}).Error)
		require.NoError(t, database.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]any{"remain_quota": initialQuota, "used_quota": 0}).Error)
		require.NoError(t, database.Where("1 = 1").Delete(&model.Task{}).Error)
		require.NoError(t, database.Where("1 = 1").Delete(&model.Log{}).Error)
	}
	assertQuota := func(want int) {
		// Host failure refunds intentionally run asynchronously.
		require.Eventually(t, func() bool {
			var gotUser model.User
			var gotToken model.Token
			return database.First(&gotUser, user.Id).Error == nil && database.First(&gotToken, token.Id).Error == nil &&
				gotUser.Quota == initialQuota-want && gotToken.RemainQuota == initialQuota-want && gotToken.UsedQuota == want
		}, 3*time.Second, 10*time.Millisecond)
	}
	for _, tc := range []struct {
		name, path, usage string
		charge            int
	}{
		{"native path", "/v1beta/decisions", `{"total_input_tokens":100,"total_output_tokens":0,"total_tokens":100}`, 100},
		{"namespaced path", "/gemini-decisions/v1beta/decisions", `{"total_input_tokens":100,"total_output_tokens":0,"total_tokens":100}`, 100},
		{"explicit zero usage", "/v1beta/decisions", `{"total_input_tokens":0,"total_output_tokens":0,"total_tokens":0}`, 0},
		{"missing usage retains reserve", "/v1beta/decisions", `{}`, reservedQuota},
		{"negative usage retains reserve", "/v1beta/decisions", `{"total_input_tokens":-1}`, reservedQuota},
		{"fractional usage retains reserve", "/v1beta/decisions", `{"total_input_tokens":0.5}`, reservedQuota},
		{"oversized usage retains reserve", "/v1beta/decisions", `{"total_input_tokens":262145}`, reservedQuota},
		{"string usage retains reserve", "/v1beta/decisions", `{"total_input_tokens":"100"}`, reservedQuota},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reset()
			providerStatus.Store(http.StatusOK)
			body := `{` + answers + `,"usage":` + tc.usage + `}`
			providerBody.Store(body)
			before := calls.Load()
			response := request(tc.path, validBody, "Bearer fakegatewaytoken", "untrustedclientkey")
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assert.JSONEq(t, body, response.Body.String())
			assert.Equal(t, before+1, calls.Load())
			assertQuota(tc.charge)
			var task model.Task
			require.NoError(t, database.First(&task).Error)
			assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), task.Status)
			assert.Equal(t, tc.charge, task.Quota)
			assert.True(t, task.PrivateData.ResultDiscarded)
			assert.False(t, task.ResultRetrievable())
			assert.Empty(t, task.Data, "retainResult:false must not store the submitted documents or answers")
			var logs []model.Log
			require.NoError(t, database.Where("type = ?", model.LogTypeConsume).Find(&logs).Error)
			require.Len(t, logs, 1)
			assert.Equal(t, tc.charge, logs[0].Quota)
		})
	}
	t.Run("New API upstream uses namespaced route and Bearer", func(t *testing.T) {
		reset()
		gatewayUpstream.Store(true)
		defer gatewayUpstream.Store(false)
		require.NoError(t, database.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("type", constant.ChannelTypeNewAPI).Error)
		defer database.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("type", constant.ChannelTypeTaskPlugin)
		providerBody.Store(`{` + answers + `,"usage":{"total_input_tokens":100}}`)
		before := calls.Load()
		response := request("/v1beta/decisions", validBody, "Bearer fakegatewaytoken", "")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, before+1, calls.Load())
		assertQuota(100)
	})
	for _, tc := range []struct {
		name, body, auth, key string
		status                int
	}{
		{"missing gateway token", validBody, "", "", http.StatusUnauthorized},
		{"invalid gateway token", validBody, "Bearer fakeinvalidtoken", "", http.StatusUnauthorized},
		{"Google key is not gateway authorization", validBody, "", "fakegatewaytoken", http.StatusUnauthorized},
		{"malformed JSON", `{"model":`, "Bearer fakegatewaytoken", "", http.StatusBadRequest},
		{"unsupported model", strings.Replace(validBody, "gemini-decision-1", "gemini-unknown", 1), "Bearer fakegatewaytoken", "", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reset()
			before := calls.Load()
			response := request("/v1beta/decisions", tc.body, tc.auth, tc.key)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
			assert.Equal(t, before, calls.Load(), "rejected requests must not reach Google")
			assertQuota(0)
		})
	}
	t.Run("upstream failure refunds both balances", func(t *testing.T) {
		reset()
		providerStatus.Store(http.StatusBadRequest)
		providerBody.Store(`{"error":{"code":400,"message":"fake upstream failure","status":"INVALID_ARGUMENT"}}`)
		before := calls.Load()
		response := request("/v1beta/decisions", validBody, "Bearer fakegatewaytoken", "")
		assert.GreaterOrEqual(t, response.Code, 400, response.Body.String())
		assert.Equal(t, before+1, calls.Load())
		assertQuota(0)
		assert.NotContains(t, response.Body.String(), "fakegooglekey")
		assert.NotContains(t, response.Body.String(), upstream.URL)
	})
	t.Run("invalid successful upstream response refunds both balances", func(t *testing.T) {
		reset()
		providerStatus.Store(http.StatusOK)
		providerBody.Store(`{"answers":{},"usage":{"total_input_tokens":100}}`)
		before := calls.Load()
		response := request("/v1beta/decisions", validBody, "Bearer fakegatewaytoken", "")
		assert.GreaterOrEqual(t, response.Code, 400, response.Body.String())
		assert.Equal(t, before+1, calls.Load())
		assertQuota(0)
	})
	t.Run("insufficient quota prevents provider call", func(t *testing.T) {
		reset()
		require.NoError(t, database.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", reservedQuota-1).Error)
		before := calls.Load()
		response := request("/v1beta/decisions", validBody, "Bearer fakegatewaytoken", "")
		assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
		assert.Equal(t, before, calls.Load())
	})
}

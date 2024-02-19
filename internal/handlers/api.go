package handlers

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-session/session/v3"
	"github.com/nexodus-io/nexodus/internal/email"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/envfm"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/signalbus"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/url"
	"strconv"

	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/open-policy-agent/opa/storage"

	"github.com/nexodus-io/nexodus/internal/util"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/fflags"
	"github.com/nexodus-io/nexodus/internal/ipam"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("github.com/nexodus-io/nexodus/internal/handlers")
}

type API struct {
	logger         *zap.SugaredLogger
	db             *gorm.DB
	ipam           ipam.IPAM
	defaultZoneID  uuid.UUID
	fflags         *fflags.FFlags
	transaction    database.TransactionFunc
	dialect        database.Dialect
	store          storage.Store
	signalBus      signalbus.SignalBus
	Redis          *redis.Client
	sessionManager *session.Manager
	fetchManager   fetchmgr.FetchManager
	onlineTracker  *DeviceTracker
	URL            string
	URLParsed      *url.URL
	PrivateKey     *rsa.PrivateKey
	Certificates   []*x509.Certificate
	SmtpServer     email.SmtpServer
	SmtpFrom       string
	caKeyPair      CertificateKeyPair
	FrontendURL    string
}

func NewAPI(
	parent context.Context,
	logger *zap.SugaredLogger,
	db *gorm.DB,
	ipam ipam.IPAM,
	fflags *fflags.FFlags,
	store storage.Store,
	signalBus signalbus.SignalBus,
	redis *redis.Client,
	sessionManager *session.Manager,
	caKeyPair CertificateKeyPair,
) (*API, error) {

	fflags.RegisterEnvFlag("multi-organization", "NEXAPI_FFLAG_MULTI_ORGANIZATION", true)
	fflags.RegisterEnvFlag("security-groups", "NEXAPI_FFLAG_SECURITY_GROUPS", true)
	fflags.RegisterEnvFlag("devices", "NEXAPI_FFLAG_DEVICES", true)
	fflags.RegisterEnvFlag("sites", "NEXAPI_FFLAG_SITES", false)
	fflags.RegisterFlag("ca", func() bool {
		if !fflags.Flags["sites"]() {
			return false
		}
		if caKeyPair.Certificate == nil {
			return false
		}
		return true
	})

	ctx, span := tracer.Start(parent, "NewAPI")
	defer span.End()

	transactionFunc, dialect, err := database.GetTransactionFunc(db)
	if err != nil {
		return nil, err
	}

	fetchManager, err := envfm.New(logger.Desugar())
	if err != nil {
		return nil, err
	}

	onlineTracker, err := New(logger.Desugar())
	if err != nil {
		return nil, err
	}

	api := &API{
		logger:         logger,
		db:             db,
		ipam:           ipam,
		defaultZoneID:  uuid.Nil,
		fflags:         fflags,
		transaction:    transactionFunc,
		dialect:        dialect,
		store:          store,
		signalBus:      signalBus,
		Redis:          redis,
		sessionManager: sessionManager,
		fetchManager:   fetchManager,
		onlineTracker:  onlineTracker,
		caKeyPair:      caKeyPair,
	}

	if err := api.populateStore(ctx); err != nil {
		return nil, err
	}

	err = api.createDefaultIPamNamespace(ctx)
	if err != nil {
		return nil, err
	}

	return api, nil
}

func (api *API) Logger(ctx context.Context) *zap.SugaredLogger {
	return util.WithTrace(ctx, api.logger)
}

func (api *API) sendList(c *gin.Context, ctx context.Context, getList func(db *gorm.DB) (fetchmgr.ResourceList, error)) {
	db := api.db.WithContext(ctx)

	items, err := getList(db)
	if err != nil {
		api.SendInternalServerError(c, err)
		return
	}

	// For pagination
	c.Header("Access-Control-Expose-Headers", TotalCountHeader)
	c.Header(TotalCountHeader, strconv.Itoa(items.Len()))
	c.JSON(http.StatusOK, items)
}

func (api *API) SendInternalServerError(c *gin.Context, err error) {
	SendInternalServerError(c, api.logger, err)
}

func SendInternalServerError(c *gin.Context, logger *zap.SugaredLogger, err error) {
	ctx := c.Request.Context()
	util.WithTrace(ctx, logger).Errorw("internal server error", "error", err)

	result := models.InternalServerError{
		BaseError: models.BaseError{
			Error: "internal server error",
		},
	}
	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.HasTraceID() {
		result.TraceId = sc.TraceID().String()
	}
	c.JSON(http.StatusInternalServerError, result)
}
func (api *API) GetCurrentUserID(c *gin.Context) uuid.UUID {
	userId, found := c.Get(gin.AuthUserKey)
	if !found {
		api.SendInternalServerError(c, fmt.Errorf("no current user found"))
		panic("no current user found")
	}
	return userId.(uuid.UUID)
}

func (api *API) FlagCheck(c *gin.Context, name string) bool {
	enabled, err := api.fflags.GetFlag(c, name)
	if err != nil {
		api.SendInternalServerError(c, err)
		return false
	}
	if !enabled {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError(fmt.Sprintf("%s support is disabled", name)))
		return false
	}
	return enabled
}

func (api *API) createDefaultIPamNamespace(ctx context.Context) error {
	// Create namespaces and cidrs
	if err := api.ipam.CreateNamespace(ctx, defaultIPAMNamespace); err != nil {
		return fmt.Errorf("failed to create ipam namespace: %w", err)
	}
	if err := api.ipam.AssignCIDR(ctx, defaultIPAMNamespace, defaultIPAMv4Cidr); err != nil {
		return fmt.Errorf("can't assign default ipam v4 prefix: %w", err)
	}
	if err := api.ipam.AssignCIDR(ctx, defaultIPAMNamespace, defaultIPAMv6Cidr); err != nil {
		return fmt.Errorf("can't assign default ipam v6 prefix: %w", err)
	}
	return nil
}

func (api *API) SendEmail(message email.Message) error {
	if api.SmtpServer.HostPort == "" {
		return nil
	}
	message.From = api.SmtpFrom
	return email.Send(api.SmtpServer, message)
}

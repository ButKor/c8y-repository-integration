package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/kobu/dm-repo-integration/internal/model"
	"github.com/kobu/dm-repo-integration/pkg/c8yauth"
	est "github.com/kobu/dm-repo-integration/pkg/externalstorage"
	"github.com/kobu/dm-repo-integration/pkg/handlers"
	"github.com/labstack/echo/v4"
	"github.com/reubenmiller/go-c8y/pkg/c8y"
	"github.com/reubenmiller/go-c8y/pkg/microservice"
	"go.uber.org/zap"
)

// App represents the http server and c8y microservice application
type App struct {
	echoServer      *echo.Echo
	c8ymicroservice *microservice.Microservice
}

// NewApp initializes the microservice with default configuration and registers the microservice
func NewApp() *App {
	app := &App{}
	log.Printf("Application information: Version %s, branch %s, commit %s, buildTime %s", Version, Branch, Commit, BuildTime)

	customHTTPClient := retryablehttp.NewClient()
	opts := microservice.Options{
		HTTPClient: customHTTPClient.StandardClient(),
	}
	opts.AgentInformation = microservice.AgentInformation{
		SerialNumber: Commit,
		Revision:     Version,
		BuildTime:    BuildTime,
	}

	c8ymicroservice := microservice.NewDefaultMicroservice(opts)

	customHTTPClient.RetryMax = 2
	customHTTPClient.PrepareRetry = func(req *http.Request) error {
		// Update latest service user credentials
		if username, _, ok := req.BasicAuth(); ok {
			if tenant, username, found := strings.Cut(username, "/"); found {
				for _, serviceUser := range c8ymicroservice.Client.ServiceUsers {
					if serviceUser.Tenant == tenant && serviceUser.Username == username {
						slog.Info("Updating service user credentials for request.", "tenant", tenant, "userID", username)
						req.SetBasicAuth(tenant+"/"+username, serviceUser.Password)
						return nil
					}
				}
			}
		}
		return nil
	}

	customHTTPClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if resp.StatusCode == http.StatusTooManyRequests {
			return true, nil
		}

		// unauthorized errors can occurs if the service user's credentials are not up to date
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			slog.Info("Service user credentials are invalid, refreshing them.", "statusCode", resp.StatusCode)
			if serviceUsersErr := c8ymicroservice.Client.Microservice.SetServiceUsers(); serviceUsersErr != nil {
				slog.Error("Could not update service users list.", "err", serviceUsersErr)
			} else {
				slog.Info("Updated service users list")
			}
			return true, nil
		}

		if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
			return true, fmt.Errorf("unexpected HTTP status %s", resp.Status)
		}

		return false, nil
	}

	c8ymicroservice.Config.SetDefault("server.port", "80")
	// c8ymicroservice.RegisterMicroserviceAgent()
	app.c8ymicroservice = c8ymicroservice
	return app
}

func syncSubscriptionsWithTenantControllers(c *c8y.Client, estClient *est.ExternalStorageClient, fwControllers *FirmwareTenantControllers, ctxPath string) {
	subscriptions, _, _ := c.Application.GetCurrentApplicationSubscriptions(c.Context.BootstrapUserFromEnvironment())
	for _, user := range subscriptions.Users {
		tenant := user.Tenant
		if len(tenant) == 0 {
			slog.Warn("No tenant for for subscription user")
			continue
		}
		_, exists := fwControllers.Get(tenant)
		if exists {
			slog.Info("Controller already existing for tenant", "tenant", tenant)
			continue
		}
		currentTenant, _, err := c.Tenant.GetCurrentTenant(c.Context.ServiceUserContext(tenant, false))
		if err != nil {
			slog.Warn("Error while invoking /tenant/currentTenant. Skipping this tenant subscription", "err", err, "tenant", tenant)
		}
		domainName := currentTenant.DomainName
		// firmware controller for tenant does not exist, create and register it
		fc := FirmwareTenantController{
			tenantStore: &FirmwareTenantStore{
				FirmwareByName:         make(map[string]FirmwareStoreFwEntry),
				FirmwareVersionsByName: make(map[string][]FirmwareStoreVersionEntry),
			},
			ctx:            c.Context.ServiceUserContext(tenant, false),
			c8yClient:      c,
			estClient:      estClient,
			tenantId:       tenant,
			serviceBaseUrl: "https://" + domainName + "/service/" + ctxPath,
		}
		fwControllers.Register(fc)
		fwControllers.SyncTenantsWithIndexFiles([]string{tenant})
	}
}

func syncSubscriptionsWithTenantControllersPeriodically(c *c8y.Client, estClient *est.ExternalStorageClient, fwControllers *FirmwareTenantControllers, ctxPath string) {
	for {
		syncSubscriptionsWithTenantControllers(c, estClient, fwControllers, ctxPath)
		time.Sleep(60 * time.Second)
	}
}

func CreateStorageClientFromTenantOptions(application *microservice.Microservice) (est.ExternalStorageClient, error) {
	ctx := application.WithServiceUser(application.Client.TenantName)
	c8yClient := application.Client
	sProviderOptionCategory := "repoIntegrationFirmware"
	sProviderOptionKey := "storageProvider"
	storageProvider, _, err := c8yClient.TenantOptions.GetOption(ctx, sProviderOptionCategory, sProviderOptionKey)
	if err != nil {
		slog.Error(fmt.Sprintf("Could not read required storageProvider tenant option (category=%s, key=%s)", sProviderOptionCategory, sProviderOptionKey), "err", err)
		return nil, err
	}

	switch storageProvider.Value {
	case "awsS3":
		slog.Info("Detected desired storage account to be awsS3. Initializing client...")
		awsClient := &est.AWSClient{}
		if err := awsClient.Init(ctx, c8yClient); err != nil {
			slog.Error("Fatal problem while initializing AWS client", "err", err)
			return nil, err
		}
		return awsClient, nil
	case "azblob":
		slog.Info("Detected desired storage account to be azblob. Initializing client...")
		azClient := &est.AzClient{}
		if err := azClient.Init(ctx, c8yClient); err != nil {
			slog.Error("Fatal problem while initializing Azure client", "err", err)
			return nil, err
		}
		return azClient, nil
	default:
		slog.Error("Storage provider not supported", "err", err)
		return nil, errors.New("provided none or an unsupported storage provider. Make sure the tenant options align with documentation")
	}
}

// Run starts the microservice
func (a *App) Run() {
	application := a.c8ymicroservice
	application.Scheduler.Start()

	slog.Info("Tenant Info", "tenant", application.Client.TenantName)
	bUrl := application.Client.BaseURL
	slog.Info("Service BaseURL", "url", bUrl.Scheme+"://"+bUrl.Hostname()+"/service/"+application.Application.ContextPath)

	estClient, err := CreateStorageClientFromTenantOptions(application)
	if err != nil {
		slog.Error("Error while initiating the connection to the external storage. This is a fatal error, exiting in 10 seconds...", "error", err)
		time.Sleep(time.Second * 10)
		os.Exit(1)
	}

	// init Firmware Controllers
	tenantFwControllers := FirmwareTenantControllers{
		estClient:         estClient,
		tenantControllers: make(map[string]FirmwareTenantController),
	}
	// check registered tenants, create a Firmware Controller for each of them
	syncSubscriptionsWithTenantControllers(application.Client, &estClient, &tenantFwControllers, application.Application.ContextPath)
	// Start routine to periodically check for tenant subscriptions and add Firmware Controller for Each
	go syncSubscriptionsWithTenantControllersPeriodically(application.Client, &estClient, &tenantFwControllers, application.Application.ContextPath)
	// let firmware controller observe external storage
	// TODO make this configurable
	go tenantFwControllers.AutoObserve(600)

	// now start webserver
	if a.echoServer == nil {
		addr := ":" + application.Config.GetString("server.port")
		zap.S().Infof("starting http server on %s", addr)

		a.echoServer = echo.New()
		setDefaultContextHandler(a.echoServer, a.c8ymicroservice)
		provider := c8yauth.NewAuthProvider(application.Client)
		a.echoServer.Use(c8yauth.AuthenticationBasic(provider))
		a.echoServer.Use(c8yauth.AuthenticationBearer(provider))

		a.setRouters(&estClient)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		// Start server
		go func() {
			if err := a.echoServer.Start(addr); err != nil && err != http.ErrServerClosed {
				a.echoServer.Logger.Fatal("shutting down the server")
			}
		}()

		// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.echoServer.Shutdown(ctx); err != nil {
			a.echoServer.Logger.Fatal(err)
		}
	}
}

func setDefaultContextHandler(e *echo.Echo, c8yms *microservice.Microservice) {
	// Add Custom Context
	e.Use(func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := &model.RequestContext{
				Context:      c,
				Microservice: c8yms,
			}
			return h(cc)
		}
	})
}
func (a *App) setRouters(estClient *est.ExternalStorageClient) {
	server := a.echoServer
	handlers.RegisterFirmwareHandler(server, estClient)
	a.c8ymicroservice.AddHealthEndpointHandlers(server)
}

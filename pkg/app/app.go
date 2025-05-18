package app

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/kobu/repo-int/internal/model"
	"github.com/kobu/repo-int/pkg/aws"
	"github.com/kobu/repo-int/pkg/c8yauth"
	"github.com/kobu/repo-int/pkg/handlers"
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
	c8ymicroservice.RegisterMicroserviceAgent()
	app.c8ymicroservice = c8ymicroservice
	return app
}

func checkSubscriptions(c *c8y.Client) {
	for {
		as, _, _ := c.Application.GetCurrentApplicationSubscriptions(c.Context.BootstrapUserFromEnvironment())
		slog.Info("Registered Services: ", "users", as.Users)
		time.Sleep(10 * time.Second)
	}
}

// Run starts the microservice
func (a *App) Run() {
	application := a.c8ymicroservice
	application.Scheduler.Start()

	slog.Info("Tenant Info", "tenant", application.Client.TenantName)
	bUrl := application.Client.BaseURL
	serviceBaseUrl := bUrl.Scheme + "://" + bUrl.Hostname() + "/service/" + application.Application.ContextPath
	slog.Info("Service BaseURL", "url", serviceBaseUrl)

	go checkSubscriptions(application.Client)

	// create AWS Client
	awsClient := aws.NewClient(application.WithServiceUser(application.Client.TenantName), application.Client, "repo-integration-fw", "awsConnectionDetails")

	// create firmware tenant controller (for current tenant)
	fc := FirmwareTenantController{
		tenantStore: &FirmwareTenantStore{
			FirmwareByName:         make(map[string]FirmwareStoreFwEntry),
			FirmwareVersionsByName: make(map[string][]FirmwareStoreVersionEntry),
		},
		ctx:            application.WithServiceUser(application.Client.TenantName),
		c8yClient:      application.Client,
		awsClient:      awsClient,
		tenantId:       application.Client.TenantName,
		serviceBaseUrl: serviceBaseUrl,
	}
	// add tenant controller to controllers list
	tenantFwControllers := FirmwareTenantControllers{
		tenantControllers: make(map[string]FirmwareTenantController, 0),
	}
	tenantFwControllers.Register(fc)
	// create external Storage observer (pass tenant controllers + aws Client)
	externalStorageObserver := ExternalStorageObserver{
		awsClient:             awsClient,
		lastKnownHashVersions: "",
		firmwareIndexEntries:  make([]ExtFirmwareVersionEntry, 0),
		tenantControllers:     &tenantFwControllers,
	}
	externalStorageObserver.SyncFirmwareVersionsFile()

	// now start webserver
	if a.echoServer == nil {
		addr := ":" + application.Config.GetString("server.port")
		zap.S().Infof("starting http server on %s", addr)

		a.echoServer = echo.New()
		setDefaultContextHandler(a.echoServer, a.c8ymicroservice)
		provider := c8yauth.NewAuthProvider(application.Client)
		a.echoServer.Use(c8yauth.AuthenticationBasic(provider))
		a.echoServer.Use(c8yauth.AuthenticationBearer(provider))

		a.setRouters(awsClient)

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
func (a *App) setRouters(awsClient *aws.AWSClient) {
	server := a.echoServer
	handlers.RegisterFirmwareHandler(server, awsClient)
	a.c8ymicroservice.AddHealthEndpointHandlers(server)
}

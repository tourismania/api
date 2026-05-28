// Package config Package app is the composition root: it wires every concrete
// dependency into a Container that callers (HTTP server, CLI) consume.
// Keeping it here (rather than inline in main) avoids the duplication
// you'd otherwise get between cmd/server/main.go and cmd/cli/main.go.
package config

import (
	"api/internal/presentation/http/api/v1/user/create"
	getmehttp "api/internal/presentation/http/api/v1/user/get_me"
	"context"
	"fmt"

	createusercmd "api/internal/application/command/create_user"
	syncairportscmd "api/internal/application/command/sync_airports"
	getmeq "api/internal/application/query/get_me"
	searchairports "api/internal/application/query/search_airports"
	"api/internal/domain/factory"
	"api/internal/domain/service"
	"api/internal/infrastructure/auth"
	"api/internal/infrastructure/broker/kafka"
	"api/internal/infrastructure/geo/mwgg"
	"api/internal/infrastructure/geo/static"
	"api/internal/infrastructure/geo/wikidata"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"
	"api/internal/infrastructure/security"
	loginhttp "api/internal/presentation/http/api/login"
	searchairporthttp "api/internal/presentation/http/api/v1/airport/search"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Container holds every wired collaborator the entrypoints need.
type Container struct {
	Cfg *Config

	Pool     *pgxpool.Pool
	Queries  *db.Queries
	Kafka    *kafka.Producer
	JWT      *auth.Service
	Validate *validator.Validate

	// App groups application-layer use-case handlers (write + read sides).
	App struct {
		CreateUser     *createusercmd.Handler
		GetMe          *getmeq.Handler
		SearchAirports *searchairports.Handler
		SyncAirports   *syncairportscmd.Handler
	}

	// Http groups presentation-layer HTTP handlers.
	Http struct {
		Login      *loginhttp.Handler
		CreateUser *createuserhttp.Handler
		GetMe      *getmehttp.Handler
		Airports   *searchairporthttp.Handler
	}
}

// Build constructs the Container.
//
// The order matters: infrastructure adapters first, then domain
// services (which depend on those adapters via interfaces), then
// application handlers, then the buses, then HTTP handlers.
func Build(ctx context.Context, cfg *Config) (*Container, error) {
	pool, err := postgres.NewPool(ctx, cfg.Database.URL())
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	queries := db.New(pool)

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("kafka: %w", err)
	}

	jwtSvc, err := auth.NewService(
		cfg.JWT.PrivateKeyPath, cfg.JWT.PublicKeyPath, cfg.JWT.Passphrase, cfg.JWT.TTL,
	)
	if err != nil {
		_ = producer.Close()
		pool.Close()
		return nil, fmt.Errorf("jwt: %w", err)
	}

	// Domain wiring.
	hasher := security.NewBcryptHasher(bcrypt.DefaultCost)
	userRepo := pgrepo.NewUserRepository(queries)
	userCreator := service.NewUserCreator(userRepo, hasher, producer)
	rightsFactory := factory.NewRightsDescribeFactory()
	rightsDescriber := service.NewRightsDescriber(rightsFactory)

	// Airport domain wiring.
	airportRepo := pgrepo.NewAirportRepository(queries)
	searchAirportsApp := searchairports.NewHandler(airportRepo)

	// Geo sync wiring.
	geoSyncRepo := pgrepo.NewGeoSyncRepository(pool)
	syncAirportsApp := syncairportscmd.NewHandler(
		geoSyncRepo,
		mwgg.New(),
		wikidata.New(),
		static.CountryNames{},
	)

	// Application handlers.
	createUserApp := createusercmd.NewHandler(userCreator)
	getMeApp := getmeq.NewHandler(userRepo, rightsDescriber)

	// Validation.
	validate := validator.New(validator.WithRequiredStructEnabled())

	// HTTP handlers.
	loginH := loginhttp.NewHandler(queries, hasher, jwtSvc, validate)
	createUserH := createuserhttp.NewHandler(createUserApp, validate)
	getMeH := getmehttp.NewHandler(getMeApp, getmehttp.NewResolver())
	airportsH := searchairporthttp.NewHandler(searchAirportsApp, validate)

	c := &Container{
		Cfg:      cfg,
		Pool:     pool,
		Queries:  queries,
		Kafka:    producer,
		JWT:      jwtSvc,
		Validate: validate,
	}

	c.App.CreateUser = createUserApp
	c.App.GetMe = getMeApp
	c.App.SearchAirports = searchAirportsApp
	c.App.SyncAirports = syncAirportsApp

	c.Http.Login = loginH
	c.Http.CreateUser = createUserH
	c.Http.GetMe = getMeH
	c.Http.Airports = airportsH

	return c, nil
}

// Close releases every owned resource. Safe to call on a partially-built
// container (nil fields are skipped).
func (c *Container) Close() {
	if c.Kafka != nil {
		_ = c.Kafka.Close()
	}
	if c.Pool != nil {
		c.Pool.Close()
	}
}

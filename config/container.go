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
	getmeq "api/internal/application/query/get_me"
	"api/internal/domain/factory"
	"api/internal/domain/service"
	"api/internal/infrastructure/auth"
	"api/internal/infrastructure/broker/kafka"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"
	"api/internal/infrastructure/security"
	loginhttp "api/internal/presentation/http/api/login"

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

	CreateUserApp *createusercmd.Handler
	GetMeApp      *getmeq.Handler

	LoginHandler      *loginhttp.Handler
	CreateUserHandler *createuserhttp.Handler
	GetMeHandler      *getmehttp.Handler
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

	// Application handlers.
	createUserApp := createusercmd.NewHandler(userCreator)
	getMeApp := getmeq.NewHandler(userRepo, rightsDescriber)

	// Validation.
	validate := validator.New(validator.WithRequiredStructEnabled())

	// HTTP handlers.
	loginH := loginhttp.NewHandler(queries, hasher, jwtSvc, validate)
	createUserH := createuserhttp.NewHandler(createUserApp, validate)
	getMeH := getmehttp.NewHandler(getMeApp, getmehttp.NewResolver())

	return &Container{
		Cfg:               cfg,
		Pool:              pool,
		Queries:           queries,
		Kafka:             producer,
		JWT:               jwtSvc,
		Validate:          validate,
		CreateUserApp:     createUserApp,
		GetMeApp:          getMeApp,
		LoginHandler:      loginH,
		CreateUserHandler: createUserH,
		GetMeHandler:      getMeH,
	}, nil
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

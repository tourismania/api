// Package config Package app is the composition root: it wires every concrete
// dependency into a Container that callers (HTTP server, CLI) consume.
// Keeping it here (rather than inline in main) avoids the duplication
// you'd otherwise get between cmd/server/main.go and cmd/cli/main.go.
package config

import (
	"log/slog"
	"os"

	"api/internal/domain/agency"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"
	"api/internal/domain/user"
	createuserhttp "api/internal/presentation/http/api/v1/user/create"
	getmehttp "api/internal/presentation/http/api/v1/user/get_me"
	"context"
	"fmt"

	activateagencycmd "api/internal/application/command/activate_agency"
	createagencycmd "api/internal/application/command/create_agency"
	createoffercmd "api/internal/application/command/create_offer"
	createusercmd "api/internal/application/command/create_user"
	deactivateagencycmd "api/internal/application/command/deactivate_agency"
	deleteoffercmd "api/internal/application/command/delete_offer"
	syncairlinescmd "api/internal/application/command/sync_airlines"
	syncairportscmd "api/internal/application/command/sync_airports"
	updateoffercmd "api/internal/application/command/update_offer"
	getairlineq "api/internal/application/query/get_airline"
	getmeq "api/internal/application/query/get_me"
	getofferq "api/internal/application/query/get_offer"
	getoffersq "api/internal/application/query/get_offers"
	getpublishedofferq "api/internal/application/query/get_published_offer"
	listcurrencies "api/internal/application/query/list_currencies"
	searchairlines "api/internal/application/query/search_airlines"
	searchairports "api/internal/application/query/search_airports"
	"api/internal/infrastructure/auth"
	"api/internal/infrastructure/broker/kafka"
	"api/internal/infrastructure/geo/mwgg"
	"api/internal/infrastructure/geo/static"
	"api/internal/infrastructure/geo/wikidata"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"
	pgtxmanager "api/internal/infrastructure/persistence/postgres/txmanager"
	"api/internal/infrastructure/security"
	loginhttp "api/internal/presentation/http/api/login"
	getairlinehttp "api/internal/presentation/http/api/v1/airline/get"
	searchairlinehttp "api/internal/presentation/http/api/v1/airline/search"
	searchairporthttp "api/internal/presentation/http/api/v1/airport/search"
	listcurrencieshttp "api/internal/presentation/http/api/v1/currency/list"
	createofferhttp "api/internal/presentation/http/api/v1/offer/create"
	deleteofferhttp "api/internal/presentation/http/api/v1/offer/delete"
	getofferhttp "api/internal/presentation/http/api/v1/offer/get"
	listoffershttp "api/internal/presentation/http/api/v1/offer/get_list"
	getpublicofferhttp "api/internal/presentation/http/api/v1/offer/get_public"
	updateofferhttp "api/internal/presentation/http/api/v1/offer/update"

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
		CreateUser        *createusercmd.Handler
		CreateAgency      *createagencycmd.Handler
		DeactivateAgency  *deactivateagencycmd.Handler
		ActivateAgency    *activateagencycmd.Handler
		GetMe             *getmeq.Handler
		SearchAirports    *searchairports.Handler
		ListCurrencies    *listcurrencies.Handler
		SyncAirports      *syncairportscmd.Handler
		SearchAirlines    *searchairlines.Handler
		GetAirline        *getairlineq.Handler
		SyncAirlines      *syncairlinescmd.Handler
		CreateOffer       *createoffercmd.Handler
		UpdateOffer       *updateoffercmd.Handler
		DeleteOffer       *deleteoffercmd.Handler
		GetOffer          *getofferq.Handler
		GetOffers         *getoffersq.Handler
		GetPublishedOffer *getpublishedofferq.Handler
	}

	// Http groups presentation-layer HTTP handlers.
	Http struct {
		Login             *loginhttp.Handler
		CreateUser        *createuserhttp.Handler
		GetMe             *getmehttp.Handler
		Airports          *searchairporthttp.Handler
		SearchAirlines    *searchairlinehttp.Handler
		GetAirline        *getairlinehttp.Handler
		Currencies        *listcurrencieshttp.Handler
		CreateOffer       *createofferhttp.Handler
		GetOffer          *getofferhttp.Handler
		GetOffers         *listoffershttp.Handler
		GetPublishedOffer *getpublicofferhttp.Handler
		UpdateOffer       *updateofferhttp.Handler
		DeleteOffer       *deleteofferhttp.Handler
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
	agencyRepo := pgrepo.NewAgencyRepository(queries)
	agencyManager := agency.NewManager(agencyRepo)
	userCreator := user.NewCreator(userRepo, agencyRepo, hasher, producer)
	rightsFactory := user.NewRightsDescribeFactory()
	rightsDescriber := user.NewRightsDescriber(rightsFactory)

	// Offer domain wiring.
	offerRepo := pgrepo.NewOfferRepository(queries)
	offerManager := offer.NewManager(offerRepo, agencyRepo)
	userFinder := user.NewFinder(userRepo)

	// Airport domain wiring.
	airportRepo := pgrepo.NewAirportRepository(queries, pool)
	searchAirportsApp := searchairports.NewHandler(airportRepo)
	listCurrenciesApp := listcurrencies.NewHandler()

	// Airline reference wiring.
	airlineRepo := pgrepo.NewAirlineRepository(queries)
	searchAirlinesApp := searchairlines.NewHandler(airlineRepo)
	getAirlineApp := getairlineq.NewHandler(airlineRepo)

	// Offer flight domain wiring (issue №20). TxManager makes offer +
	// flight writes commit/roll back together across the two repositories.
	offerFlightRepo := pgrepo.NewOfferFlightRepository(queries)
	offerFlightManager := flight.NewManager(offerFlightRepo, airportRepo, airlineRepo)
	txManager := pgtxmanager.New(pool)

	countryRepo := pgrepo.NewCountryRepository(pool)
	cityRepo := pgrepo.NewCityRepository(pool)

	// Geo sync wiring.
	syncAirportsApp := syncairportscmd.NewHandler(
		airportRepo,
		countryRepo,
		cityRepo,
		mwgg.New(),
		wikidata.New(),
		static.CountryNames{},
	)
	// Журнал аномалий синхронизации авиакомпаний — JSON в stderr CLI, отдельно
	// от строк прогресса в stdout (см. docs/cli/sync_airlines.md).
	syncAirlinesApp := syncairlinescmd.NewHandler(
		airlineRepo,
		countryRepo,
		wikidata.New(),
		static.CountryNames{},
		slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	)

	// Application handlers.
	createUserApp := createusercmd.NewHandler(userCreator)
	createAgencyApp := createagencycmd.NewHandler(agencyManager)
	deactivateAgencyApp := deactivateagencycmd.NewHandler(agencyManager)
	activateAgencyApp := activateagencycmd.NewHandler(agencyManager)
	getMeApp := getmeq.NewHandler(userRepo, agencyRepo, rightsDescriber)
	createOfferApp := createoffercmd.NewHandler(offerManager, offerFlightManager, userFinder, txManager)
	updateOfferApp := updateoffercmd.NewHandler(offerManager, offerFlightManager, userFinder, txManager)
	deleteOfferApp := deleteoffercmd.NewHandler(offerManager, userFinder)
	getOfferApp := getofferq.NewHandler(offerManager, offerFlightRepo, airlineRepo, airportRepo, userFinder)
	getOffersApp := getoffersq.NewHandler(offerRepo, userFinder)
	getPublishedOfferApp := getpublishedofferq.NewHandler(offerRepo, offerFlightRepo, airlineRepo, airportRepo)

	// Validation.
	validate := validator.New(validator.WithRequiredStructEnabled())

	// HTTP handlers.
	loginH := loginhttp.NewHandler(queries, hasher, jwtSvc, validate)
	createUserH := createuserhttp.NewHandler(createUserApp, validate)
	getMeH := getmehttp.NewHandler(getMeApp)
	airportsH := searchairporthttp.NewHandler(searchAirportsApp, validate)
	searchAirlineH := searchairlinehttp.NewHandler(searchAirlinesApp, validate)
	getAirlineH := getairlinehttp.NewHandler(getAirlineApp)
	currenciesH := listcurrencieshttp.NewHandler(listCurrenciesApp)
	createOfferH := createofferhttp.NewHandler(createOfferApp, validate)
	getOfferH := getofferhttp.NewHandler(getOfferApp)
	getOffersH := listoffershttp.NewHandler(getOffersApp, validate)
	getPublishedOfferH := getpublicofferhttp.NewHandler(getPublishedOfferApp)
	updateOfferH := updateofferhttp.NewHandler(updateOfferApp, validate)
	deleteOfferH := deleteofferhttp.NewHandler(deleteOfferApp)

	c := &Container{
		Cfg:      cfg,
		Pool:     pool,
		Queries:  queries,
		Kafka:    producer,
		JWT:      jwtSvc,
		Validate: validate,
	}

	c.App.CreateUser = createUserApp
	c.App.CreateAgency = createAgencyApp
	c.App.DeactivateAgency = deactivateAgencyApp
	c.App.ActivateAgency = activateAgencyApp
	c.App.GetMe = getMeApp
	c.App.SearchAirports = searchAirportsApp
	c.App.ListCurrencies = listCurrenciesApp
	c.App.SyncAirports = syncAirportsApp
	c.App.SearchAirlines = searchAirlinesApp
	c.App.GetAirline = getAirlineApp
	c.App.SyncAirlines = syncAirlinesApp
	c.App.CreateOffer = createOfferApp
	c.App.UpdateOffer = updateOfferApp
	c.App.DeleteOffer = deleteOfferApp
	c.App.GetOffer = getOfferApp
	c.App.GetOffers = getOffersApp
	c.App.GetPublishedOffer = getPublishedOfferApp

	c.Http.Login = loginH
	c.Http.CreateUser = createUserH
	c.Http.GetMe = getMeH
	c.Http.Airports = airportsH
	c.Http.SearchAirlines = searchAirlineH
	c.Http.GetAirline = getAirlineH
	c.Http.Currencies = currenciesH
	c.Http.CreateOffer = createOfferH
	c.Http.GetOffer = getOfferH
	c.Http.GetOffers = getOffersH
	c.Http.GetPublishedOffer = getPublishedOfferH
	c.Http.UpdateOffer = updateOfferH
	c.Http.DeleteOffer = deleteOfferH

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

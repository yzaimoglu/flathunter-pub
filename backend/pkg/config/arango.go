package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
	"github.com/gookit/slog"
	"github.com/yzaimoglu/flathunter/pkg/models"
)

const (
	ArangoConnectionString        = "http://%s:%d"
	ArangoConnectionTimeout       = 5
	ArangoUsersCollection         = "users"
	ArangoRolesCollection         = "roles"
	ArangoPlatformsCollection     = "platforms"
	ArangoURLsCollection          = "urls"
	ArangoListingsCollection      = "listings"
	ArangoUserListingsCollection  = "user_listings"
	ArangoUserURLsCollection      = "user_urls"
	ArangoUserSessionsCollection  = "user_sessions"
	ArangoUserNotifiersCollection = "user_notifiers"
)

// ArangoConnection is a struct that holds the connection information for the ArangoDB database.
type ArangoConnection struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

// ArangoClient is a struct that holds the client and context for the ArangoDB database.
type ArangoClient struct {
	Connection ArangoConnection
	Client     driver.Client
	Ctx        context.Context
	Cancel     context.CancelFunc
	Database   driver.Database
}

// NewArangoClient creates a new ArangoDB client.
func NewArangoClient() *ArangoClient {
	arangoConnection := ArangoConnection{
		Host:     GetString("DB_HOST"),
		Port:     GetInteger("DB_PORT"),
		Database: GetString("DB_DATABASE"),
		Username: GetString("DB_USER"),
		Password: GetString("DB_PASSWORD"),
	}

	connectionURI := fmt.Sprintf(ArangoConnectionString, arangoConnection.Host, arangoConnection.Port)

	conn, err := http.NewConnection(http.ConnectionConfig{
		Endpoints: []string{connectionURI},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
		ConnLimit: 100,
	})
	if err != nil {
		slog.Fatalf("Failed to create connection: %v", err)
	}

	client, err := driver.NewClient(driver.ClientConfig{
		Connection:     conn,
		Authentication: driver.BasicAuthentication(arangoConnection.Username, arangoConnection.Password),
	})
	if err != nil {
		slog.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ArangoConnectionTimeout*time.Second)

	arangoClient := &ArangoClient{
		Client:     client,
		Ctx:        ctx,
		Cancel:     cancel,
		Connection: arangoConnection,
	}

	return arangoClient.GetDatabase()
}

// NewArangoClientWithoutDatabase creates a new ArangoDB client without a database.
func NewArangoClientWithoutDatabase() *ArangoClient {
	arangoConnection := ArangoConnection{
		Host:     GetString("DB_HOST"),
		Port:     GetInteger("DB_PORT"),
		Database: GetString("DB_DATABASE"),
		Username: GetString("DB_USER"),
		Password: GetString("DB_PASSWORD"),
	}

	connectionURI := fmt.Sprintf(ArangoConnectionString, arangoConnection.Host, arangoConnection.Port)

	conn, err := http.NewConnection(http.ConnectionConfig{
		Endpoints: []string{connectionURI},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
		ConnLimit: 100,
	})
	if err != nil {
		slog.Fatalf("Failed to create connection: %v", err)
	}

	client, err := driver.NewClient(driver.ClientConfig{
		Connection:     conn,
		Authentication: driver.BasicAuthentication(arangoConnection.Username, arangoConnection.Password),
	})
	if err != nil {
		slog.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ArangoConnectionTimeout*time.Second)

	arangoClient := &ArangoClient{
		Client:     client,
		Ctx:        ctx,
		Cancel:     cancel,
		Connection: arangoConnection,
	}
	return arangoClient
}

// SetupArango creates the database if it does not exist.
func SetupArango() {
	slog.Info("Setting up the ArangoDB database...")

	arangoWithoutDB := NewArangoClientWithoutDatabase()
	if !arangoWithoutDB.CheckDatabase() {
		arangoWithoutDB.CreateDatabase()
	}

	arango := NewArangoClient()
	defer arango.Cancel()

	arango.CheckCollectionsAndCreate()
	arango.CreateStartRoles()
	arango.CreateStartPlatforms()
	slog.Info("Setup of the ArangoDB database complete.")
}

// CreateDatabase creates the flathunter database.
func (arango *ArangoClient) CreateDatabase() (driver.Database, error) {
	db, err := arango.Client.CreateDatabase(arango.Ctx, arango.Connection.Database, &driver.CreateDatabaseOptions{})
	if err != nil {
		slog.Errorf("Failed to create database: %v", err)
		return nil, err
	}

	arango.Database = db
	return db, nil
}

// CheckDatabase checks if the flathunter database exists.
func (arango *ArangoClient) CheckDatabase() bool {
	exists, err := arango.Client.DatabaseExists(arango.Ctx, arango.Connection.Database)
	if err != nil {
		slog.Fatalf("Failed to check if database exists: %v", err)
	}
	return exists
}

// CheckCollectionsAndCreate checks if the collections exist and creates them if they do not.
func (arango *ArangoClient) CheckCollectionsAndCreate() {
	collections := []string{ArangoUsersCollection,
		ArangoPlatformsCollection,
		ArangoRolesCollection,
		ArangoURLsCollection,
		ArangoListingsCollection,
		ArangoUserListingsCollection,
		ArangoUserURLsCollection,
		ArangoUserSessionsCollection,
		ArangoUserNotifiersCollection}
	for _, collection := range collections {
		if !arango.CheckCollection(collection) {
			arango.CreateCollection(collection)
		}
	}
}

// CreateStartRoles creates the start roles if they do not exist.
func (arango *ArangoClient) CreateStartRoles() {
	roles := []string{"admin", "user"}
	for _, role := range roles {
		collection, err := arango.Database.Collection(arango.Ctx, ArangoRolesCollection)
		if err != nil {
			slog.Fatalf("Failed to get collection %s: %v", ArangoRolesCollection, err)
		}
		exists, err := collection.DocumentExists(arango.Ctx, role)
		if err != nil {
			slog.Fatalf("Failed to check if document exists: %v", err)
		}

		if !exists {
			collection, err := arango.Database.Collection(arango.Ctx, ArangoRolesCollection)
			if err != nil {
				slog.Fatalf("Failed to get collection %s: %v", ArangoRolesCollection, err)
			}

			_, err = collection.CreateDocument(arango.Ctx, models.Role{
				ArangoModel: models.ArangoModel{
					Key: role,
				},
				Name:        strings.Title(role),
				Permissions: 999,
			})

			if err != nil {
				slog.Fatalf("Failed to create document: %v", err)
			}
		}
	}
}

// PlatformExists checks if a platform exists.
func (arango *ArangoClient) PlatformExists(name string) bool {
	result, err := arango.Database.Query(arango.Ctx, "FOR platform IN platforms FILTER platform.name == @name RETURN platform", map[string]interface{}{"name": name})
	if err != nil {
		slog.Fatalf("Failed to query database: %v", err)
	}
	defer result.Close()

	var platform models.Platform
	if _, err := result.ReadDocument(arango.Ctx, &platform); err != nil {
		return false
	}
	return true
}

// CreateStartPlatform creates the start platform if it does not exist.
func (arango *ArangoClient) CreateStartPlatform(name string, readable_name string) {
	collection, err := arango.Database.Collection(arango.Ctx, ArangoPlatformsCollection)
	if err != nil {
		slog.Fatalf("Failed to get collection %s: %v", ArangoPlatformsCollection, err)
	}
	if !arango.PlatformExists(name) {
		_, err = collection.CreateDocument(arango.Ctx, models.Platform{
			Name:         name,
			ReadableName: readable_name,
		})

		if err != nil {
			slog.Fatalf("Failed to create document: %v", err)
		}
		slog.Infof("Created platform %s", readable_name)
	}
}

// CreateStartPlatforms creates the start platforms if they do not exist.
func (arango *ArangoClient) CreateStartPlatforms() {
	arango.CreateStartPlatform("ebay_kleinanzeigen", "Ebay Kleinanzeigen")
	arango.CreateStartPlatform("wg_gesucht", "WG-Gesucht")
}

// CheckCollection checks if a collection exists.
func (arango *ArangoClient) CheckCollection(name string) bool {
	collections, err := arango.Database.Collections(arango.Ctx)
	if err != nil {
		slog.Fatalf("Failed to check if collections exist: %v", err)
	}

	for _, collection := range collections {
		if collection.Name() == name {
			return true
		}
	}

	return false
}

// CreateCollection creates a collection.
func (arango *ArangoClient) CreateCollection(name string) error {
	if arango.CheckCollection(name) {
		slog.Infof("Collection %s already exists.", name)
		return nil
	}

	if _, err := arango.Database.CreateCollection(arango.Ctx, name, nil); err != nil {
		slog.Errorf("Failed to create collection: %v", err)
		return err
	}
	return nil
}

// GetDatabase retrieves the flathunter database.
func (arango *ArangoClient) GetDatabase() *ArangoClient {
	db, err := arango.Client.Database(arango.Ctx, arango.Connection.Database)
	if err != nil {
		slog.Fatalf("Failed to retrieve database: %v", err)
	}

	arango.Database = db
	return arango
}

// Close closes the ArangoDB client.
func (arango *ArangoClient) Close() {
	arango.Cancel()
}

package services

import (
	"time"

	"github.com/arangodb/go-driver"
	"github.com/gookit/slog"
	"github.com/yzaimoglu/flathunter/pkg/config"
	"github.com/yzaimoglu/flathunter/pkg/models"
)

// GetURLs retrieves all urls to be crawled from the database.
func GetURLs() ([]models.URL, error) {
	arango := config.NewArangoClient()
	defer arango.Close()

	var urls []models.URL

	result, err := arango.Database.Query(arango.Ctx,
		"FOR url IN urls SORT url.last_crawled ASC LIMIT 50 FILTER url.last_crawled+@crawl_time <= @now FOR platform in platforms FILTER url.platform == platform.name SORT url.last_crawled ASC RETURN merge(url, {platform: platform})",
		map[string]interface{}{"now": time.Now().Unix() + 60*5, "crawl_time": 300})
	if err != nil {
		slog.Error(err)
		return []models.URL{}, config.ErrURLsNotFound
	}
	defer result.Close()

	for {
		var url models.URL
		_, err := result.ReadDocument(arango.Ctx, &url)
		if driver.IsNoMoreDocuments(err) {
			break
		} else if err != nil {
			slog.Errorf("Failed to read document: %v", err)
			return []models.URL{}, config.ErrURLsNotFound
		}
		urls = append(urls, url)
	}

	if len(urls) == 0 {
		return []models.URL{}, config.ErrURLsNotFound
	}

	return urls, nil
}

// InsertURL inserts a new url into the database.
func InsertURL(createURL models.CreateURL) (interface{}, error) {
	arango := config.NewArangoClient()
	defer arango.Close()

	collection, err := arango.Database.Collection(arango.Ctx, "urls")
	if err != nil {
		slog.Errorf("Failed to retrieve collection: %v", err)
		return nil, err
	}

	meta, err := collection.CreateDocument(arango.Ctx, createURL)
	if err != nil {
		slog.Errorf("Failed to create document: %v", err)
		return nil, err
	}

	return meta.Key, nil
}

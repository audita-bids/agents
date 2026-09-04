package store

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func index(keys bson.D) mongo.IndexModel {
	return mongo.IndexModel{Keys: keys}
}

func uniqueIndex(keys bson.D) mongo.IndexModel {
	return mongo.IndexModel{Keys: keys, Options: options.Index().SetUnique(true)}
}

// persist one index at a time, so a single conflict does not drop the whole set.
func persist(ctx context.Context, c *mongo.Collection, indexes []mongo.IndexModel) error {
	var errs error

	for _, i := range indexes {
		if _, err := c.Indexes().CreateOne(ctx, i); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

// PersistIndexes an analysis is looked up by the pair that produced it, and never any other way.
func PersistIndexes(ctx context.Context, db *mongo.Database) error {
	return persist(ctx, db.Collection("analysis"), []mongo.IndexModel{
		index(bson.D{{Key: "user_id", Value: 1}, {Key: "process_id", Value: 1}}),
	})
}

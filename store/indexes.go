package store

import (
	"context"

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

func persist(ctx context.Context, c *mongo.Collection, indexes []mongo.IndexModel) error {
	_, err := c.Indexes().CreateMany(ctx, indexes)
	return err
}

// PersistIndexes an analysis is looked up by the pair that produced it, and never any other way.
func PersistIndexes(ctx context.Context, db *mongo.Database) error {
	return persist(ctx, db.Collection("analysis"), []mongo.IndexModel{
		uniqueIndex(bson.D{{Key: "userid", Value: 1}, {Key: "bidid", Value: 1}}),
	})
}

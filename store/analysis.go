package store

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Analysis struct {
	ID        *bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Finished  bool           `json:"finished" bson:"finished"`
	ProcessID string         `json:"process_id" bson:"process_id,omitempty"`
	UserID    string         `json:"user_id" bson:"user_id,omitempty"`
	CreatedAt *time.Time     `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt *time.Time     `json:"updated_at" bson:"updated_at,omitempty"`

	Base64           string `json:"base64,omitempty" bson:"-"`
	Content          string `json:"content,omitempty" bson:"content,omitempty"`
	Object           string `json:"object,omitempty" bson:"object,omitempty"`
	Modality         string `json:"modality,omitempty" bson:"modality,omitempty"`
	ProcessNumber    string `json:"process_number,omitempty" bson:"process_number,omitempty"`
	EstimatedValue   string `json:"estimated_value,omitempty" bson:"-"`
	OpeningDate      string `json:"opening_date,omitempty" bson:"opening_date,omitempty"`
	JudgmentCriteria string `json:"judgment_criteria,omitempty" bson:"judgment_criteria,omitempty"`
}

func (a *Analysis) Unmarshal(v interface{}) error {
	b, _ := json.Marshal(a)
	return json.Unmarshal(b, &v)
}

func (a *Analysis) Decode(r *http.Request) {
	json.NewDecoder(r.Body).Decode(&a)
	vars := mux.Vars(r)
	id, err := bson.ObjectIDFromHex(vars["id"])
	if err != nil {
		id = bson.NewObjectID()
	}

	a.ID = &id
}

/* Stores */
type AnalysisStore struct {
	C *mongo.Collection
}

func (store *AnalysisStore) CreateAnalysis(ctx context.Context, analysis *Analysis) error {
	_, err := store.C.InsertOne(ctx, analysis)

	if err != nil {
		return err
	}

	return nil
}

/*
func (store *FavoriteBidStore) CreateFavorite(ctx context.Context, favorite *FavoriteBid) error {
	_, err := store.C.InsertOne(ctx, favorite)

	if err != nil {
		return err
	}

	return nil
}

func (store *FavoriteBidStore) ListFavorite(ctx context.Context, favorite *FavoriteBid) ([]*FavoriteBid, int64, error) {
	f, _ := decode.GetFromContext[query.Filter](ctx, "filter")

	if favorite.UserID != "" {
		f.Matches = append(f.Matches, query.Match{
			Key:   "user_id",
			Op:    "eq",
			Value: favorite.UserID,
		})
	}

	if favorite.ProcessID != "" {
		f.Matches = append(f.Matches, query.Match{
			Key:   "process_id",
			Op:    "eq",
			Value: favorite.ProcessID,
		})
	}

	bsonFilter := f.AdaptBsonFilter(bson.M{}, &f)

	limit := f.Rows
	skip := (f.Page - 1) * limit
	opt := options.Find().
		SetLimit(limit).
		SetSkip(skip)

	var bids []*FavoriteBid

	cursor, err := store.C.Find(ctx, bsonFilter, opt)
	if err != nil {
		return nil, 0, err
	}

	if err := cursor.All(ctx, &bids); err != nil {
		return nil, 0, err
	}

	var total int64
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		total, err = store.C.CountDocuments(ctx, bsonFilter)

		if err != nil {
			total = 0
		}
	}()

	wg.Wait()

	return bids, total, err
}*/
